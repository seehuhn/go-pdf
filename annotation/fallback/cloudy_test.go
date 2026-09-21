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

	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/polygon"
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// CCW unit square
var unitSquare = []vec.Vec2{
	{X: 0, Y: 0},
	{X: 100, Y: 0},
	{X: 100, Y: 100},
	{X: 0, Y: 100},
}

// CCW 100x60 rectangle
var wideRectangle = []vec.Vec2{
	{X: 0, Y: 0},
	{X: 100, Y: 0},
	{X: 100, Y: 60},
	{X: 0, Y: 60},
}

func hasCurveOps(ops []content.Operator) bool {
	for _, op := range ops {
		switch op.Name {
		case content.OpCurveTo, content.OpCurveToV, content.OpCurveToY:
			return true
		}
	}
	return false
}

func hasOp(ops []content.Operator, name content.OpName) bool {
	for _, op := range ops {
		if op.Name == name {
			return true
		}
	}
	return false
}

func TestCloudyBorderSmallPolygon(t *testing.T) {
	// tiny polygon too small for 3 cloud bulges
	tiny := []vec.Vec2{
		{X: 0, Y: 0},
		{X: 2, Y: 0},
		{X: 1, Y: 1.7},
	}
	b := builder.New(content.Form, nil, pdf.V2_0)
	drawCloudyBorder(b, tiny, 1, 1, true, true)

	if hasCurveOps(b.Stream) {
		t.Error("expected no CurveTo ops for small polygon fallback")
	}
	if !hasOp(b.Stream, content.OpMoveTo) {
		t.Error("expected MoveTo in plain polygon")
	}
}

func TestCloudyBorderRectangle(t *testing.T) {
	b := builder.New(content.Form, nil, pdf.V2_0)
	bbox := drawCloudyBorder(b, unitSquare, 1, 1, true, true)

	if !hasCurveOps(b.Stream) {
		t.Error("expected CurveTo ops for cloudy border")
	}
	if bbox.IsZero() {
		t.Error("expected non-zero bbox")
	}
}

func TestCloudyBorderCWReversal(t *testing.T) {
	// CW version of unit square
	cw := []vec.Vec2{
		{X: 0, Y: 0},
		{X: 0, Y: 100},
		{X: 100, Y: 100},
		{X: 100, Y: 0},
	}

	b1 := builder.New(content.Form, nil, pdf.V2_0)
	bbox1 := drawCloudyBorder(b1, unitSquare, 1, 1, true, false)

	b2 := builder.New(content.Form, nil, pdf.V2_0)
	bbox2 := drawCloudyBorder(b2, cw, 1, 1, true, false)

	// bounding boxes should be similar (tolerance accounts for bulge placement shift)
	if math.Abs(bbox1.Dx()-bbox2.Dx()) > 5 || math.Abs(bbox1.Dy()-bbox2.Dy()) > 5 {
		t.Errorf("CW and CCW bboxes differ: %v vs %v", bbox1, bbox2)
	}
}

func TestCloudyBorderBBox(t *testing.T) {
	b := builder.New(content.Form, nil, pdf.V2_0)
	bbox := drawCloudyBorder(b, unitSquare, 1, 1, true, false)

	// bbox must include the polygon
	if bbox.URx < 100 || bbox.URy < 100 {
		t.Errorf("bbox should include polygon: %v", bbox)
	}
	// cloud bulges extend beyond the polygon
	if bbox.Dx() <= 100 || bbox.Dy() <= 100 {
		t.Errorf("bbox should be larger than polygon: %v", bbox)
	}
}

func TestCloudyBorderTriangle(t *testing.T) {
	tri := []vec.Vec2{
		{X: 50, Y: 0},
		{X: 100, Y: 86.6},
		{X: 0, Y: 86.6},
	}
	b := builder.New(content.Form, nil, pdf.V2_0)
	bbox := drawCloudyBorder(b, tri, 1, 1, true, true)

	if !hasCurveOps(b.Stream) {
		t.Error("expected CurveTo ops for triangle")
	}
	if bbox.IsZero() {
		t.Error("expected non-zero bbox")
	}
}

func TestCloudOutlineRectangle(t *testing.T) {
	fill, stroke, ok := CloudOutline(wideRectangle, 1, 1)
	if !ok {
		t.Fatal("expected ok for 100x60 rectangle")
	}

	var lastCmd path.Command
	var sawCmd bool
	for cmd := range fill {
		lastCmd = cmd
		sawCmd = true
	}
	if !sawCmd || lastCmd != path.CmdClose {
		t.Errorf("expected fill path to be closed, last command %v", lastCmd)
	}

	want := rect.Rect{LLx: 0, LLy: 0, URx: 100, URy: 60}

	fillBBox := fill.BBox()
	if !fillBBox.Covers(want) {
		t.Errorf("fill bbox %v does not contain rectangle %v", fillBBox, want)
	}

	strokeBBox := stroke.BBox()
	if !strokeBBox.Covers(want) {
		t.Errorf("stroke bbox %v does not contain rectangle %v", strokeBBox, want)
	}
}

