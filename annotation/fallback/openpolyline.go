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
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
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
// endings at the start and end.  The line width, stroke color, and dash
// pattern must already be set on the builder.
func drawOpenPolyline(b *builder.Builder, points []vec.Vec2, startLE, endLE annotation.LineEndingStyle, fillColor color.Color) {
	if len(points) < 2 {
		return
	}

	n := len(points)

	// start point
	if startLE != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:        points[0],
			Dir:       points[0].Sub(points[1]),
			FillColor: fillColor,
			IsStart:   true,
		}
		drawLineEndingBuilder(b, startLE, info)
	} else {
		b.MoveTo(pdf.Round(points[0].X, 2), pdf.Round(points[0].Y, 2))
	}

	// intermediate points
	for i := 1; i < n-1; i++ {
		b.LineTo(pdf.Round(points[i].X, 2), pdf.Round(points[i].Y, 2))
	}

	// end point
	if endLE != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:        points[n-1],
			Dir:       points[n-1].Sub(points[n-2]),
			FillColor: fillColor,
			IsStart:   false,
		}
		drawLineEndingBuilder(b, endLE, info)
	} else {
		b.LineTo(pdf.Round(points[n-1].X, 2), pdf.Round(points[n-1].Y, 2))
		b.Stroke()
	}
}

// openPolylineBBox computes a bounding box for an open polyline with the
// given line width and optional line endings.  The path is stroked with the
// default miter joins, so a sharp corner reaches beyond the line width.
func openPolylineBBox(points []vec.Vec2, lw float64, startLE, endLE annotation.LineEndingStyle) pdf.Rectangle {
	bbox, ok := strokeBounds([][]vec.Vec2{points}, false, lw,
		graphics.LineJoinMiter, defaultMiterLimit)
	if !ok {
		return pdf.Rectangle{}
	}

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

	bbox.IRound(2)
	return bbox
}
