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

func hasCurveOps(ops content.Operators) bool {
	for _, op := range ops {
		switch op.Name {
		case content.OpCurveTo, content.OpCurveToV, content.OpCurveToY:
			return true
		}
	}
	return false
}

func hasOp(ops content.Operators, name content.OpName) bool {
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
	b := builder.New(content.Form, nil)
	drawCloudyBorder(b, tiny, 1, 1, true, true)

	if hasCurveOps(b.Stream) {
		t.Error("expected no CurveTo ops for small polygon fallback")
	}
	if !hasOp(b.Stream, content.OpMoveTo) {
		t.Error("expected MoveTo in plain polygon")
	}
}

func TestCloudyBorderRectangle(t *testing.T) {
	b := builder.New(content.Form, nil)
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

	b1 := builder.New(content.Form, nil)
	bbox1 := drawCloudyBorder(b1, unitSquare, 1, 1, true, false)

	b2 := builder.New(content.Form, nil)
	bbox2 := drawCloudyBorder(b2, cw, 1, 1, true, false)

	// bounding boxes should be similar (tolerance accounts for bulge placement shift)
	if math.Abs(bbox1.Dx()-bbox2.Dx()) > 5 || math.Abs(bbox1.Dy()-bbox2.Dy()) > 5 {
		t.Errorf("CW and CCW bboxes differ: %v vs %v", bbox1, bbox2)
	}
}

func TestCloudyBorderBBox(t *testing.T) {
	b := builder.New(content.Form, nil)
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
	b := builder.New(content.Form, nil)
	bbox := drawCloudyBorder(b, tri, 1, 1, true, true)

	if !hasCurveOps(b.Stream) {
		t.Error("expected CurveTo ops for triangle")
	}
	if bbox.IsZero() {
		t.Error("expected non-zero bbox")
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
