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
	"sync"
	"testing"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/font"
	cfffont "seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/truetype"
	type1font "seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/internal/debug/makefont"
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
		CapHeight:          600.0 / 1000,
		XHeight:            400.0 / 1000,
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
			{LLx: 0, LLy: 0, URx: 0, URy: 0},         // .notdef (empty)
			{LLx: 0, LLy: 0, URx: 0, URy: 0},         // space (empty)
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
			checkFloatEqual(t, "CapHeight", expectedGeometry.CapHeight, geometry.CapHeight)
			checkFloatEqual(t, "XHeight", expectedGeometry.XHeight, geometry.XHeight)
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

// TestGetGeometryIsPure checks that GetGeometry only reports what the font
// constructor put there.  Font instances are shared between goroutines, so
// it must not fill in missing values on the fly.
func TestGetGeometryIsPure(t *testing.T) {
	g := &font.Geometry{Ascent: 0.8, Descent: -0.2}
	if got := g.GetGeometry().Leading; got != 0 {
		t.Errorf("GetGeometry modified Leading: got %g, want 0", got)
	}
}

// TestGeometryConcurrentAccess fails under -race if GetGeometry ever writes to
// the receiver again.  Font instances are shared, so concurrent readers are
// normal: standard.Font.New hands the same instance to every caller.
func TestGeometryConcurrentAccess(t *testing.T) {
	F, err := standard.Helvetica.New()
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if F.GetGeometry().Leading <= 0 {
				t.Error("standard font reports no leading")
			}
		})
	}
	wg.Wait()
}

// TestStandardFontLeading checks that the type1 constructor supplies a leading,
// since neither the AFM metrics nor the font program record one.
func TestStandardFontLeading(t *testing.T) {
	for _, f := range standard.All {
		t.Run(string(f), func(t *testing.T) {
			F, err := f.New()
			if err != nil {
				t.Fatal(err)
			}
			geom := F.GetGeometry()
			if math.Abs(geom.Leading-1.2) > eps {
				t.Errorf("Leading = %g, want 1.2", geom.Leading)
			}
		})
	}
}

// TestLeadingWithoutVerticalMetrics checks that fonts whose ascent, descent
// and line gap are all zero still report a usable leading.  Such fonts are
// rare but legal: sfnt supplies no substitute when a font has neither an OS/2
// nor a hhea table, so the constructors have to.
func TestLeadingWithoutVerticalMetrics(t *testing.T) {
	for _, c := range strippedFontCases() {
		t.Run(c.name, func(t *testing.T) {
			F, err := c.make()
			if err != nil {
				t.Fatal(err)
			}
			geom := F.GetGeometry()
			if math.Abs(geom.Leading-1.2) > eps {
				t.Errorf("Leading = %g, want 1.2", geom.Leading)
			}
		})
	}
}

// stripVerticalMetrics removes the vertical metrics from a font, leaving the
// leading which the constructors derive from them at zero.
func stripVerticalMetrics(info *sfnt.Font) *sfnt.Font {
	info.Ascent = 0
	info.Descent = 0
	info.LineGap = 0
	info.CapHeight = 0
	info.XHeight = 0
	return info
}

// TestEstimatedHeightsAreOrdered checks the ordering the estimates assume:
// a font which reports nothing must still come out with
// 0 < XHeight <= CapHeight <= Leading.
func TestEstimatedHeightsAreOrdered(t *testing.T) {
	for _, c := range strippedFontCases() {
		t.Run(c.name, func(t *testing.T) {
			F, err := c.make()
			if err != nil {
				t.Fatal(err)
			}
			geom := F.GetGeometry()
			if geom.XHeight <= 0 {
				t.Errorf("XHeight = %g, want > 0", geom.XHeight)
			}
			if geom.XHeight > geom.CapHeight+eps {
				t.Errorf("XHeight %g exceeds CapHeight %g", geom.XHeight, geom.CapHeight)
			}
			if geom.CapHeight > geom.Leading+eps {
				t.Errorf("CapHeight %g exceeds Leading %g", geom.CapHeight, geom.Leading)
			}
		})
	}
}

// TestType1MeasuresHeights checks that a Type 1 font embedded without metrics
// has its cap height and x-height measured from the "H" and "x" glyphs rather
// than estimated.  The test font is a text face, so the measurements differ
// from the estimates used when no such glyph exists.
func TestType1MeasuresHeights(t *testing.T) {
	psFont := makefont.Type1()

	F, err := type1font.New(psFont, nil)
	if err != nil {
		t.Fatal(err)
	}
	geom := F.GetGeometry()

	wantCap := psFont.CapHeightPDF() / 1000
	wantX := psFont.XHeightPDF() / 1000
	if wantCap <= 0 || wantX <= 0 {
		t.Fatal("test font has no measurable H or x")
	}
	if math.Abs(geom.CapHeight-wantCap) > eps {
		t.Errorf("CapHeight = %g, want %g", geom.CapHeight, wantCap)
	}
	if math.Abs(geom.XHeight-wantX) > eps {
		t.Errorf("XHeight = %g, want %g", geom.XHeight, wantX)
	}
}

// strippedFontCases returns one constructor per sfnt-backed font type, each
// fed a font with its vertical metrics removed.
func strippedFontCases() []struct {
	name string
	make func() (font.Layouter, error)
} {
	cff := func() *sfnt.Font { return stripVerticalMetrics(makefont.OpenType()) }
	ttf := func() *sfnt.Font { return stripVerticalMetrics(makefont.TrueType()) }

	return []struct {
		name string
		make func() (font.Layouter, error)
	}{
		{"cff.NewSimple", func() (font.Layouter, error) { return cfffont.NewSimple(cff(), nil) }},
		{"cff.NewComposite", func() (font.Layouter, error) { return cfffont.NewComposite(cff(), nil) }},
		{"truetype.NewSimple", func() (font.Layouter, error) { return truetype.NewSimple(ttf(), nil) }},
		{"truetype.NewComposite", func() (font.Layouter, error) { return truetype.NewComposite(ttf(), nil) }},
		{"opentype.NewSimple/cff", func() (font.Layouter, error) { return opentype.NewSimple(cff(), nil) }},
		{"opentype.NewComposite/cff", func() (font.Layouter, error) { return opentype.NewComposite(cff(), nil) }},
		{"opentype.NewSimple/glyf", func() (font.Layouter, error) { return opentype.NewSimple(ttf(), nil) }},
		{"opentype.NewComposite/glyf", func() (font.Layouter, error) { return opentype.NewComposite(ttf(), nil) }},
	}
}
