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
	"testing"

	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// spikeHalfWidth makes the corner of [spikeTriangle] at the origin span 20
// degrees, sharp enough for its miter join to reach some five times half the
// line width, but not so sharp that the default miter limit bevels it.
const spikeHalfWidth = 17.6327 // 100*tan(10 degrees)

// spikeTriangle is a triangle with one sharp corner, at the origin.  Its miter
// join there reaches well beyond half the line width, so a bounding box which
// only allows for the line width cuts the spike off.
var spikeTriangle = []vec.Vec2{
	{X: 0, Y: 0},
	{X: 100, Y: spikeHalfWidth},
	{X: 100, Y: -spikeHalfWidth},
}

// spikeTipX is how far the miter join of [spikeTriangle] reaches to the left
// of the sharp corner, for a stroke of width lw.  The miter tip lies lw/2
// divided by the sine of the corner's half-angle away from the vertex, along
// the bisector, which here is the negative x axis.
func spikeTipX(lw float64) float64 {
	return -lw / 2 / math.Sin(math.Atan2(spikeHalfWidth, 100))
}

func TestStrokeBoundsCap(t *testing.T) {
	pts := []vec.Vec2{{X: 10, Y: 20}, {X: 30, Y: 20}}
	got, ok := strokeBounds([][]vec.Vec2{pts}, false, 4, graphics.LineJoinRound, graphics.DefaultMiterLimit)
	if !ok {
		t.Fatal("no bounds for a two-point path")
	}
	want := pdf.Rectangle{LLx: 8, LLy: 18, URx: 32, URy: 22}
	if !got.NearlyEqual(&want, 1e-6) {
		t.Errorf("bounds = %v, want %v", got, want)
	}
}

func TestStrokeBoundsEmpty(t *testing.T) {
	if _, ok := strokeBounds(nil, false, 4, graphics.LineJoinRound, graphics.DefaultMiterLimit); ok {
		t.Error("an empty path has bounds")
	}
	if _, ok := strokeBounds([][]vec.Vec2{{}}, false, 4, graphics.LineJoinRound, graphics.DefaultMiterLimit); ok {
		t.Error("a path with no points has bounds")
	}
}

// TestStrokeBoundsMiterSpike checks that the miter join of a sharp corner is
// inside the bounds, and that a round join, which has no spike, is not
// widened for one.
func TestStrokeBoundsMiterSpike(t *testing.T) {
	const lw = 4

	miter, ok := strokeBounds([][]vec.Vec2{spikeTriangle}, true, lw,
		graphics.LineJoinMiter, graphics.DefaultMiterLimit)
	if !ok {
		t.Fatal("no bounds for the triangle")
	}
	if tip := spikeTipX(lw); miter.LLx > tip+1e-6 {
		t.Errorf("bounds start at x = %g, which cuts off the miter tip at %g",
			miter.LLx, tip)
	}

	round, ok := strokeBounds([][]vec.Vec2{spikeTriangle}, true, lw,
		graphics.LineJoinRound, graphics.DefaultMiterLimit)
	if !ok {
		t.Fatal("no bounds for the triangle")
	}
	if want := -lw / 2.0; math.Abs(round.LLx-want) > 1e-6 {
		t.Errorf("round join bounds start at x = %g, want %g", round.LLx, want)
	}
}

// TestStrokeBoundsMiterLimit checks that a corner too sharp for the miter
// limit falls back to a bevel, which reaches no further than half the line
// width.
func TestStrokeBoundsMiterLimit(t *testing.T) {
	const lw = 4

	got, ok := strokeBounds([][]vec.Vec2{spikeTriangle}, true, lw,
		graphics.LineJoinMiter, 2)
	if !ok {
		t.Fatal("no bounds for the triangle")
	}
	if want := -lw / 2.0; math.Abs(got.LLx-want) > 1e-6 {
		t.Errorf("bounds start at x = %g, want %g: the join has to be bevelled",
			got.LLx, want)
	}
}

// TestStrokeBoundsOpenPathHasNoJoinAtEnds checks that the first and last
// vertex of an open path carry a cap rather than a join.
func TestStrokeBoundsOpenPathHasNoJoinAtEnds(t *testing.T) {
	const lw = 4

	got, ok := strokeBounds([][]vec.Vec2{spikeTriangle}, false, lw,
		graphics.LineJoinMiter, graphics.DefaultMiterLimit)
	if !ok {
		t.Fatal("no bounds for the path")
	}
	if want := -lw / 2.0; math.Abs(got.LLx-want) > 1e-6 {
		t.Errorf("bounds start at x = %g, want %g: an end point has no join",
			got.LLx, want)
	}
}

// appearanceBBox returns the bounding box of an annotation's normal
// appearance.
func appearanceBBox(t *testing.T, a annotation.Annotation) pdf.Rectangle {
	t.Helper()

	ap := annotation.Resolve(a.GetCommon(), appearance.Normal)
	if ap == nil {
		t.Fatal("the annotation has no normal appearance")
	}
	return ap.BBox
}

