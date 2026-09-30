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
	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// normalizeLE replaces an empty line ending style with None.
func normalizeLE(le annotation.LineEndingStyle) annotation.LineEndingStyle {
	if le == "" {
		return annotation.LineEndingStyleNone
	}
	return le
}

// drawOpenPolyline draws an open path through points with optional line
// endings at the start and end.  The points must already be rounded; see
// [roundPoints].  The line width, stroke color, and dash pattern must already
// be set on the builder.
func drawOpenPolyline(b *builder.Builder, points []vec.Vec2, startLE, endLE annotation.LineEndingStyle, fillColor color.Color) {
	if len(points) < 2 {
		return
	}
	n := len(points)

	lw := b.State.GState.LineWidth
	e0 := newLineEnding(startLE, lineEndingInfo{At: points[0], Dir: points[0].Sub(points[1]), FillColor: fillColor})
	e1 := newLineEnding(endLE, lineEndingInfo{At: points[n-1], Dir: points[n-1].Sub(points[n-2]), FillColor: fillColor})
	c := roundPoints([]vec.Vec2{e0.connection(lw), e1.connection(lw)})

	// The endings join the path at points set back from its ends.  Where an
	// end segment is shorter than the setback, the path starts or stops at
	// the inner point of that segment instead, so that it does not run
	// backwards through the ending.
	var path []vec.Vec2
	if n == 2 {
		if c[0].Sub(c[1]).Dot(points[0].Sub(points[1])) >= 0 {
			path = c
		}
	} else {
		if c[0].Sub(points[1]).Dot(points[0].Sub(points[1])) >= 0 {
			path = append(path, c[0])
		}
		path = append(path, points[1:n-1]...)
		if c[1].Sub(points[n-2]).Dot(points[n-1].Sub(points[n-2])) >= 0 {
			path = append(path, c[1])
		}
	}

	e0.drawShape(b)
	if len(path) >= 2 {
		b.MoveTo(path[0].X, path[0].Y)
		for _, p := range path[1:] {
			b.LineTo(p.X, p.Y)
		}
		b.Stroke()
	}
	e1.drawShape(b)
}

// openPolylineBBox computes a bounding box for an open polyline stroked
// with the given options and optional line endings.  With miter joins, a
// sharp corner reaches beyond the line width.  The points must be the
// rounded ones the polyline is drawn through; see [roundPoints].  The
// result is the zero rectangle if nothing is drawn.
func openPolylineBBox(points []vec.Vec2, opt path.StrokeOptions,
	startLE, endLE annotation.LineEndingStyle) pdf.Rectangle {
	lw := opt.Width

	// zero where the stroke draws nothing, which the line endings take as
	// absent
	bbox, _ := strokeBounds([][]vec.Vec2{points}, false, opt)

	n := len(points)

	// expand for start line ending
	if n >= 2 && startLE != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:  points[0],
			Dir: points[0].Sub(points[1]),
		}
		lineEndingBBox(&bbox, startLE, info, lw)
	}

	// expand for end line ending
	if n >= 2 && endLE != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:  points[n-1],
			Dir: points[n-1].Sub(points[n-2]),
		}
		lineEndingBBox(&bbox, endLE, info, lw)
	}
	if bbox.IsZero() {
		return bbox // nothing is drawn
	}

	// the line endings are drawn with two decimals, which can carry them a
	// little outside the exact geometry measured here
	return roundOut(bbox.Grow(pathPrecision + hairlineAllowance(lw)))
}
