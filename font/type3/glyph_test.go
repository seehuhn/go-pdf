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

package type3

import (
	"testing"

	"seehuhn.de/go/postscript/funit"
)

// TestGlyphError makes sure that errors during glyph construction are reported.
func TestGlyphError(t *testing.T) {
	F := New(1000)
	g, err := F.AddGlyph("test", 1000, funit.Rect16{LLx: 0, LLy: 0, URx: 1000, URy: 1000}, false)
	if err != nil {
		t.Fatalf("AddGlyph failed: %v", err)
	}

	g.TextEnd() // should cause an error

	err = g.Close()
	if err == nil {
		t.Fatalf("Close failed to report the error")
	}
}