// TestAppearanceBoundsCoverMiterSpike checks that the four annotation types
// which stroke a vertex path get an appearance whose bounding box covers the
// miter spike of a sharp corner.  A box which cut the spike off would clip the
// stroke where the corner reaches outside it.
func TestAppearanceBoundsCoverMiterSpike(t *testing.T) {
	const lw = 4
	tip := spikeTipX(lw)

	verts := []float64{}
	for _, p := range spikeTriangle {
		verts = append(verts, p.X, p.Y)
	}

	common := func() annotation.Common {
		return annotation.Common{
			Color:  color.DeviceRGB{1, 0, 0},
			Border: &annotation.Border{Width: lw},
		}
	}

	cases := map[string]annotation.Annotation{
		"Polygon":  &annotation.Polygon{Common: common(), Vertices: verts},
		"PolyLine": &annotation.PolyLine{Common: common(), Vertices: verts},
		"Ink":      &annotation.Ink{Common: common(), InkList: [][]vec.Vec2{spikeTriangle}},
		"Line": &annotation.Line{
			Common: common(),
			Coords: [4]float64{spikeTriangle[0].X, spikeTriangle[0].Y, spikeTriangle[1].X, spikeTriangle[1].Y},
		},
	}

	for name, a := range cases {
		t.Run(name, func(t *testing.T) {
			g := newGen(t, pdf.V2_0)
			if err := g.AddAppearance(a); err != nil {
				t.Fatal(err)
			}

			rect := a.GetCommon().Rect
			if rect.IsZero() {
				t.Fatal("the annotation was left without a rectangle")
			}
			bbox := appearanceBBox(t, a)
			if !bbox.NearlyEqual(&rect, 0.01) {
				t.Errorf("appearance box %v does not match the rectangle %v", bbox, rect)
			}

			// only a closed path has a join at the sharp corner; every other
			// case caps it, and half the line width is all it needs
			want := -lw/2.0 + 0.01
			if name == "Polygon" {
				want = tip + 0.01
			}
			if rect.LLx > want {
				t.Errorf("rectangle starts at x = %g, want at most %g", rect.LLx, want)
			}
		})
	}
}

// TestPolygonWithTwoVertices checks that a polygon with only two vertices,
// which the file may legitimately carry, still gets a rectangle covering its
// stroke.  Nothing about the path needs three points.
func TestPolygonWithTwoVertices(t *testing.T) {
	const lw = 4
	a := &annotation.Polygon{
		Common: annotation.Common{
			Color:  color.DeviceRGB{1, 0, 0},
			Border: &annotation.Border{Width: lw},
		},
		Vertices: []float64{10, 20, 60, 20},
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	want := pdf.Rectangle{LLx: 8, LLy: 18, URx: 62, URy: 22}
	if !a.Rect.NearlyEqual(&want, 0.01) {
		t.Errorf("rect = %v, want %v", a.Rect, want)
	}
	if n := countStrokes(t, a); n == 0 {
		t.Error("the polygon stroked nothing")
	}
}

// TestRoundOutKeepsTwoDecimalValues checks that roundOut leaves edges alone
// which already have two decimals, including those, like 0.29, whose
// product with 100 is not a whole number in floating point.
func TestRoundOutKeepsTwoDecimalValues(t *testing.T) {
	for i := -100000; i <= 100000; i++ {
		x := float64(i) / 100
		r := pdf.Rectangle{LLx: x, LLy: x, URx: x, URy: x}
		if got := roundOut(r); got != r {
			t.Fatalf("roundOut(%v) = %v, want unchanged", r, got)
		}
	}
}

// TestRoundOutContains checks that roundOut returns the smallest rectangle
// with two-decimal edges which contains its argument.
func TestRoundOutContains(t *testing.T) {
	for _, x := range []float64{0.289, 0.2899999999, 0.291, 9.625, 20.375, -0.071, -3.005, 1e-9, -1e-9} {
		r := pdf.Rectangle{LLx: x, LLy: x, URx: x, URy: x}
		got := roundOut(r)
		for _, v := range []float64{got.LLx, got.LLy, got.URx, got.URy} {
			if pdf.Round(v, 2) != v {
				t.Errorf("roundOut(%g): edge %v has more than two decimals", x, v)
			}
		}
		if got.LLx > x || got.LLy > x || got.URx < x || got.URy < x {
			t.Errorf("roundOut(%g) = %v does not contain it", x, got)
		}
		if got.LLx+0.01 <= x || got.URx-0.01 >= x {
			t.Errorf("roundOut(%g) = %v is not the smallest", x, got)
		}
	}
}

// TestAppearanceBoundsRoundOutwards checks that a rectangle derived from a
// stroke holds it where half the line width puts an edge exactly between two
// hundredths.  Rounding to nearest takes such an edge away from zero, which
// on the lower-left side is into the stroke.
func TestAppearanceBoundsRoundOutwards(t *testing.T) {
	const lw = 0.75
	pts := []vec.Vec2{{X: 10, Y: 10}, {X: 20, Y: 10}, {X: 20, Y: 20}}
	verts := []float64{10, 10, 20, 10, 20, 20}

	common := func() annotation.Common {
		return annotation.Common{
			Color:  color.DeviceRGB{1, 0, 0},
			Border: &annotation.Border{Width: lw},
		}
	}
	cases := map[string]annotation.Annotation{
		"Polygon":  &annotation.Polygon{Common: common(), Vertices: verts},
		"PolyLine": &annotation.PolyLine{Common: common(), Vertices: verts},
		"Ink":      &annotation.Ink{Common: common(), InkList: [][]vec.Vec2{pts}},
		"Line":     &annotation.Line{Common: common(), Coords: [4]float64{10, 10, 20, 20}},
	}

	for name, a := range cases {
		t.Run(name, func(t *testing.T) {
			g := newGen(t, pdf.V2_0)
			if err := g.AddAppearance(a); err != nil {
				t.Fatal(err)
			}
			rect := a.GetCommon().Rect
			if rect.LLx > 10-lw/2 || rect.LLy > 10-lw/2 || rect.URx < 20+lw/2 || rect.URy < 20+lw/2 {
				t.Errorf("rectangle %v does not hold the stroke", rect)
			}
		})
	}
}

// TestInkBoundsAreTight checks that the rectangle of an ink annotation, whose
// points are drawn exactly as rounded, is the smallest one with two-decimal
// edges that holds the stroke.
func TestInkBoundsAreTight(t *testing.T) {
	a := &annotation.Ink{
		Common: annotation.Common{
			Color:  color.DeviceRGB{1, 0, 0},
			Border: &annotation.Border{Width: 0.75},
		},
		InkList: [][]vec.Vec2{{{X: 10, Y: 10}, {X: 20, Y: 10}, {X: 20, Y: 20}}},
	}
	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}
	want := pdf.Rectangle{LLx: 9.62, LLy: 9.62, URx: 20.38, URy: 20.38}
	if a.Rect != want {
		t.Errorf("rectangle = %v, want %v", a.Rect, want)
	}
}

