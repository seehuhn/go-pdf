// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package cmap

import (
	"fmt"
	"io"
	"unicode/utf16"

	"seehuhn.de/go/postscript"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
)

type ToUnicodeInfo struct {
	charcode.CodeSpaceRange

	Singles []ToUnicodeSingle
	Ranges  []ToUnicodeRange

	Parent *ToUnicodeInfo // This corresponds to the UseCMap entry in the PDF spec.
}

// ToUnicodeSingle specifies that character code Code represents the given unicode string.
type ToUnicodeSingle struct {
	Code  []byte
	Value []rune
}

func (s ToUnicodeSingle) String() string {
	return fmt.Sprintf("% 02x: %q", s.Code, s.Value)
}

// ToUnicodeRange describes a range of character codes.
type ToUnicodeRange struct {
	First  []byte
	Last   []byte
	Values [][]rune
}

func (r ToUnicodeRange) String() string {
	ss := make([]string, len(r.Values))
	for i, v := range r.Values {
		ss[i] = string(v)
	}
	return fmt.Sprintf("% 02x-% 02x: %q", r.First, r.Last, ss)
}

func ExtractToUnicodeNew(r pdf.Getter, obj pdf.Object) (*ToUnicodeInfo, error) {
	cycle := pdf.NewCycleChecker()
	return safeExtractToUnicode(r, cycle, obj)
}

func safeExtractToUnicode(r pdf.Getter, cycle *pdf.CycleChecker, obj pdf.Object) (*ToUnicodeInfo, error) {
	if err := cycle.Check(obj); err != nil {
		return nil, err
	}

	stmObj, err := pdf.GetStream(r, obj)
	if err != nil {
		return nil, err
	}

	err = pdf.CheckDictType(r, stmObj.Dict, "CMap")
	if err != nil {
		return nil, err
	}

	body, err := pdf.DecodeStream(r, stmObj, 0)
	if err != nil {
		return nil, err
	}

	res, err := readToUnicode(body)
	if err != nil {
		return nil, err
	}

	parent := stmObj.Dict["UseCMap"]
	if parent != nil {
		parentInfo, err := safeExtractToUnicode(r, cycle, parent)
		if err != nil {
			return nil, err
		}
		res.Parent = parentInfo
	}

	return res, nil
}

func readToUnicode(r io.Reader) (*ToUnicodeInfo, error) {
	raw, err := postscript.ReadCMap(r)
	if err != nil {
		return nil, err
	}

	if tp, _ := raw["CMapType"].(postscript.Integer); !(tp == 0 || tp == 2) {
		return nil, pdf.Errorf("invalid CMapType: %d", tp)
	}

	res := &ToUnicodeInfo{}

	codeMap, ok := raw["CodeMap"].(*postscript.CMapInfo)
	if !ok {
		return nil, pdf.Error("unsupported CMap format")
	}

	for _, entry := range codeMap.CodeSpaceRanges {
		if len(entry.Low) != len(entry.High) || len(entry.Low) == 0 {
			continue
		}
		res.CodeSpaceRange = append(res.CodeSpaceRange,
			charcode.Range{Low: entry.Low, High: entry.High})
	}

	for _, entry := range codeMap.BfChars {
		if len(entry.Src) == 0 {
			continue
		}
		rr, _ := toRunes(entry.Dst)
		if rr == nil {
			continue
		}
		res.Singles = append(res.Singles, ToUnicodeSingle{
			Code:  entry.Src,
			Value: rr,
		})
	}
	for _, entry := range codeMap.BfRanges {
		if len(entry.Low) != len(entry.High) || len(entry.Low) == 0 {
			continue
		}

		switch r := entry.Dst.(type) {
		case postscript.String:
			rr, _ := toRunes(r)
			if rr == nil {
				continue
			}
			res.Ranges = append(res.Ranges, ToUnicodeRange{
				First:  entry.Low,
				Last:   entry.High,
				Values: [][]rune{rr},
			})
		case postscript.Array:
			values := make([][]rune, 0, len(r))
			for _, v := range r {
				rr, _ := toRunes(v)
				if rr != nil {
					values = append(values, rr)
				} else {
					values = append(values, brokenReplacement)
				}
			}
			res.Ranges = append(res.Ranges, ToUnicodeRange{
				First:  entry.Low,
				Last:   entry.High,
				Values: values,
			})
		}
	}

	return res, nil
}

func toRunes(obj postscript.Object) ([]rune, error) {
	dst, ok := obj.(postscript.String)
	if !ok || len(dst)%2 != 0 {
		return nil, fmt.Errorf("invalid ToUnicode CMap")
	}
	buf := make([]uint16, 0, len(dst)/2)
	for i := 0; i < len(dst); i += 2 {
		buf = append(buf, uint16(dst[i])<<8|uint16(dst[i+1]))
	}
	return utf16.Decode(buf), nil
}

var brokenReplacement = []rune{0xFFFD}
