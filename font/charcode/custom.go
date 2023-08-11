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

package charcode

import (
	"bytes"

	"seehuhn.de/go/pdf"
)

func NewCodeSpace(ranges []Range) CodeSpaceRange {
	if len(ranges) == 1 {
		r := ranges[0]
		if bytes.Equal(r.Low, []byte{0x00}) && bytes.Equal(r.High, []byte{0xFF}) {
			return Simple
		}
		if bytes.Equal(r.Low, []byte{0x00, 0x00}) && bytes.Equal(r.High, []byte{0xFF, 0xFF}) {
			return UCS2
		}
	}

	res := make(customCS, len(ranges))
	for i, r := range ranges {
		var numCodes CharCode = 1
		for i := range r.Low {
			numCodes *= CharCode(r.High[i]-r.Low[i]) + 1
		}
		res[i].Range = r
		res[i].NumCodes = numCodes
	}
	return res
}

type customCS []customRange

func (c customCS) Append(s pdf.String, code CharCode) pdf.String {
	for _, r := range c {
		if code >= r.NumCodes {
			code -= r.NumCodes
			continue
		}

		n := len(s)
		for range r.Low {
			s = append(s, 0)
		}
		for i := len(r.Low) - 1; i >= 0; i-- {
			k := CharCode(r.High[i]) - CharCode(r.Low[i]) + 1
			s[n+i] = r.Low[i] + byte(code%k)
			code /= k
		}
		break
	}
	return s
}

func (c customCS) Decode(s pdf.String) (CharCode, int) {
	var base CharCode
tryNextRange:
	for _, r := range c {
		if len(s) < len(r.Low) {
			base += r.NumCodes
			continue tryNextRange
		}

		var code CharCode
		for i := 0; i < len(r.Low); i++ {
			b := s[i]
			if b < r.Low[i] || b > r.High[i] {
				base += r.NumCodes
				continue tryNextRange
			}

			k := CharCode(r.High[i]) - CharCode(r.Low[i]) + 1
			code = code*k + CharCode(b-r.Low[i])
		}
		return code + base, len(r.Low)
	}

	if len(s) == 0 {
		return -1, 0
	}
	return -1, 1
}

func (c customCS) Ranges() []Range {
	res := make([]Range, len(c))
	for i, r := range c {
		res[i] = r.Range
	}
	return res
}

type customRange struct {
	Range
	NumCodes CharCode
}
