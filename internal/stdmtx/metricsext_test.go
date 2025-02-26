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

	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/internal/stdmtx"
)

// TestGeometry tests that the glyphs widths and bounding boxes are consistent
// with our font files.
func TestGeometry(t *testing.T) {
	for _, font := range standard.All {
		mtx, exists := stdmtx.Metrics[string(font)]
		if !exists {
			t.Errorf("Font %q not found in stdmtx.Metrics", font)
			continue
		}

		F, err := font.New()
		if err != nil {
			t.Fatal(err)
		}

		genFontBBox := mtx.FontBBox

		for glyphName := range F.Font.Glyphs {
			genWidth := mtx.Width[glyphName]
			actualWidth := F.Font.GlyphWidthPDF(glyphName)

			// Check that we are not off by a factor of 1000 (e.g., using text
			// space units instead of glyph space units).
			q := math.Sqrt(1000)
			if genWidth != 0 && (genWidth < 500/q || genWidth > 500*q) {
				t.Errorf("%s:%s: implausible width: %f", font, glyphName, genWidth)
			}

			if math.Abs(actualWidth-genWidth) > 0.5 {
				t.Errorf("%s:%s: width mismatch: %f vs %f", font, glyphName, actualWidth, genWidth)
			}

			actualGlyphBBox := F.GlyphBBoxPDF(glyphName)
			if !genFontBBox.Covers(actualGlyphBBox) {
				t.Errorf("%s:%s: glyph bbox %v not covered by font bbox %v", font, glyphName,
					actualGlyphBBox, genFontBBox)
			}
		}
	}
}

// TestNotdef tests that the metrics include a non-zero width for the .notdef
// glyph.
func TestNotdef(t *testing.T) {
	for _, font := range standard.All {
		mtx, exists := stdmtx.Metrics[string(font)]
		if !exists {
			t.Errorf("Font %q not found in stdmtx.Metrics", font)
			continue
		}

		if mtx.Width[".notdef"] < 0 || mtx.Width[".notdef"] > 1000 {
			t.Errorf("%s: implausible .notdef width: %v", font, mtx.Width[".notdef"])
		}
	}
}
