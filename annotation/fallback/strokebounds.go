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
		LLx: math.Floor(r.LLx*100) / 100,
		LLy: math.Floor(r.LLy*100) / 100,
		URx: math.Ceil(r.URx*100) / 100,
		URy: math.Ceil(r.URy*100) / 100,
	}
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
