// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package tounicode

import (
	"errors"
	"fmt"
	"io"
	"unicode/utf16"

	"seehuhn.de/go/postscript"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
)

// Extract extracts a ToUnicode CMap from a PDF file.
// If cs is not nil, it overrides the code space range given inside the CMap.
func Extract(r pdf.Getter, obj pdf.Object, cs charcode.CodeSpaceRange) (*Info, error) {
	stm, err := pdf.GetStream(r, obj)
	if err != nil {
		return nil, err
	} else if stm == nil {
		return nil, nil
	}
	data, err := pdf.DecodeStream(r, stm, 0)
	if err != nil {
		return nil, err
	}
	return Read(data, cs)
}

// Read reads a ToUnicode CMap.
// If cs is not nil, it overrides the code space range given inside the CMap.
func Read(r io.Reader, cs charcode.CodeSpaceRange) (*Info, error) {
	cmap, err := cmap.ReadRaw(r)
	if err != nil {
		return nil, err
	}

	tp := cmap["CMapType"]
	if tp != postscript.Integer(2) {
		return nil, fmt.Errorf("tounicode: invalid CMap type %v", tp)
	}
	codeMap := cmap["CodeMap"].(*postscript.CMapInfo)

	cmapName, _ := cmap["CMapName"].(postscript.Name)

	csRanges := make(charcode.CodeSpaceRange, len(codeMap.CodeSpaceRanges))
	for i, r := range codeMap.CodeSpaceRanges {
		csRanges[i] = charcode.Range(r)
	}
	if cs == nil { // TODO(voss): check this
		cs = csRanges
	}

	res := &Info{
		Name: string(cmapName),
		CS:   cs,
	}

	for _, c := range codeMap.Chars {
		code, k := cs.Decode(c.Src)
		if code < 0 || len(c.Src) != k {
			return nil, fmt.Errorf("tounicode: invalid code <%02x>", c.Src)
		}
		rr, err := toRunes(c.Dst)
		if err != nil {
			return nil, err
		}
		res.Singles = append(res.Singles, SingleEntry{
			Code:  code,
			Value: rr,
		})
	}
	for _, r := range codeMap.Ranges {
		low, k := cs.Decode(r.Low)
		if low < 0 || len(r.Low) != k {
			return nil, fmt.Errorf("tounicode: invalid first code <%02x>", r.Low)
		}
		high, k := cs.Decode(r.High)
		if high < 0 || len(r.High) != k {
			return nil, fmt.Errorf("tounicode: invalid last code <%02x>", r.High)
		}

		switch r := r.Dst.(type) {
		case postscript.String:
			rr, err := toRunes(r)
			if err != nil {
				return nil, err
			}
			res.Ranges = append(res.Ranges, RangeEntry{
				First:  low,
				Last:   high,
				Values: [][]rune{rr},
			})
		case postscript.Array:
			if len(r) != int(high)-int(low)+1 {
				return nil, errors.New("invalid CMap")
			}
			var values [][]rune
			for code := low; code <= high; code++ {
				rr, err := toRunes(r[code-low])
				if err != nil {
					return nil, err
				}
				values = append(values, rr)
			}
			res.Ranges = append(res.Ranges, RangeEntry{
				First:  low,
				Last:   high,
				Values: values,
			})
			// TODO(voss): do we need to check for other types?
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
