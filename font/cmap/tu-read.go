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

package cmap

import (
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/postscript"

	"seehuhn.de/go/pdf/font/charcode"
)

// ReadToUnicode reads a ToUnicode CMap.
// If cs is not nil, it overrides the code space range given inside the CMap.
func ReadToUnicode(r io.Reader, cs charcode.CodeSpaceRange) (*ToUnicodeOld, error) {
	raw, err := postscript.ReadCMap(r)
	if err != nil {
		return nil, err
	}

	if tp, ok := raw["CMapType"].(postscript.Integer); ok && tp != 2 {
		return nil, fmt.Errorf("invalid CMapType: %v", tp)
	}
	codeMap, ok := raw["CodeMap"].(*postscript.CMapInfo)
	if !ok {
		return nil, fmt.Errorf("unsupported CMap format")
	}

	csRanges := make(charcode.CodeSpaceRange, len(codeMap.CodeSpaceRanges))
	for i, r := range codeMap.CodeSpaceRanges {
		csRanges[i] = charcode.Range(r)
	}
	if cs == nil { // TODO(voss): check this
		cs = csRanges
	}

	res := &ToUnicodeOld{
		CS: cs,
	}

	for _, c := range codeMap.BfChars {
		code, k := cs.Decode(c.Src)
		if code < 0 || len(c.Src) != k {
			return nil, fmt.Errorf("tounicode: invalid code <%02x>", c.Src)
		}
		s, err := toString(c.Dst)
		if err != nil {
			return nil, err
		}
		res.Singles = append(res.Singles, ToUnicodeSingleOld{
			Code:  code,
			Value: []rune(s),
		})
	}
	for _, r := range codeMap.BfRanges {
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
			s, err := toString(r)
			if err != nil {
				return nil, err
			}
			res.Ranges = append(res.Ranges, ToUnicodeRangeOld{
				First:  low,
				Last:   high,
				Values: [][]rune{[]rune(s)},
			})
		case postscript.Array:
			if len(r) != int(high)-int(low)+1 {
				return nil, errors.New("invalid CMap")
			}
			var values [][]rune
			for code := low; code <= high; code++ {
				s, err := toString(r[code-low])
				if err != nil {
					return nil, err
				}
				values = append(values, []rune(s))
			}
			res.Ranges = append(res.Ranges, ToUnicodeRangeOld{
				First:  low,
				Last:   high,
				Values: values,
			})
			// TODO(voss): do we need to check for other types?
		}
	}

	return res, nil
}
