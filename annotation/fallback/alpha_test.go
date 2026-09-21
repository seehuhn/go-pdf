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

	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/form"
)

// transparentAnnotations returns one annotation of each type whose fallback
// appearance carries the file's transparency into the graphics state, each
// with the same transparency set.
func transparentAnnotations(t float64) map[string]annotation.Annotation {
	common := func() annotation.Common {
		return annotation.Common{
			Rect:                    pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 100},
			Color:                   color.DeviceRGB{1, 0, 0},
			Border:                  &annotation.Border{Width: 2},
			StrokingTransparency:    t,
			NonStrokingTransparency: t,
		}
	}
	verts := []float64{10, 10, 100, 10, 100, 60}
	quad := []vec.Vec2{{X: 10, Y: 10}, {X: 100, Y: 10}, {X: 100, Y: 60}, {X: 10, Y: 60}}
	return map[string]annotation.Annotation{
		"Line":     &annotation.Line{Common: common(), Coords: [4]float64{10, 10, 100, 60}},
		"Square":   &annotation.Square{Common: common()},
		"Circle":   &annotation.Circle{Common: common()},
		"Polygon":  &annotation.Polygon{Common: common(), Vertices: verts},
		"PolyLine": &annotation.PolyLine{Common: common(), Vertices: verts},
		"Ink": &annotation.Ink{
			Common:  common(),
			InkList: [][]vec.Vec2{{{X: 10, Y: 10}, {X: 100, Y: 60}}},
		},
		"Caret": &annotation.Caret{Common: common()},
		"Stamp": &annotation.Stamp{Common: common()},
		"TextMarkup": &annotation.TextMarkup{
			Common: common(), Type: annotation.TextMarkupTypeHighlight, QuadPoints: quad,
		},
		"Movie":          &annotation.Movie{Common: common()},
		"Screen":         &annotation.Screen{Common: common()},
		"Text":           &annotation.Text{Common: common()},
		"Sound":          &annotation.Sound{Common: common()},
		"FileAttachment": &annotation.FileAttachment{Common: common()},
		"FreeText": &annotation.FreeText{
			Common: common(), DefaultAppearance: "/Helv 12 Tf 0 g",
		},
		"Link":   &annotation.Link{Common: common()},
		"Widget": &annotation.Widget{Common: common()},
	}
}

// TestAlphaReachesTheAppearance checks that the transparency an annotation
// gives reaches its fallback appearance, that it is applied to the drawing as
// a whole rather than to each paint in it, and that it is rounded on the way.
//
// Carrying it is the generator's job: §12.5.2 says a /CA entry "shall not be
// used if the annotation has an appearance stream", so an appearance which
// leaves the alpha out drops it altogether.
//
// Applying it to the drawing as a whole is what keeps ink painted over ink
// intact, and it is what viewers do; see [Generator.applyAlpha].  The
// appearance is therefore a transparency group painted once at the alpha,
// and not a drawing whose graphics state carries one.
//
// The alpha is 1 minus the transparency the file gave, which for an ordinary
// value such as 0.7 is not exact in binary: written out as it stands it
// becomes /CA 0.30000000000000004, a long number in every file carrying such
// an annotation.
func TestAlphaReachesTheAppearance(t *testing.T) {
	// a variable, not a constant: constant arithmetic is exact, and the
	// whole point is the float64 subtraction the generator performs
	transparency := 0.7
	if 1-transparency == 0.3 {
		t.Fatal("1 - 0.7 is exact here; the test has nothing to catch")
	}

	for name, a := range transparentAnnotations(transparency) {
		t.Run(name, func(t *testing.T) {
			g := newGen(t, pdf.V2_0)
			if err := g.AddAppearance(a); err != nil {
				t.Fatal(err)
			}
			ap := annotation.Resolve(a.GetCommon(), appearance.Normal)
			if ap == nil || ap.Res == nil {
				t.Fatal("the appearance carries no resources")
			}
			// the drawing the alpha is applied to has to be a
			// transparency group, or the alpha reaches each paint in it
			// separately after all
			var group bool
			for _, x := range ap.Res.XObject {
				if f, ok := x.(*form.Form); ok && f.Group != nil {
					group = true
				}
			}
			if !group {
				t.Error("the appearance paints no transparency group")
			}

			// the generator resets other state through a graphics state
			// dictionary of its own, so it is not enough that the
			// appearance has one: the alpha has to be set in it
			var found bool
			for _, gs := range ap.Res.ExtGState {
				if gs.Set&graphics.StateStrokeAlpha == 0 ||
					gs.Set&graphics.StateFillAlpha == 0 {
					continue
				}
				found = true
				if gs.StrokeAlpha != 0.3 {
					t.Errorf("stroke alpha = %v, want 0.3", gs.StrokeAlpha)
				}
				if gs.FillAlpha != 0.3 {
					t.Errorf("fill alpha = %v, want 0.3", gs.FillAlpha)
				}
			}
			if !found {
				t.Error("no graphics state sets the alpha, so the appearance drops it")
			}
		})
	}
}
