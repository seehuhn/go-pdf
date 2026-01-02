// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package font_test

import (
	"testing"

	"seehuhn.de/go/pdf/internal/fonttypes"
)

// TestGlyphExtentsPlausibleRange verifies that GlyphExtents for "A" are in
// a plausible range (text space units) for all font types.
func TestGlyphExtentsPlausibleRange(t *testing.T) {
	// Conservative bounds for text space units.
	// Typical values: LLx ~0.0, LLy ~0.0, URx ~0.6, URy ~0.7
	const minVal, maxVal = -0.5, 1.5

	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			font := sample.MakeFont()
			if font == nil {
				t.Fatal("MakeFont returned nil")
			}

			geom := font.GetGeometry()
			if geom == nil {
				t.Fatal("GetGeometry returned nil")
			}

			// layout "A" to get the glyph ID
			seq := font.Layout(nil, 12, "A")
			if len(seq.Seq) == 0 {
				t.Skip("font cannot layout 'A'")
			}
			gid := seq.Seq[0].GID

			if int(gid) >= len(geom.GlyphExtents) {
				t.Fatalf("glyph ID %d out of range (len=%d)", gid, len(geom.GlyphExtents))
			}

			ext := geom.GlyphExtents[gid]
			if ext.IsZero() {
				t.Skip("glyph 'A' has zero extent")
			}

			// check each coordinate is in plausible range
			if ext.LLx < minVal || ext.LLx > maxVal {
				t.Errorf("LLx=%v outside plausible range [%v, %v]", ext.LLx, minVal, maxVal)
			}
			if ext.LLy < minVal || ext.LLy > maxVal {
				t.Errorf("LLy=%v outside plausible range [%v, %v]", ext.LLy, minVal, maxVal)
			}
			if ext.URx < minVal || ext.URx > maxVal {
				t.Errorf("URx=%v outside plausible range [%v, %v]", ext.URx, minVal, maxVal)
			}
			if ext.URy < minVal || ext.URy > maxVal {
				t.Errorf("URy=%v outside plausible range [%v, %v]", ext.URy, minVal, maxVal)
			}

			// sanity check: URx > LLx and URy > LLy
			if ext.URx <= ext.LLx {
				t.Errorf("URx=%v should be greater than LLx=%v", ext.URx, ext.LLx)
			}
			if ext.URy <= ext.LLy {
				t.Errorf("URy=%v should be greater than LLy=%v", ext.URy, ext.LLy)
			}

			t.Logf("glyph 'A' extent: [%.3f, %.3f] × [%.3f, %.3f]",
				ext.LLx, ext.URx, ext.LLy, ext.URy)
		})
	}
}
