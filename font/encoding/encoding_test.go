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
		s := e.AppendEncoded(nil, glyph.ID(i+1), []rune{rune(i + 32)})
		if len(s) != 1 {
			t.Fatalf("unexpected length %d", len(s))
		}

		c := s[0]
		if _, seen := codes[c]; seen {
			t.Errorf("%d: code %d used twice", i, c)
		}
		codes[c] = i
	}

	if e.Overflow() {
		t.Errorf("unexpected overflow")
	}
	if len(e.code) != 256 {
		t.Errorf("unexpected cache length %d", len(e.code))
	}

	for i := 0; i < 256; i++ {
		s := e.AppendEncoded(nil, glyph.ID(i+1), []rune{rune(i + 32)})
		if len(s) != 1 {
			t.Errorf("%d: unexpected length %d", i, len(s))
			continue
		}

		c := s[0]
		prevI, seen := codes[c]
		if !seen {
			t.Errorf("code %d not seen before", c)
		} else if prevI != i {
			t.Errorf("previous code %d != %d", prevI, i)
		}
	}
}
