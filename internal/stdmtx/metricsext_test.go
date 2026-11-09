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

package stdmtx_test

import (
	"math"
	"testing"

	"seehuhn.de/go/postscript/afm"

	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// TestGeometry tests that the glyphs widths and bounding boxes are consistent
// with our font files.
func TestGeometry(t *testing.T) {
	for _, f := range standard.All {
		mtx, exists := stdmtx.Metrics[string(f)]
		if !exists {
			t.Errorf("Font %q not found in stdmtx.Metrics", f)
			continue
		}

		F := font.Must(f.New())

		genFontBBox := mtx.FontBBox

		for gid, glyphName := range F.GlyphNames {
			genWidth := mtx.Width[glyphName]
			actualWidth := math.Round(F.Widths[gid] * 1000)

			// Check that we are not off by a factor of 1000 (e.g., using text
			// space units instead of glyph space units).
			q := math.Sqrt(1000)
			if genWidth != 0 && (genWidth < 500/q || genWidth > 500*q) {
				t.Errorf("%s:%s: implausible width: %f", f, glyphName, genWidth)
			}

			if math.Abs(actualWidth-genWidth) > 0.5 {
				t.Errorf("%s:%s: width mismatch: %f vs %f", f, glyphName, actualWidth, genWidth)
			}

			// the extents are in text space; the glyph coordinates are integers
			e := F.GlyphExtents[gid]
			actualGlyphBBox := rect.Rect{
				LLx: math.Round(e.LLx * 1000),
				LLy: math.Round(e.LLy * 1000),
				URx: math.Round(e.URx * 1000),
				URy: math.Round(e.URy * 1000),
			}
			if !genFontBBox.Covers(actualGlyphBBox) {
				t.Errorf("%s:%s: glyph bbox %v not covered by font bbox %v", f, glyphName,
					actualGlyphBBox, genFontBBox)
			}
		}
	}
}

// TestNotdef tests that the metrics give the .notdef width of the standard
// font instances.
func TestNotdef(t *testing.T) {
	for _, f := range standard.All {
		mtx, exists := stdmtx.Metrics[string(f)]
		if !exists {
			t.Errorf("font %q not found in stdmtx.Metrics", f)
			continue
		}

		want := font.Must(f.New()).GetGeometry().Widths[0] * 1000
		if got, ok := mtx.Width[".notdef"]; !ok || got <= 0 || got != want {
			t.Errorf("%s: .notdef width %v, want %v", f, got, want)
		}
	}
}

// TestAdobeWidths tests that the widths are those in Adobe's AFM files.
func TestAdobeWidths(t *testing.T) {
	l := loader.NewFontLoader()
	for _, f := range standard.All {
		name := string(f)
		r, err := l.Open(name, loader.FontTypeAFM)
		if err != nil {
			t.Fatal(err)
		}
		metrics, err := afm.Read(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}

		mtx := stdmtx.Metrics[name]
		// the table covers the glyphs of the standard font instance
		for _, glyphName := range font.Must(f.New()).GlyphNames {
			if glyphName == ".notdef" {
				continue
			}
			want := metrics.Glyphs[glyphName].WidthX
			if got, ok := mtx.Width[glyphName]; !ok || got != want {
				t.Errorf("%s:%s: width %v, want %v", name, glyphName, got, want)
			}
		}
		for glyphName, got := range mtx.Width {
			if g, ok := metrics.Glyphs[glyphName]; ok && got != g.WidthX {
				t.Errorf("%s:%s: width %v, want %v", name, glyphName, got, g.WidthX)
			}
		}
	}

	// the width of "fraction" in Helvetica is 167, not 278
	if got := stdmtx.Metrics["Helvetica"].Width["fraction"]; got != 167 {
		t.Errorf("Helvetica fraction: width %v, want 167", got)
	}
}
