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

package font_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug"
)

// TestSpaceIsBlank tests that space characters of common fonts are blank.
func TestSpaceIsBlank(t *testing.T) {
	samples, err := debug.MakeFontSamples()
	if err != nil {
		t.Fatal(err)
	}

	for _, sample := range samples {
		t.Run(sample.Label, func(t *testing.T) {
			data := pdf.NewData(pdf.V1_7)
			F := sample.Font
			E, err := F.Embed(data, "F")
			if err != nil {
				t.Fatal(err)
			}
			gg := E.Layout(" ")
			if len(gg) != 1 {
				t.Fatalf("expected 1 glyph, got %d", len(gg))
			}
			geom := E.GetGeometry()
			if !geom.GlyphExtents[gg[0].GID].IsZero() {
				t.Errorf("expected blank glyph, got %v", geom.GlyphExtents[gg[0].GID])
			}
		})
	}
}
