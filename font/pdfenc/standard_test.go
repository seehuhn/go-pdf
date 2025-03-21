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

	"seehuhn.de/go/postscript/psenc"
	"seehuhn.de/go/postscript/type1/names"
)

// TestStandardEncoding tests that the PDF standard encoding is
// the same as the postscript standard encoding.
func TestStandardEncoding(t *testing.T) {
	for code, name := range Standard.Encoding {
		if name != psenc.StandardEncoding[code] {
			t.Errorf("StandardEncoding[%d] = %q != %q", code, name, psenc.StandardEncoding[code])
		}
	}
}

// TestStandardEncoding2 tests that all encoded entries in StandardEncoding
// correspond to a single unicode rune.
func TestStandardEncoding2(t *testing.T) {
	for _, name := range Standard.Encoding {
		if name == ".notdef" {
			continue
		}
		rr := names.ToUnicode(name, "")
		if len([]rune(rr)) != 1 {
			t.Errorf("StandardEncoding[%q] = %q, which is not a single unicode rune", name, rr)
		}
	}
}
