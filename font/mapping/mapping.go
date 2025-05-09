// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package mapping

import (
	"compress/gzip"
	"embed"
	"errors"
	"fmt"
	_ "io/fs" // for use in the GetCIDTextMapping doc string
	"sync"
	"unicode/utf16"

	"seehuhn.de/go/postscript"
	"seehuhn.de/go/postscript/cid"
)

//go:embed resources
var resources embed.FS

var (
	resourceMutex sync.Mutex
	cache         = make(map[string]map[cid.CID]string)
)

// GetCIDTextMapping returns a mapping from CID to text for the given registry
// and ordering. If no mapping is known, an error is returned.  The returned
// error in this case wraps [fs.ErrNotExist].
//
// The returned mapping is read-only and must not be modified by the caller.
func GetCIDTextMapping(registry, ordering string) (map[cid.CID]string, error) {
	resourceMutex.Lock()
	defer resourceMutex.Unlock()

	fileName := registry + "-" + ordering + "-UCS2.gz"
	if mapping, ok := cache[fileName]; ok {
		return mapping, nil
	}

	cmapCompressed, err := resources.Open("resources/" + fileName)
	if err != nil {
		return nil, err
	}
	defer cmapCompressed.Close()

	cmapFile, err := gzip.NewReader(cmapCompressed)
	if err != nil {
		return nil, err
	}
	defer cmapFile.Close()

	cmapData, err := postscript.ReadCMap(cmapFile)
	if err != nil {
		return nil, err
	}

	codeMap := cmapData["CodeMap"].(*postscript.CMapInfo)
	if codeMap.UseCMap != "" {
		return nil, errors.New("not implemented: chained PDF mapping resources")
	}

	// after this point, ignore errors to allow best-effort decoding of
	// broken mappings.

	mapping := make(map[cid.CID]string)
rangeLoop:
	for _, entry := range codeMap.BfRanges {
		low, err := getCID(entry.Low)
		if err != nil {
			continue
		}
		high, err := getCID(entry.High)
		if err != nil {
			continue
		}

		var base string
		var values []string
		switch r := entry.Dst.(type) {
		case postscript.String:
			base, _ = toString(r)
		case postscript.Array:
			values = make([]string, 0, len(r))
			for _, v := range r {
				s, _ := toString(v)
				if len(s) > 0 {
					values = append(values, s)
				} else {
					values = append(values, brokenReplacement)
				}
			}
		default:
			continue rangeLoop
		}

		// We have high<=0xFFFF and cid.CID is uint32, so this loop will
		// always terminate.
		for cid := low; cid <= high; cid++ {
			if idx := cid - low; int(idx) < len(values) {
				if values[idx] != brokenReplacement {
					mapping[cid] = values[idx]
				}
			} else if len(base) > 0 {
				rr := []rune(base)
				rr[len(rr)-1] += rune(idx)
				mapping[cid] = string(rr)
			}
		}
	}

	for _, entry := range codeMap.BfChars {
		cid, err := getCID(entry.Src)
		if err != nil {
			continue
		}
		s, err := toString(entry.Dst)
		if err == nil {
			mapping[cid] = s
		}
	}

	cache[fileName] = mapping
	return mapping, nil
}

func getCID(s []byte) (cid.CID, error) {
	if len(s) != 2 {
		return 0, errors.New("malformed PDF mapping resource")
	}
	return cid.CID(uint16(s[0])<<8 | uint16(s[1])), nil
}

func toString(obj postscript.Object) (string, error) {
	dst, ok := obj.(postscript.String)
	if !ok || len(dst)%2 != 0 {
		return "", fmt.Errorf("malformed PDF mapping resource")
	}
	buf := make([]uint16, 0, len(dst)/2)
	for i := 0; i < len(dst); i += 2 {
		buf = append(buf, uint16(dst[i])<<8|uint16(dst[i+1]))
	}
	return string(utf16.Decode(buf)), nil
}

const brokenReplacement = "\uFFFD"
