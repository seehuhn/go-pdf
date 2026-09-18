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

func TestCloudOutlineTooSmall(t *testing.T) {
	tiny := []vec.Vec2{
		{X: 0, Y: 0},
		{X: 2, Y: 0},
		{X: 2, Y: 2},
		{X: 0, Y: 2},
	}
	_, _, ok := CloudOutline(tiny, 1, 1)
	if ok {
		t.Error("expected !ok for a 2x2 rectangle")
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

func TestSignedArea(t *testing.T) {
	// CCW square should have positive area
	area := signedArea(unitSquare)
	if area <= 0 {
		t.Errorf("expected positive area for CCW polygon, got %f", area)
	}

	// CW version
	cw := []vec.Vec2{
		{X: 0, Y: 0},
		{X: 0, Y: 100},
		{X: 100, Y: 100},
		{X: 100, Y: 0},
	}
	area = signedArea(cw)
	if area >= 0 {
		t.Errorf("expected negative area for CW polygon, got %f", area)
	}
}
