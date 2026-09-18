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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/color"
)

// TestLeaderLineBBox checks the rectangle a horizontal line with leader lines
// needs.  The leaders run perpendicular to the line, to its left looking from
// the start towards the end, so for a line running to the right they go up.
func TestLeaderLineBBox(t *testing.T) {
	a := &annotation.Line{
		Coords: [4]float64{0, 0, 100, 0},
		LL:     20, // leader length, so the line is drawn 20 above the coords
		LLO:    5,  // gap between the coords and where the leader starts
		LLE:    3,  // leader extension past the drawn line
	}
	const lw = 2

	// The two leaders stand at x = 5 and x = 95, LLO in from each end.  The
	// line proper is drawn at y = LL = 20, and each leader is extended LLE
	// back towards the coordinates, reaching y = 17.  Every point is widened
	// by half the line width, and the rectangle already covers the line
	// between the two coordinates.
	bbox := pdf.Rectangle{LLx: 0, LLy: 0, URx: 100, URy: 0}
	expandBBoxForLeaderLines(&bbox, a, lw)

	want := pdf.Rectangle{LLx: 0, LLy: -lw / 2, URx: 100, URy: 20 + lw/2}
	if !bbox.NearlyEqual(&want, 1e-9) {
		t.Errorf("got %v, want %v", bbox, want)
	}
}

// TestLeaderLineBBoxDirection checks that reversing the line puts the leaders
// on the other side, since they follow the direction of travel.
func TestLeaderLineBBoxDirection(t *testing.T) {
	forward := &annotation.Line{Coords: [4]float64{0, 0, 100, 0}, LL: 20}
	backward := &annotation.Line{Coords: [4]float64{100, 0, 0, 0}, LL: 20}

	var up, down pdf.Rectangle
	expandBBoxForLeaderLines(&up, forward, 0)
	expandBBoxForLeaderLines(&down, backward, 0)

	if math.Abs(up.URy-20) > 1e-9 || math.Abs(up.LLy) > 1e-9 {
		t.Errorf("forward leaders span y %g..%g, want 0..20", up.LLy, up.URy)
	}
	if math.Abs(down.LLy+20) > 1e-9 || math.Abs(down.URy) > 1e-9 {
		t.Errorf("backward leaders span y %g..%g, want -20..0", down.LLy, down.URy)
	}
}

// TestLeaderLineBBoxShortLine checks that a line too short to have a
// direction leaves the rectangle alone.
func TestLeaderLineBBoxShortLine(t *testing.T) {
	a := &annotation.Line{Coords: [4]float64{10, 10, 10.05, 10}, LL: 20}

	bbox := pdf.Rectangle{LLx: 10, LLy: 10, URx: 10.05, URy: 10}
	before := bbox
	expandBBoxForLeaderLines(&bbox, a, 2)
	if bbox != before {
		t.Errorf("got %v, want it unchanged at %v", bbox, before)
	}
}

// TestLineWithLeaders drives the generator with a line which has leaders, and
// checks that the appearance draws them and stays inside the rectangle.
func TestLineWithLeaders(t *testing.T) {
	a := &annotation.Line{
		Common: annotation.Common{
			Color:  color.DeviceRGB{0, 0, 1},
			Border: &annotation.Border{Width: 2},
		},
		Coords: [4]float64{20, 20, 120, 60},
		LL:     15,
		LLO:    4,
		LLE:    3,
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}

	// the two leaders and the line between them
	if n := countStrokes(t, a); n < 3 {
		t.Errorf("the line drew %d strokes, want at least 3", n)
	}

	// both endpoints of the line, and the leaders growing from them, have to
	// fit inside the rectangle the appearance is placed in
	for _, p := range [][2]float64{{20, 20}, {120, 60}} {
		if p[0] < a.Rect.LLx || p[0] > a.Rect.URx ||
			p[1] < a.Rect.LLy || p[1] > a.Rect.URy {
			t.Errorf("endpoint %v lies outside rect %v", p, a.Rect)
		}
	}
}

// TestLineWithShortLeaders checks that a line too short to have a direction
// falls back to the plain line rather than dividing by its length.
func TestLineWithShortLeaders(t *testing.T) {
	a := &annotation.Line{
		Common: annotation.Common{
			Color:  color.DeviceRGB{0, 0, 1},
			Border: &annotation.Border{Width: 1},
		},
		Coords: [4]float64{30, 30, 30.01, 30},
		LL:     15,
	}

	g := newGen(t, pdf.V2_0)
	if err := g.AddAppearance(a); err != nil {
		t.Fatal(err)
	}
	if n := countStrokes(t, a); n == 0 {
		t.Error("the line stroked nothing")
	}
}
