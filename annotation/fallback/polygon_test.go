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
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/color"
)

// TestPolygonWithoutRect checks that a polygon whose annotation rectangle is
// missing still gets an appearance which can be written out: the vertices say
// where it lies, and a form with a zero bounding box cannot be embedded.
func TestPolygonWithoutRect(t *testing.T) {
	a := &annotation.Polygon{
		Common: annotation.Common{
			Color:  color.DeviceRGB{1, 0, 0},
			Border: &annotation.Border{Width: 1},
		},
		Vertices: []float64{10, 10, 60, 10, 35, 50},
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	if a.Rect.IsZero() {
		t.Fatal("the polygon was left without a rectangle")
	}
	for i := 0; i+1 < len(a.Vertices); i += 2 {
		x, y := a.Vertices[i], a.Vertices[i+1]
		if x < a.Rect.LLx || x > a.Rect.URx || y < a.Rect.LLy || y > a.Rect.URy {
			t.Errorf("vertex (%g, %g) lies outside rect %v", x, y, a.Rect)
		}
	}
	if n := countStrokes(t, a); n == 0 {
		t.Error("the polygon stroked nothing")
	}
}
