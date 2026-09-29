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

	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// pathPrecision is how far a path written to the file can lie outside the
// coordinates it was computed from: the operands carry two decimals, and a
// point of a curve is a weighted average of them, so it moves by no more
// than half of the last digit.
const pathPrecision = 0.005

// roundOut returns the smallest rectangle with two-decimal edges which
// contains r.
//
// An annotation whose /RD gives an inner rectangle as insets from its own
// needs its rectangle rounded this way: rounding to nearest can move an edge
// inside the rectangle it is derived from, and an inset may not be negative.
func roundOut(r pdf.Rectangle) pdf.Rectangle {
	return pdf.Rectangle{
		LLx: roundDown(r.LLx),
		LLy: roundDown(r.LLy),
		URx: -roundDown(-r.URx),
		URy: -roundDown(-r.URy),
	}
}

// roundPoints returns a copy of pts with every point rounded to two
// decimals, the precision a path is written to the file with.
//
// A generator draws a polyline through the rounded points and measures its
// stroke from the same points, so that the bounds hold the path exactly as
// it is drawn.  A margin added to bounds taken from the exact points is not
// enough where the path has miter joins: moving a vertex by a fraction of a
// hundredth can turn a bevelled corner into a long miter.
func roundPoints(pts []vec.Vec2) []vec.Vec2 {
	if pts == nil {
		return nil
	}
	out := make([]vec.Vec2, len(pts))
	for i, p := range pts {
		out[i] = vec.Vec2{X: pdf.Round(p.X, 2), Y: pdf.Round(p.Y, 2)}
	}
	return out
}

// roundPaths applies [roundPoints] to every sub-path.
func roundPaths(subpaths [][]vec.Vec2) [][]vec.Vec2 {
	out := make([][]vec.Vec2, len(subpaths))
	for i, pts := range subpaths {
		out[i] = roundPoints(pts)
	}
	return out
}

// roundDown returns the largest two-decimal value which is not greater
// than x.
//
// Scaling x by 100 is not exact, so taking the floor of x*100 can move a
// value which already has two decimals, such as 0.29, down by 0.01.  The
// value is therefore rounded to nearest first, which leaves such a value
// alone, and stepped down only when that lands above x.
func roundDown(x float64) float64 {
	n := math.Round(x * 100)
	if n/100 > x {
		n--
	}
	return n / 100
}

// strokeBounds returns the rectangle covered by stroking the polylines in
// subpaths with a line of width lw.  It is the bounding box an appearance
// stream drawing them needs, and the annotation rectangle that appearance is
// placed in.
//
// The generators which use this stroke with a butt or a round cap, neither of
// which reaches further than half the line width, so the bounds are taken
// with a butt cap.  A projecting square cap reaches further, and needs
// [path.PolylineStrokeBBox] called directly.
//
// The second result is false if no sub-path has a vertex, in which case there
// is nothing to bound.
func strokeBounds(subpaths [][]vec.Vec2, closed bool, lw float64, join graphics.LineJoinStyle, miterLimit float64) (pdf.Rectangle, bool) {
	bbox, ok := path.PolylineStrokeBBox(subpaths, closed, path.StrokeOptions{
		Width:      lw,
		Cap:        path.CapButt,
		Join:       join,
		MiterLimit: miterLimit,
	})
	return pdf.RectangleFromRect(bbox), ok
}
