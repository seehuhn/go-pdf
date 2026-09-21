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
	"slices"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestCloudyShapeRDRecordsTheBorder checks what a cloudy Circle or Square
// does with its rectangle: the curls bulge outside the border they are drawn
// along, the rectangle grows to take them in, and /RD gives the difference
// back.  This is the case §12.5.6.8 describes, where a border effect pushes
// Rect out beyond the shape itself.
//
// Rect less /RD has to come back as the edge the file gave, both so that the
// shape stays where it was put and so that building the appearance a second
// time leaves it there.
func TestCloudyShapeRDRecordsTheBorder(t *testing.T) {
	const penWidth = 3
	rng := rand.New(rand.NewSource(1))

	for _, shape := range []string{"circle", "square"} {
		t.Run(shape, func(t *testing.T) {
			g := newGen(t, pdf.V2_0)
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)

			for range 100 {
				// coordinates as a file would give them: the appearance
				// bounding box follows the annotation rectangle, so long
				// fractions in the rectangle land in the file twice over
				x := pdf.Round(rng.Float64()*500, 2)
				y := pdf.Round(rng.Float64()*700, 2)
				rect := pdf.Rectangle{LLx: x, LLy: y, URx: x + 120, URy: y + 80}.Round(2)
				margin := []float64{3, 3, 3, 3}
				outer := applyMargins(rect, margin)

				common := annotation.Common{Rect: rect, Color: color.Black}
				style := &annotation.BorderStyle{Width: penWidth}
				effect := &annotation.BorderEffect{Style: "C", Intensity: 2}
				var a annotation.Annotation
				switch shape {
				case "circle":
					a = &annotation.Circle{
						Common:       common,
						Margin:       margin,
						BorderStyle:  style,
						BorderEffect: effect,
						FillColor:    color.DeviceGray(0.5),
					}
				default:
					a = &annotation.Square{
						Common:       common,
						Margin:       margin,
						BorderStyle:  style,
						BorderEffect: effect,
						FillColor:    color.DeviceGray(0.5),
					}
				}

				if err := g.AddAppearance(a); err != nil {
					t.Fatal(err)
				}

				// the border's outer edge is where the file put it
				got := applyMargins(a.GetCommon().Rect, marginOf(t, a))
				if !got.NearlyEqual(&outer, 0.01) {
					t.Fatalf("shape at %g,%g: Rect less RD is %v, want %v",
						x, y, got, outer)
				}

				ink := inkBounds(t, a).Grow(penWidth / 2.0)
				grown := a.GetCommon().Rect

				// the curls reach outside that edge, which is what the
				// rectangle has to grow for
				if ink.URy <= outer.URy || ink.LLx >= outer.LLx {
					t.Fatalf("shape at %g,%g: the cloud covers %v, inside the border edge %v",
						x, y, ink, outer)
				}

				// and the rectangle took them in
				const eps = 1e-6
				if ink.LLx < grown.LLx-eps || ink.LLy < grown.LLy-eps ||
					ink.URx > grown.URx+eps || ink.URy > grown.URy+eps {
					t.Fatalf("shape at %g,%g: the cloud covers %v, outside %v",
						x, y, ink, grown)
				}

				// building the appearance again leaves the annotation where
				// it is: the second run reads back the edge the first one
				// recorded
				before, beforeMargin := grown, slices.Clone(marginOf(t, a))
				if err := g.AddAppearance(a); err != nil {
					t.Fatal(err)
				}
				if after := a.GetCommon().Rect; !after.NearlyEqual(&before, 0.02) {
					t.Fatalf("shape at %g,%g: a second appearance moves Rect to %v from %v",
						x, y, after, before)
				}
				for i, m := range marginOf(t, a) {
					if d := m - beforeMargin[i]; d > 0.02 || d < -0.02 {
						t.Fatalf("shape at %g,%g: a second appearance changes RD to %v from %v",
							x, y, marginOf(t, a), beforeMargin)
					}
				}

				if _, err := a.Encode(rm); err != nil {
					t.Fatalf("shape at %g,%g: %v", x, y, err)
				}
			}
		})
	}
}

// marginOf returns the /RD array of an annotation.
func marginOf(t *testing.T, a annotation.Annotation) []float64 {
	t.Helper()

	switch a := a.(type) {
	case *annotation.Circle:
		return a.Margin
	case *annotation.Square:
		return a.Margin
	default:
		t.Fatalf("%T has no RD", a)
		return nil
	}
}
