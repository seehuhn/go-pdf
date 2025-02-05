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

package encoding

import (
	"testing"

	"seehuhn.de/go/sfnt/glyph"
)

// TestSimpleEncoder tests whether the SimpleEncoder can allocate 256 codes,
// and whether it reuses codes when possible.
func TestSimpleEncoder(t *testing.T) {
	e := NewSimpleEncoder()

	codes := make(map[byte]int)
	for i := 0; i < 256; i++ {
		gid := max(glyph.ID(i), 10)
		c := e.GIDToCode(gid, string(rune(i+32)))
		if _, seen := codes[c]; seen {
			t.Errorf("%d: code %d used twice", i, c)
		}
		codes[c] = i

		c2 := e.GIDToCode(gid, string(rune(i+32)))
		if c != c2 {
			t.Errorf("%d: code %d != %d", i, c, c2)
		}
	}

	if e.Overflow() {
		t.Errorf("unexpected overflow")
	}
	if len(e.code) != 256 {
		t.Errorf("unexpected cache length %d", len(e.code))
	}
}
