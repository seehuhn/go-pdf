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

package fallback

import (
	"math/rand"
	"testing"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestCalloutNegativeRD checks that the generator's outer rectangle still
// contains the text box after it has been rounded.
//
// /RD gives the text box as insets from the annotation rectangle, and an
// inset may not be negative.  A rectangle rounded to nearest can have an edge
// land inside the text box, which makes the corresponding inset negative and
// the annotation impossible to write.
func TestCalloutNegativeRD(t *testing.T) {
	// the right edge lies just below a rounding boundary, so rounding to
	// nearest moves it inwards, past the text box's own right edge
	a := &annotation.FreeText{
		Common: annotation.Common{
			Rect:     pdf.Rectangle{LLx: 100.1158, LLy: 200, URx: 280.1142, URy: 240},
			Contents: "a line of text",
			Border:   &annotation.Border{Width: 1},
		},
		Markup:            annotation.Markup{Intent: annotation.FreeTextIntentCallout},
		DefaultAppearance: "/Helv 12 Tf 0 0 0 rg",
		CalloutLine: []vec.Vec2{
			{X: 40, Y: 300}, {X: 70, Y: 220}, {X: 100.1158, Y: 220},
		},
		LineEndingStyle: annotation.LineEndingStyleOpenArrow,
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}
	for i, xi := range a.Margin {
		if xi < 0 {
			t.Errorf("RD entry %d is %g", i, xi)
		}
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	if _, err := a.Encode(rm); err != nil {
		t.Fatalf("Rect=%v Margin=%v: %v", a.Rect, a.Margin, err)
	}
}

// TestCalloutRDAtArbitraryCoordinates checks the same for callouts placed at
// coordinates with arbitrary fractional parts, which is what a viewer sends
// when it divides a mouse position by the zoom factor.
func TestCalloutRDAtArbitraryCoordinates(t *testing.T) {
	g := newGen(t, pdf.V2_0)
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	rng := rand.New(rand.NewSource(1))
	for range 200 {
		x := rng.Float64() * 500
		y := rng.Float64() * 700
		a := &annotation.FreeText{
			Common: annotation.Common{
				Rect:     pdf.Rectangle{LLx: x, LLy: y, URx: x + 180, URy: y + 40},
				Contents: "a line of text",
				Border:   &annotation.Border{Width: 1},
			},
			Markup:            annotation.Markup{Intent: annotation.FreeTextIntentCallout},
			DefaultAppearance: "/Helv 12 Tf 0 0 0 rg",
			CalloutLine: []vec.Vec2{
				{X: x - 60, Y: y + 100}, {X: x - 30, Y: y + 20}, {X: x, Y: y + 20},
			},
			LineEndingStyle: annotation.LineEndingStyleOpenArrow,
		}

		if err := g.AddAppearance(a); err != nil {
			t.Fatal(err)
		}
		for i, xi := range a.Margin {
			if xi < 0 {
				t.Fatalf("box at %g,%g: RD entry %d is %g", x, y, i, xi)
			}
		}
		if _, err := a.Encode(rm); err != nil {
			t.Fatalf("box at %g,%g: %v", x, y, err)
		}
	}
}
