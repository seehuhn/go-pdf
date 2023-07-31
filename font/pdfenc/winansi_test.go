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

// TestWinAnsiEncoding verifies that the standard encoding here and
// in seehuh.de/pdf/font are consistent.
func TestWinAnsiEncoding(t *testing.T) {
	for code, name := range WinAnsiEncoding {
		r1 := font.WinAnsiEncoding.Decode(byte(code))
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
			t.Errorf("WinAnsiEncoding[0o%03o] = %q != %q", code, r1, r2)
		}
	}
}
