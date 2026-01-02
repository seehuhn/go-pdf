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

package font_test

import (
	"fmt"
	"math"
	"testing"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/internal/squarefont"
)

// tolerance for floating point comparisons
const eps = 1e-6

// TestGeometryConsistency validates that all squarefont fonts produce consistent geometry
func TestGeometryConsistency(t *testing.T) {
	// known values for the squarefont
	var expectedGeometry = &font.Geometry{
		Ascent:             800.0 / 1000,
		Descent:            -200.0 / 1000,
		Leading:            1200.0 / 1000,
		UnderlinePosition:  -100.0 / 1000,
		UnderlineThickness: 50.0 / 1000,

		// Expected glyph widths in PDF text space units
		Widths: []float64{
			500.0 / 1000, // .notdef width: 0.5
			250.0 / 1000, // space width: 0.25
			500.0 / 1000, // A width: 0.5
		},

		// Expected glyph extents in text space units
		// Note: empty glyphs (.notdef, space) should have zero extents
		GlyphExtents: []rect.Rect{
			{LLx: 0, LLy: 0, URx: 0, URy: 0},             // .notdef (empty)
			{LLx: 0, LLy: 0, URx: 0, URy: 0},             // space (empty)
			{LLx: 0.1, LLy: 0.2, URx: 0.5, URy: 0.6}, // A (square)
		},
	}

	for _, sample := range squarefont.All {
		t.Run(sample.Label, func(t *testing.T) {
			font := sample.MakeFont()
			if font == nil {
				t.Fatal("MakeFont returned nil")
			}

			geometry := font.GetGeometry()
			if geometry == nil {
				t.Fatal("GetGeometry returned nil")
			}

			// Check font metrics
			checkFloatEqual(t, "Ascent", expectedGeometry.Ascent, geometry.Ascent)
			checkFloatEqual(t, "Descent", expectedGeometry.Descent, geometry.Descent)
			checkFloatEqual(t, "Leading", expectedGeometry.Leading, geometry.Leading)
			checkFloatEqual(t, "UnderlinePosition", expectedGeometry.UnderlinePosition, geometry.UnderlinePosition)
			checkFloatEqual(t, "UnderlineThickness", expectedGeometry.UnderlineThickness, geometry.UnderlineThickness)

			// Check glyph count
			if len(geometry.Widths) != 3 {
				t.Errorf("expected 3 glyph widths, got %d", len(geometry.Widths))
			}
			if len(geometry.GlyphExtents) != 3 {
				t.Errorf("expected 3 glyph extents, got %d", len(geometry.GlyphExtents))
			}

			// Check glyph widths
			for i, expectedWidth := range expectedGeometry.Widths {
				if i < len(geometry.Widths) {
					checkFloatEqual(t, fmt.Sprintf("Width[%d]", i), expectedWidth, geometry.Widths[i])
				}
			}

			// Check glyph extents
			for i, expectedExtent := range expectedGeometry.GlyphExtents {
				if i < len(geometry.GlyphExtents) {
					actual := geometry.GlyphExtents[i]
					checkFloatEqual(t, fmt.Sprintf("GlyphExtents[%d].LLx", i), expectedExtent.LLx, actual.LLx)
					checkFloatEqual(t, fmt.Sprintf("GlyphExtents[%d].LLy", i), expectedExtent.LLy, actual.LLy)
					checkFloatEqual(t, fmt.Sprintf("GlyphExtents[%d].URx", i), expectedExtent.URx, actual.URx)
					checkFloatEqual(t, fmt.Sprintf("GlyphExtents[%d].URy", i), expectedExtent.URy, actual.URy)
				}
			}

			// Log actual values for debugging
			t.Logf("Font %s geometry:", sample.Label)
			t.Logf("  Ascent=%.3f, Descent=%.3f, Leading=%.3f", geometry.Ascent, geometry.Descent, geometry.Leading)
			t.Logf("  UnderlinePosition=%.3f, UnderlineThickness=%.3f", geometry.UnderlinePosition, geometry.UnderlineThickness)
			if len(geometry.Widths) >= 3 {
				t.Logf("  Widths: [%.3f, %.3f, %.3f]", geometry.Widths[0], geometry.Widths[1], geometry.Widths[2])
			}
			if len(geometry.GlyphExtents) >= 3 {
				t.Logf("  Square extent: [%.1f,%.1f]x[%.1f,%.1f]",
					geometry.GlyphExtents[2].LLx, geometry.GlyphExtents[2].URx,
					geometry.GlyphExtents[2].LLy, geometry.GlyphExtents[2].URy)
			}
		})
	}
}

// Helper function to check if two floats are approximately equal
func checkFloatEqual(t *testing.T, field string, expected, actual float64) {
	if math.Abs(expected-actual) > eps {
		t.Errorf("%s: expected %.6f, got %.6f (diff: %.6f)", field, expected, actual, math.Abs(expected-actual))
	}
}
