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

package pdfenc

import (
	"testing"
	"unicode"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/postscript/type1/names"
)

func TestMacRoman(t *testing.T) {
	enc := font.MacRomanEncoding
	for c := 0; c < 256; c++ {
		r1 := enc.Decode(byte(c))

		name := MacRomanEncoding[c]
		if name == ".notdef" && r1 == unicode.ReplacementChar {
			continue
		}
		rr := names.ToUnicode(MacRomanEncoding[c], false)
		if len(rr) != 1 {
			t.Errorf("len(rr) != 1 for %d", c)
			continue
		}
		r2 := rr[0]

		if r1 != r2 {
			t.Errorf("wrong result: %d: %d vs %d", c, r1, r2)
		}
	}
}