// TestCloudOutlineTooSmall checks that a shape with no room for a border
// gets no cloud.  What counts is the pen against the shape, not the shape
// on its own: the rule is scale free, so a 2 by 2 box with a 2 point pen is
// the same case as a 200 by 200 box with a 200 point one.
func TestCloudOutlineTooSmall(t *testing.T) {
	tiny := []vec.Vec2{
		{X: 0, Y: 0},
		{X: 2, Y: 0},
		{X: 2, Y: 2},
		{X: 0, Y: 2},
	}
	_, _, ok := CloudOutline(tiny, 1, 2)
	if ok {
		t.Error("expected !ok for a 2x2 rectangle with a 2pt pen")
	}
}

func TestCloudOutlineTrimToCloud(t *testing.T) {
	co := newCloudOutline(wideRectangle, 1, 1)
	if co == nil {
		t.Fatal("expected a cloud outline for the 100x60 rectangle")
	}

	// left side's midpoint: outside the box, but within bulge reach
	got := co.trimToCloud(vec.Vec2{X: -40, Y: 30}, vec.Vec2{X: 0, Y: 30})
	if got.X >= 0 {
		t.Errorf("expected trimmed point outside the box, got %v", got)
	}
	if got.X <= -20 {
		t.Errorf("expected trimmed point within bulge reach, got %v", got)
	}
	if math.Abs(got.Y-30) > 1 {
		t.Errorf("expected trimmed point near y=30, got %v", got)
	}

	// a segment fully inside the cloud, away from the outline
	inside := vec.Vec2{X: 60, Y: 35}
	got2 := co.trimToCloud(vec.Vec2{X: 40, Y: 25}, inside)
	if got2 != inside {
		t.Errorf("expected inside point unchanged, got %v", got2)
	}
}

// TestCloudOutlineSampleLimit checks that a polygon with coordinates far
// beyond any page does not make the generator sample an unbounded number of
// boundary points, or emit a content stream to match.  The spacing follows
// the line width, so without the limit both would grow with the coordinates
// while the file stayed the same size.
func TestCloudOutlineSampleLimit(t *testing.T) {
	for _, size := range []float64{1e5, 1e10, 1e30, 1e100} {
		verts := []vec.Vec2{
			{X: 0, Y: 0},
			{X: size, Y: 0},
			{X: size, Y: size},
			{X: 0, Y: size},
		}

		co := newCloudOutline(verts, 1, 1)
		if co == nil {
			t.Errorf("size %g: no cloud outline", size)
			continue
		}
		if len(co.points) > maxSamplePoints {
			t.Errorf("size %g: sampled %d points, limit is %d",
				size, len(co.points), maxSamplePoints)
		}

		segments := 0
		for range co.fill() {
			segments++
		}
		if segments > maxSamplePoints {
			t.Errorf("size %g: fill path has %d segments", size, segments)
		}
	}
}

// TestCloudOutlineDegenerate checks that a polygon which encloses nothing is
// rejected rather than dividing by a zero perimeter.
func TestCloudOutlineDegenerate(t *testing.T) {
	cases := map[string][]vec.Vec2{
		"no vertices":         nil,
		"one vertex":          {{X: 7, Y: 7}},
		"two vertices":        {{X: 0, Y: 0}, {X: 100, Y: 0}},
		"coincident vertices": {{X: 7, Y: 7}, {X: 7, Y: 7}, {X: 7, Y: 7}},
	}
	for name, verts := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, ok := CloudOutline(verts, 1, 1); ok {
				t.Error("expected no cloud")
			}
		})
	}
}

// cloudRect is a w by h rectangle, counter-clockwise from the origin.
func cloudRect(w, h float64) []vec.Vec2 {
	return []vec.Vec2{{X: 0, Y: 0}, {X: w, Y: 0}, {X: w, Y: h}, {X: 0, Y: h}}
}

// TestCloudBulgeCountFallsWithTheLineWidth checks that a thicker pen draws
// fewer, larger bulges all the way up, rather than the count freezing once
// the pen passes some width: bulges no bigger than the line they are drawn
// with read as mush.
func TestCloudBulgeCountFallsWithTheLineWidth(t *testing.T) {
	verts := cloudRect(200, 100)
	prev := 0
	for i, lw := range []float64{1, 2, 4, 8, 16, 32} {
		co := newCloudOutline(verts, 1.5, lw)
		if co == nil {
			t.Fatalf("width %v: no cloud, but the border is far from filling the shape", lw)
		}
		n := co.numBulges()
		if i > 0 && n > prev {
			t.Errorf("width %v draws %d bulges, more than the %d at the width before",
				lw, n, prev)
		}
		if n < minBulges {
			t.Errorf("width %v draws %d bulges, fewer than the %d a cloud needs",
				lw, n, minBulges)
		}
		prev = n
	}
	if prev >= 10 {
		t.Errorf("the count never fell: %d bulges at the widest pen", prev)
	}
}

