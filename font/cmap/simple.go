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

import "seehuhn.de/go/pdf"

func (info *ToUnicode) GetSimpleMapping() [][]rune {
	cs := info.CS
	res := make([][]rune, 256)
	var s pdf.String
	for _, entry := range info.Singles {
		s = cs.Append(s[:0], entry.Code)
		if len(s) == 1 {
			res[s[0]] = entry.Value
		}
	}
	for _, entry := range info.Ranges {
		s = cs.Append(s[:0], entry.First)
		if len(s) != 1 {
			continue
		}
		first := int(s[0])
		s = cs.Append(s[:0], entry.Last)
		if len(s) != 1 || int(s[0]) < first {
			continue
		}
		last := int(s[0])
		if len(entry.Values) == 1 {
			base := entry.Values[0]
			if len(base) > 0 {
				for i := 0; i <= last-first; i++ {
					res[first+i] = base
					base = c(base)
					base[len(base)-1]++
				}
			}
		} else {
			for i := 0; i <= last-first && i < len(entry.Values); i++ {
				res[first+i] = entry.Values[i]
			}
		}
	}
	return res
}
