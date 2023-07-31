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
	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1/names"
)

// TestStandardEncoding verifies that the standard encoding here and
// in seehuh.de/pdf/font are consistent.
func TestStandardEncoding(t *testing.T) {
	for code, name := range StandardEncoding {
		r1 := font.StandardEncoding.Decode(byte(code))
		var r2 rune
		if name == ".notdef" {
			r2 = unicode.ReplacementChar
		} else {
			rr2 := names.ToUnicode(string(name), false)
			if len(rr2) != 1 {
				t.Errorf("bad name: %s", name)
				continue
			}
			r2 = rr2[0]
		}
		if r1 != r2 {
			t.Errorf("StandardEncoding[%d] = %q != %q", code, r1, r2)
		}
	}
}

// TestStandardEncoding2 tests that the PDF standard encoding is
// the same as the postscript standard encoding.
func TestStandardEncoding2(t *testing.T) {
	for code, name := range StandardEncoding {
		if string(name) != psenc.StandardEncoding[code] {
			t.Errorf("StandardEncoding[%d] = %q != %q", code, name, psenc.StandardEncoding[code])
		}
	}
}
