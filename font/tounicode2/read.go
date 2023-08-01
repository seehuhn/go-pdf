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
	"fmt"
	"io"
	"unicode/utf16"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/postscript"
	"seehuhn.de/go/postscript/type1"
)

func Read(r io.Reader) (*Info, error) {
	cmap, err := cmap.ReadRaw(r)
	if err != nil {
		return nil, err
	}

	tp := cmap["CMapType"]
	if tp != postscript.Integer(2) {
		return nil, fmt.Errorf("tounicode: invalid CMap type %v", tp)
	}
	codeMap := cmap["CodeMap"].(*postscript.CmapInfo)

	cmapName, _ := cmap["CMapName"].(postscript.Name)
	var ROS *type1.CIDSystemInfo
	if cidSystemInfo, _ := cmap["CIDSystemInfo"].(postscript.Dict); cidSystemInfo != nil {
		Registry, ok1 := cidSystemInfo["Registry"].(postscript.String)
		Ordering, ok2 := cidSystemInfo["Ordering"].(postscript.String)
		Supplement, ok3 := cidSystemInfo["Supplement"].(postscript.Integer)
		if ok1 && ok2 && ok3 {
			ROS = &type1.CIDSystemInfo{
				Registry:   string(Registry),
				Ordering:   string(Ordering),
				Supplement: int32(Supplement),
			}
		}
	}

	csRanges := make([]charcode.Range, len(codeMap.CodeSpaceRanges))
	for i, r := range codeMap.CodeSpaceRanges {
		csRanges[i] = charcode.Range(r)
	}
	cs := charcode.NewCodeSpace(csRanges)

	res := &Info{
		Name:      pdf.Name(cmapName),
		ROS:       ROS,
		CodeSpace: cs,
		Singles:   []Single{},
		Ranges:    []Range{},
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
		res.Singles = append(res.Singles, Single{
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
			res.Ranges = append(res.Ranges, Range{
				First:  low,
				Last:   high,
				Values: [][]rune{rr},
			})
		case postscript.Array:
			if len(r) != int(high)-int(low)+1 {
				return nil, fmt.Errorf("invalid CMap")
			}
			var values [][]rune
			for code := low; code <= high; code++ {
				rr, err := toRunes(r[code-low])
				if err != nil {
					return nil, err
				}
				values = append(values, rr)
			}
			res.Ranges = append(res.Ranges, Range{
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
