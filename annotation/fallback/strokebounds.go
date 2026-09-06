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

	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// defaultMiterLimit is the miter limit of a graphics state which does not set
// one (Table 51).
const defaultMiterLimit = 10

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
				if prev != p && next != p {
					r = max(r, miterRadius(prev, p, next, lw, miterLimit))
				}
			}
			extend(p, r)
		}
	}
	return bbox, seeded
}

// miterRadius returns how far the miter join at b, between the segments a-b
// and b-c, reaches from b.  A corner whose miter is longer than miterLimit
// times the line width is bevelled instead (§8.4.3.5), and then reaches only
// half the line width, as does a corner whose segments are degenerate.
func miterRadius(a, b, c vec.Vec2, lw, miterLimit float64) float64 {
	in := b.Sub(a)
	out := c.Sub(b)
	lin, lout := in.Length(), out.Length()
	if lin == 0 || lout == 0 {
		return lw / 2
	}

	// half the angle the two segments enclose at b
	cosTheta := min(max(-in.Dot(out)/(lin*lout), -1), 1)
	sinHalf := math.Sqrt((1 - cosTheta) / 2)
	if sinHalf <= 0 || 1/sinHalf > miterLimit {
		return lw / 2
	}
	return lw / 2 / sinHalf
}