// TestCloudBulgesShrinkToFit checks that an outline with no room for
// [minBulges] of the size the pen and the intensity ask for gets that many
// smaller ones, rather than no cloud at all.  Only a border too wide for the
// shape gives a cloud up, which is what the next test covers.
func TestCloudBulgesShrinkToFit(t *testing.T) {
	// a small rectangle at the highest intensity: the bulge asked for is
	// longer than the whole outline
	co := newCloudOutline(cloudRect(40, 25), 2, 4)
	if co == nil {
		t.Fatal("no cloud, though the border is well inside what the shape can carry")
	}
	if got := co.numBulges(); got != minBulges {
		t.Errorf("got %d bulges, want %d", got, minBulges)
	}
}

// TestCloudGivesUpWhenTheBorderFillsTheShape checks the one thing a cloud is
// given up for: a border taking more than [cloudGiveUp] of the room the shape
// has for one leaves too little inside to read as a shape with a border round
// it.  The rule depends on the shape and the pen alone, not on the intensity,
// and it follows [cloudSize] rather than the shorter side, which puts the
// limit far too low for a long thin shape.
func TestCloudGivesUpWhenTheBorderFillsTheShape(t *testing.T) {
	for _, size := range [][2]float64{{200, 100}, {100, 50}, {40, 25}, {150, 150}, {500, 50}} {
		verts := cloudRect(size[0], size[1])
		limit := cloudGiveUp * cloudSize(verts, polygon.Perimeter(verts))
		for _, intensity := range []float64{0.5, 1, 1.5, 2} {
			if co := newCloudOutline(verts, intensity, limit*0.98); co == nil {
				t.Errorf("%v at intensity %v: no cloud just inside the limit %.2f",
					size, intensity, limit)
			}
			if co := newCloudOutline(verts, intensity, limit*1.02); co != nil {
				t.Errorf("%v at intensity %v: still a cloud past the limit %.2f",
					size, intensity, limit)
			}
		}
	}
}

// TestCloudSimplifiesBeforeItGivesUp checks that the two steps come in the
// right order and stay apart: the ticks go first, and the cloud itself is
// kept for a good stretch of pen widths after that.
func TestCloudSimplifiesBeforeItGivesUp(t *testing.T) {
	if cloudTickLimit >= cloudGiveUp {
		t.Fatalf("ticks dropped at %v, cloud given up at %v: no simplified cloud at all",
			cloudTickLimit, cloudGiveUp)
	}
	if cloudGiveUp/cloudTickLimit < 1.25 {
		t.Errorf("the simplified cloud covers only %.2fx in pen width",
			cloudGiveUp/cloudTickLimit)
	}
}

// countMoveTo counts the subpaths of p: the cusp ticks are drawn as separate
// subpaths, so this is how many of them a stroke path carries.
func countMoveTo(p path.Path) int {
	n := 0
	for cmd := range p {
		if cmd == path.CmdMoveTo {
			n++
		}
	}
	return n
}

// TestCloudDropsCuspTicksWhenThePenIsWide checks that the small ticks drawn
// across each cusp are dropped once the pen is wide enough to blot them into
// the bulges they sit between, leaving a cloud of plain curls.  The width
// this happens at follows the room the shape has for a border, not the
// intensity, which changes the size of the bulges rather than the size of
// the shape they are drawn round.
func TestCloudDropsCuspTicksWhenThePenIsWide(t *testing.T) {
	verts := cloudRect(150, 150)
	limit := cloudTickLimit * cloudSize(verts, polygon.Perimeter(verts))

	for _, intensity := range []float64{0.5, 1, 1.5, 2} {
		narrow := newCloudOutline(verts, intensity, limit*0.98)
		if narrow == nil {
			t.Fatalf("intensity %v: no cloud just inside the limit %.2f", intensity, limit)
		}
		if countMoveTo(narrow.stroke()) < 2 {
			t.Errorf("intensity %v: no cusp ticks just inside the limit %.2f",
				intensity, limit)
		}

		wide := newCloudOutline(verts, intensity, limit*1.02)
		if wide == nil {
			t.Fatalf("intensity %v: no cloud just past the limit %.2f", intensity, limit)
		}
		if got := countMoveTo(wide.stroke()); got != 1 {
			t.Errorf("intensity %v: %d subpaths just past the limit %.2f, want the ticks gone",
				intensity, got, limit)
		}
	}
}