// TestPolyLineMiterAfterRounding checks the rectangle of a polyline whose
// corner is just too sharp for a miter join, but which rounding the vertices
// to two decimals opens up far enough to take one.  The rectangle must hold
// the miter drawn.
func TestPolyLineMiterAfterRounding(t *testing.T) {
	const lw = 10
	s := 0.1985 // rounds to 0.2
	c := math.Sqrt(1 - s*s)
	a := &annotation.PolyLine{
		Common: annotation.Common{
			Color:  color.DeviceRGB{1, 0, 0},
			Border: &annotation.Border{Width: lw},
		},
		Vertices: []float64{1, 0, 0, 0, c, s},
	}

	drawn := []vec.Vec2{{X: 1, Y: 0}, {X: 0, Y: 0}, {X: pdf.Round(c, 2), Y: pdf.Round(s, 2)}}
	want, _ := strokeBounds([][]vec.Vec2{drawn}, false, lw,
		graphics.LineJoinMiter, graphics.DefaultMiterLimit)
	if want.LLx > -lw {
		t.Fatalf("test case gives no miter spike: %v", want)
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}
	r := a.Rect
	if r.LLx > want.LLx || r.LLy > want.LLy || r.URx < want.URx || r.URy < want.URy {
		t.Errorf("rectangle %v does not hold the stroke %v", a.Rect, want)
	}
}

// TestVertexRectMatchesGenerator checks that PolygonRect and PolyLineRect
// give the rectangle the generator derives for the same annotation, and
// report an annotation with too few vertices.
func TestVertexRectMatchesGenerator(t *testing.T) {
	verts := []float64{10, 10, 20, 10.004, 10, 12.196}
	common := func() annotation.Common {
		return annotation.Common{
			Color:  color.DeviceRGB{1, 0, 0},
			Border: &annotation.Border{Width: 3},
		}
	}

	pg := &annotation.Polygon{Common: common(), Vertices: verts}
	want, ok := PolygonRect(pg)
	if !ok {
		t.Fatal("PolygonRect found no vertices")
	}
	if err := newGen(t, pdf.V2_0).AddAppearance(pg); err != nil {
		t.Fatal(err)
	}
	if pg.Rect != want {
		t.Errorf("polygon: generator Rect = %v, PolygonRect = %v", pg.Rect, want)
	}

	pl := &annotation.PolyLine{
		Common:          common(),
		Vertices:        verts,
		LineEndingStyle: [2]annotation.LineEndingStyle{annotation.LineEndingStyleClosedArrow, annotation.LineEndingStyleCircle},
	}
	want, ok = PolyLineRect(pl)
	if !ok {
		t.Fatal("PolyLineRect found no vertices")
	}
	if err := newGen(t, pdf.V2_0).AddAppearance(pl); err != nil {
		t.Fatal(err)
	}
	if pl.Rect != want {
		t.Errorf("polyline: generator Rect = %v, PolyLineRect = %v", pl.Rect, want)
	}

	if _, ok := PolygonRect(&annotation.Polygon{Vertices: []float64{1, 2}}); ok {
		t.Error("PolygonRect accepted a single vertex")
	}
	if _, ok := PolyLineRect(&annotation.PolyLine{Vertices: []float64{1, 2}}); ok {
		t.Error("PolyLineRect accepted a single vertex")
	}
}
