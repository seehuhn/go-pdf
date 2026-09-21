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
	"strings"
	"testing"

	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/graphics/color"
)

// strokedAnnotations returns one annotation of each type whose fallback
// appearance is a path drawn in the annotation's own colour, built with the
// given Common.
func strokedAnnotations(c annotation.Common) map[string]annotation.Annotation {
	verts := []float64{10, 10, 100, 10, 100, 60}
	ink := [][]vec.Vec2{{{X: 10, Y: 10}, {X: 100, Y: 10}, {X: 100, Y: 60}}}
	return map[string]annotation.Annotation{
		"Line":     &annotation.Line{Common: c, Coords: [4]float64{10, 10, 100, 60}},
		"Square":   &annotation.Square{Common: c},
		"Circle":   &annotation.Circle{Common: c},
		"Polygon":  &annotation.Polygon{Common: c, Vertices: verts},
		"PolyLine": &annotation.PolyLine{Common: c, Vertices: verts},
		"Ink":      &annotation.Ink{Common: c, InkList: ink},
	}
}

// TestStrokeColorFollowsColor checks that a stroked annotation is drawn in
// the colour its /C entry names, rather than in a colour of the generator's
// own choosing.
func TestStrokeColorFollowsColor(t *testing.T) {
	col := color.DeviceRGB{0.85, 0.15, 0.15}
	common := annotation.Common{
		Rect:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 100},
		Color:  col,
		Border: &annotation.Border{Width: 2},
	}

	for name, a := range strokedAnnotations(common) {
		t.Run(name, func(t *testing.T) {
			g := newGen(t, pdf.V2_0)
			if err := g.AddAppearance(a); err != nil {
				t.Fatal(err)
			}
			got := string(appearanceStream(t, a))
			want := "0.85 0.15 0.15 RG"
			if !strings.Contains(got, want) {
				t.Errorf("appearance does not set the stroke colour to %q:\n%s", want, got)
			}
		})
	}
}

// TestNoColorDrawsNoStroke checks that an annotation whose colour array is
// absent or empty strokes nothing: the file names no ink, and the generator
// invents none.
func TestNoColorDrawsNoStroke(t *testing.T) {
	for _, tc := range []struct {
		name string
		col  color.Color
	}{
		{"absent", nil},
		{"empty", colorenc.Transparent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			common := annotation.Common{
				Rect:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 100},
				Color:  tc.col,
				Border: &annotation.Border{Width: 2},
			}
			for name, a := range strokedAnnotations(common) {
				t.Run(name, func(t *testing.T) {
					g := newGen(t, pdf.V2_0)
					if err := g.AddAppearance(a); err != nil {
						t.Fatal(err)
					}
					if n := countStrokes(t, a); n != 0 {
						t.Errorf("drew %d strokes, want none", n)
					}
				})
			}
		})
	}
}
