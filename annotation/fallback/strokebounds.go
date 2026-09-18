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

	"seehuhn.de/go/geom/linalg"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

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
// Each vertex is surrounded by half the line width, which covers a butt, round
// or projecting square cap as well as a round or bevel join.  A miter join
// reaches further, and how much further depends on how sharp the corner is, so
// the interior vertices of a miter-joined path are surrounded by the miter
// length instead, unless the corner is too sharp for miterLimit and the join is
// bevelled.  A closed path has a join at every vertex, an open one only at the
// vertices between its first and its last.
//
// The second result is false if no sub-path has a vertex, in which case there
// is nothing to bound.
func strokeBounds(subpaths [][]vec.Vec2, closed bool, lw float64, join graphics.LineJoinStyle, miterLimit float64) (pdf.Rectangle, bool) {
	var bbox pdf.Rectangle
	seeded := false

	extend := func(p vec.Vec2, r float64) {
		if !seeded {
			bbox = pdf.Rectangle{LLx: p.X - r, LLy: p.Y - r, URx: p.X + r, URy: p.Y + r}
			seeded = true
			return
		}
		bbox.LLx = min(bbox.LLx, p.X-r)
		bbox.LLy = min(bbox.LLy, p.Y-r)
		bbox.URx = max(bbox.URx, p.X+r)
		bbox.URy = max(bbox.URy, p.Y+r)
	}

	for _, pts := range subpaths {
		n := len(pts)
		for i, p := range pts {
			r := lw / 2
			if join == graphics.LineJoinMiter && n >= 3 {
				var prev, next vec.Vec2
				switch {
				case i > 0 && i < n-1:
					prev, next = pts[i-1], pts[i+1]
				case closed:
					prev, next = pts[(i+n-1)%n], pts[(i+1)%n]
				default:
					prev, next = p, p // an end point of an open path: no join
				}
				// A vertex which carries no join gives two zero directions,
				// for which the miter reaches no further than the bevel.
				length, _ := linalg.MiterLength(p.Sub(prev), next.Sub(p), lw, miterLimit)
				r = max(r, length)
			}
			extend(p, r)
		}
	}
	return bbox, seeded
}
