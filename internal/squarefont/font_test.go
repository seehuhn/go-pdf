// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package squarefont

import (
	"testing"
)

func TestSampleFonts(t *testing.T) {
	// Test that all fonts can be created without panic
	for _, sample := range All {
		t.Run(sample.Label, func(t *testing.T) {
			font := sample.MakeFont()
			if font == nil {
				t.Fatal("MakeFont returned nil")
			}

			geom := font.GetGeometry()
			if geom == nil {
				t.Fatal("GetGeometry returned nil")
			}

			if len(geom.Widths) != 3 {
				t.Errorf("expected 3 widths, got %d", len(geom.Widths))
			}

			if len(geom.GlyphExtents) != 3 {
				t.Errorf("expected 3 glyph extents, got %d", len(geom.GlyphExtents))
			}

			t.Logf("Font %s: Ascent=%.1f, Descent=%.1f, Leading=%.1f",
				sample.Label, geom.Ascent, geom.Descent, geom.Leading)

			seq := font.Layout(nil, 1, "A")
			gid := seq.Seq[0].GID
			t.Logf("  Square width: %.1f", geom.Widths[gid])
			t.Logf("  Square extent: [%.1f,%.1f]x[%.1f,%.1f]",
				geom.GlyphExtents[gid].LLx, geom.GlyphExtents[gid].URx,
				geom.GlyphExtents[gid].LLy, geom.GlyphExtents[gid].URy)
		})
	}
}
