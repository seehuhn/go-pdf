// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
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
		LLx: pdf.Floor(r.LLx, 2),
		LLy: pdf.Floor(r.LLy, 2),
		URx: pdf.Ceil(r.URx, 2),
		URy: pdf.Ceil(r.URy, 2),
	}
}

// hairlineWidth is the width allowed for a hairline, the stroke a line width
// of 0 asks for.  A hairline is the thinnest line the device can show, one
// device pixel, which is no wider than a point at 72 dpi and above.
const hairlineWidth = 1.0

// hairlineAllowance returns how far a rectangle measured for a stroke of
// width lw has to grow on every side to hold it as drawn: half of
// [hairlineWidth] for a hairline, and nothing otherwise.
func hairlineAllowance(lw float64) float64 {
	if lw == 0 {
		return hairlineWidth / 2
	}
	return 0
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

// strokeBounds returns the rectangle covered by stroking the polylines in
// subpaths with the given stroke options.  It is the bounding box an
// appearance stream drawing them needs, and the annotation rectangle that
// appearance is placed in.  With opt.Dashed set, the bounds hold for any
// dash pattern and phase.
//
// The second result is false if the stroke draws nothing, in which case
// there is nothing to bound.
func strokeBounds(subpaths [][]vec.Vec2, closed bool, opt path.StrokeOptions) (pdf.Rectangle, bool) {
	bbox, ok := path.PolylineStrokeBBox(subpaths, closed, opt)
	return pdf.RectangleFromRect(bbox), ok
}
