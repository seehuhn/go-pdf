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
	"math"
	"math/rand"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestCloudyShapeRDDescribesTheShape checks that the /RD of a cloudy Circle
// or Square gives back the shape the annotation was asked for.
//
// /RD records the shape as insets from the annotation rectangle, so the
// rectangle less the insets has to be the shape again.  A rectangle rounded
// to nearest can have an edge land inside the shape, which makes an inset
// negative; an inset clamped to zero instead hides that, and claims a shape
// the caller never asked for.
func TestCloudyShapeRDDescribesTheShape(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	for _, shape := range []string{"circle", "square"} {
		t.Run(shape, func(t *testing.T) {
			g := newGen(t, pdf.V2_0)
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)

			for range 100 {
				x := rng.Float64() * 500
				y := rng.Float64() * 700
				rect := pdf.Rectangle{LLx: x, LLy: y, URx: x + 120, URy: y + 80}

				common := annotation.Common{Rect: rect}
				style := &annotation.BorderStyle{Width: 1}
				effect := &annotation.BorderEffect{Style: "C", Intensity: 2}
				var a annotation.Annotation
				switch shape {
				case "circle":
					a = &annotation.Circle{
						Common:       common,
						BorderStyle:  style,
						BorderEffect: effect,
						FillColor:    color.DeviceGray(0.5),
					}
				default:
					a = &annotation.Square{
						Common:       common,
						BorderStyle:  style,
						BorderEffect: effect,
						FillColor:    color.DeviceGray(0.5),
					}
				}

				if err := g.AddAppearance(a); err != nil {
					t.Fatal(err)
				}
				margin := marginOf(t, a)
				if len(margin) != 4 {
					t.Fatalf("shape at %g,%g has no RD", x, y)
				}
				for i, xi := range margin {
					if xi < 0 {
						t.Fatalf("shape at %g,%g: RD entry %d is %g", x, y, i, xi)
					}
				}

				// the rectangle less the insets is the shape again, which a
				// clamped inset would not give back
				outer := a.GetCommon().Rect
				got := pdf.Rectangle{
					LLx: outer.LLx + margin[0],
					LLy: outer.LLy + margin[1],
					URx: outer.URx - margin[2],
					URy: outer.URy - margin[3],
				}
				for _, d := range []float64{
					got.LLx - rect.LLx, got.LLy - rect.LLy,
					got.URx - rect.URx, got.URy - rect.URy,
				} {
					if math.Abs(d) > 1e-4 {
						t.Fatalf("shape at %g,%g: RD gives %v, want %v", x, y, got, rect)
					}
				}

				if _, err := a.Encode(rm); err != nil {
					t.Fatalf("shape at %g,%g: %v", x, y, err)
				}
			}
		})
	}
}

// marginOf returns the /RD array the generator set on an annotation.
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
