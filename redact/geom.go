// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"cmp"
	"iter"
	"math"
	"slices"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/redactmark"
)

const (
	// touchTol is the distance in page units below which shapes which
	// meet are considered to only touch.  It is the precision which
	// pdf.Round(…,2) gives.
	touchTol = 0.01

	// zeroWidthReach is the reach in page units of a line of width zero,
	// which the viewer draws as thinnest possible line.
	zeroWidthReach = 0.5

	// insideEps absorbs floating-point error in inside tests.
	insideEps = 1e-6

	// cellEps is the amount in image units by which a polygon is grown
	// before it is tested against image cells.
	cellEps = 1e-9
)

// poly is a convex polygon with vertices in counterclockwise order.  A
// polygon with two vertices is a segment, and one with a single vertex is a
// point.
type poly []vec.Vec2

// convexHull returns the convex hull of pts, in counterclockwise order and
// without collinear or duplicate vertices.  The hull of collinear points is
// a segment, and the hull of coincident points is a point.
func convexHull(pts []vec.Vec2) poly {
	p := slices.Clone(pts)
	slices.SortFunc(p, func(a, b vec.Vec2) int {
		return cmp.Or(cmp.Compare(a.X, b.X), cmp.Compare(a.Y, b.Y))
	})
	p = slices.Compact(p)
	if len(p) < 3 {
		return poly(p)
	}

	// Andrew's monotone chain
	hull := make(poly, 0, 2*len(p))
	for _, v := range p {
		for len(hull) >= 2 && cross3(hull[len(hull)-2], hull[len(hull)-1], v) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, v)
	}
	lower := len(hull) + 1
	for i := len(p) - 2; i >= 0; i-- {
		v := p[i]
		for len(hull) >= lower && cross3(hull[len(hull)-2], hull[len(hull)-1], v) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, v)
	}
	hull = hull[:len(hull)-1]
	if len(hull) == 2 && hull[0] == hull[1] {
		hull = hull[:1]
	}
	return hull
}

// cross3 returns the z-component of (b-a) x (c-a).
func cross3(a, b, c vec.Vec2) float64 {
	return b.Sub(a).Cross(c.Sub(a))
}

// regionHulls returns the convex hull of each quadrilateral.  The order of
// the corners within a quadrilateral does not matter.
func regionHulls(quads []redactmark.Quad) []poly {
	res := make([]poly, len(quads))
	for i, q := range quads {
		res[i] = convexHull(q[:])
	}
	return res
}

// transformPoints returns the images of pts under m.
func transformPoints(pts []vec.Vec2, m matrix.Matrix) []vec.Vec2 {
	res := make([]vec.Vec2, len(pts))
	for i, v := range pts {
		res[i] = m.Apply(v)
	}
	return res
}

// transform returns the image of h under m.  If m is singular, the result
// is a segment or a point.
func (h poly) transform(m matrix.Matrix) poly {
	return convexHull(transformPoints(h, m))
}

// clipConvex returns the part of the convex polygon with vertices pts which
// lies in the convex polygon c.  The vertices pts must be given in order,
// and c must have at least three vertices.  The result lists the vertices of
// the part in order, possibly with repeated or collinear vertices.  It is
// empty if the polygons do not meet, or if the computation overflows, so an
// empty result does not show that the polygons are disjoint.
func clipConvex(pts []vec.Vec2, c poly) []vec.Vec2 {
	n := len(c)
	for i := range n {
		a := c[i]
		e := c[(i+1)%n].Sub(a)
		pts = clipHalf(pts, func(p vec.Vec2) float64 { return e.Cross(p.Sub(a)) }, nil)
	}
	return pts
}

// clipBox returns the part of the convex polygon with vertices pts which
// lies in the square [lo,hi]², like [clipConvex] does.  The vertices must
// be finite.
func clipBox(pts []vec.Vec2, lo, hi float64) []vec.Vec2 {
	pts = clipHalf(pts, func(p vec.Vec2) float64 { return p.X - lo }, func(p *vec.Vec2) { p.X = lo })
	pts = clipHalf(pts, func(p vec.Vec2) float64 { return hi - p.X }, func(p *vec.Vec2) { p.X = hi })
	pts = clipHalf(pts, func(p vec.Vec2) float64 { return p.Y - lo }, func(p *vec.Vec2) { p.Y = lo })
	return clipHalf(pts, func(p vec.Vec2) float64 { return hi - p.Y }, func(p *vec.Vec2) { p.Y = hi })
}

// clipHalf returns the part of the convex polygon with vertices pts, given
// in order, where dist is non-negative.  The function dist must be affine.
// If snap is not nil, it moves a point which is computed on the line where
// dist is zero exactly onto the line.  The result is empty if dist is not
// finite at some vertex.
func clipHalf(pts []vec.Vec2, dist func(vec.Vec2) float64, snap func(*vec.Vec2)) []vec.Vec2 {
	var res []vec.Vec2
	for j, p := range pts {
		q := pts[(j+1)%len(pts)]
		dp, dq := dist(p), dist(q)
		if !isFinite(dp) || !isFinite(dq) {
			return nil
		}
		if dp >= 0 {
			res = append(res, p)
		}
		if (dp > 0 && dq < 0) || (dp < 0 && dq > 0) {
			// halving avoids overflow, and the convex combination
			// cannot overflow
			t := (dp / 2) / (dp/2 - dq/2)
			x := p.Mul(1 - t).Add(q.Mul(t))
			if snap != nil {
				snap(&x)
			}
			res = append(res, x)
		}
	}
	return res
}

// scaledHull returns the convex hull of pts, like convexHull does, without
// overflow for large coordinates.  The coordinates must be finite.
func scaledHull(pts []vec.Vec2) poly {
	size := 0.0
	for _, v := range pts {
		size = max(size, math.Abs(v.X), math.Abs(v.Y))
	}
	if size == 0 {
		return convexHull(pts)
	}
	// scaling by a power of two is exact
	_, k := math.Frexp(size)
	scaled := make([]vec.Vec2, len(pts))
	for i, v := range pts {
		scaled[i] = vec.Vec2{X: math.Ldexp(v.X, -k), Y: math.Ldexp(v.Y, -k)}
	}
	h := convexHull(scaled)
	for i, v := range h {
		h[i] = vec.Vec2{X: math.Ldexp(v.X, k), Y: math.Ldexp(v.Y, k)}
	}
	return h
}

// boxCorners returns the corners of the boxes [-rx,rx] × [-ry,ry] around the
// points pts.
func boxCorners(pts []vec.Vec2, rx, ry float64) []vec.Vec2 {
	res := make([]vec.Vec2, 0, 4*len(pts))
	for _, v := range pts {
		res = append(res,
			vec.Vec2{X: v.X - rx, Y: v.Y - ry}, vec.Vec2{X: v.X + rx, Y: v.Y - ry},
			vec.Vec2{X: v.X + rx, Y: v.Y + ry}, vec.Vec2{X: v.X - rx, Y: v.Y + ry})
	}
	return res
}

// isFinite reports whether all coordinates of the vertices of h are finite.
func (h poly) isFinite() bool {
	for _, v := range h {
		if !isFinite(v.X) || !isFinite(v.Y) {
			return false
		}
	}
	return true
}

// isFinite reports whether x is neither infinite nor NaN.
func isFinite(x float64) bool {
	return !math.IsInf(x, 0) && !math.IsNaN(x)
}

// grow returns a convex polygon which contains all points within distance r
// of h.  The result can be slightly larger than the set of these points.
func (h poly) grow(r float64) poly {
	if r <= 0 || len(h) == 0 {
		return h
	}
	// the octagon circumscribes the disc
	const n = 8
	R := r / math.Cos(math.Pi/n)
	pts := make([]vec.Vec2, 0, n*len(h))
	for _, v := range h {
		for k := range n {
			a := (float64(k) + 0.5) * 2 * math.Pi / n
			pts = append(pts, v.Add(vec.Vec2{X: R * math.Cos(a), Y: R * math.Sin(a)}))
		}
	}
	return convexHull(pts)
}

// intersects reports whether the interiors of the convex polygons a and b
// overlap by more than tol.  Shapes which only touch along an edge or in a
// point, within a distance of tol, do not intersect.
func intersects(a, b poly, tol float64) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	if len(a) == 1 && len(b) == 1 {
		return a[0].Sub(b[0]).Length() < tol
	}
	return !separated(a, b, tol) && !separated(b, a, tol)
}

// separated reports whether one of the axes derived from the edges of a
// separates a from b by at least -tol.
func separated(a, b poly, tol float64) bool {
	n := len(a)
	if n < 2 {
		return false
	}
	for i := range n {
		if n == 2 && i == 1 {
			break
		}
		e := a[(i+1)%n].Sub(a[i])
		if e.Length() == 0 {
			continue
		}
		// A degenerate polygon has no width along its normal, so its
		// normal separates only by more than tol; the direction of a
		// segment is needed for collinear segments.
		if sep(a, b, e.Rot90().Normalize(), tolN(n, tol)) {
			return true
		}
		if n < 3 && sep(a, b, e.Normalize(), tol) {
			return true
		}
	}
	return false
}

// tolN returns the separation needed along the normal of an edge of a polygon
// with n vertices.
func tolN(n int, tol float64) float64 {
	if n < 3 {
		return -tol
	}
	return tol
}

// sep reports whether the gap between a and b along the unit vector ax is at
// least -tol.
func sep(a, b poly, ax vec.Vec2, tol float64) bool {
	minA, maxA := project(a, ax)
	minB, maxB := project(b, ax)
	return max(minA-maxB, minB-maxA) >= -tol
}

// project returns the range of the projections of the vertices of h on the
// unit vector ax.
func project(h poly, ax vec.Vec2) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, v := range h {
		d := v.Dot(ax)
		lo = min(lo, d)
		hi = max(hi, d)
	}
	return lo, hi
}

// inside reports whether every point of pts lies at least reach inside the
// polygon h.  Since the convex hull of the control points of a curve
// contains the curve, the test is conservative when applied to control
// points.  A polygon without interior contains nothing.
func inside(pts []vec.Vec2, h poly, reach float64) bool {
	n := len(h)
	if n < 3 {
		return false
	}
	for i := range n {
		a, b := h[i], h[(i+1)%n]
		e := b.Sub(a)
		l := e.Length()
		for _, p := range pts {
			if e.Cross(p.Sub(a))/l < reach-insideEps {
				return false
			}
		}
	}
	return true
}

// sigmaMax returns the largest factor by which m scales lengths.
func sigmaMax(m matrix.Matrix) float64 {
	_, s := m.SingularValues()
	return s
}

// strokeReach returns the largest distance in page units by which a stroke
// of line width lw can extend beyond the centre line of its path, when drawn
// under the matrix m (which maps user space to page space).  A line of width
// zero has a reach of zeroWidthReach.  Dashes only remove ink and are not
// considered.
func strokeReach(lw, miterLimit float64, join graphics.LineJoinStyle, lineCap graphics.LineCapStyle, m matrix.Matrix) float64 {
	if lw == 0 {
		return zeroWidthReach
	}
	f := 1.0
	if join == graphics.LineJoinMiter {
		f = max(f, miterLimit)
	}
	if lineCap == graphics.LineCapSquare {
		f = max(f, math.Sqrt2)
	}
	return lw / 2 * f * sigmaMax(m)
}

// cellRanges finds the cells of a w × rows image which the polygon h meets
// by more than a margin.  The polygon is given in image space, where the
// image occupies the unit square [0,1]².  The first sample row of an image
// lies at the top of the image, so row r covers y from 1-(r+1)/rows to
// 1-r/rows, and column c covers x from c/w to (c+1)/w.
//
// A cell counts if the polygon meets the cell shrunk by ex at its left and
// right and by ey at its top and bottom.  The margins must be non-negative
// and less than half the width and height of a cell.  Rounding errors of up
// to cellEps count in favour of the cell.
//
// The result yields, in order of increasing r, each row r which has any cell
// which counts, together with the half-open range [c0, c1) of the columns of
// these cells.  The work is proportional to the number of rows times the
// number of polygon vertices.
func cellRanges(h poly, w, rows int, ex, ey float64) iter.Seq2[int, [2]int] {
	return func(yield func(int, [2]int) bool) {
		if len(h) == 0 || w <= 0 || rows <= 0 {
			return
		}
		yMin, yMax := math.Inf(1), math.Inf(-1)
		for _, v := range h {
			yMin = min(yMin, v.Y)
			yMax = max(yMax, v.Y)
		}
		yMin, yMax = widenNaN(yMin, yMax)
		fr, fw := float64(rows), float64(w)
		rLo := clampIndex(math.Ceil((1-yMax+ey-cellEps)*fr)-1, 0, rows)
		rHi := clampIndex(math.Floor((1-yMin-ey+cellEps)*fr), -1, rows-1)
		for r := rLo; r <= rHi; r++ {
			y0 := 1 - float64(r+1)/fr
			y1 := 1 - float64(r)/fr
			xLo, xHi, ok := bandExtent(h, y0+ey-cellEps, y1-ey+cellEps)
			if !ok {
				continue
			}
			xLo, xHi = widenNaN(xLo, xHi)
			c0 := clampIndex(math.Ceil((xLo+ex-cellEps)*fw)-1, 0, w)
			c1 := clampIndex(math.Floor((xHi-ex+cellEps)*fw)+1, 0, w)
			if c0 >= c1 {
				continue
			}
			if !yield(r, [2]int{c0, c1}) {
				return
			}
		}
	}
}

// widenNaN replaces a NaN bound of the range [lo, hi], which arithmetic on
// huge coordinates can produce, by the infinite bound in the same direction.
func widenNaN(lo, hi float64) (float64, float64) {
	if math.IsNaN(lo) {
		lo = math.Inf(-1)
	}
	if math.IsNaN(hi) {
		hi = math.Inf(1)
	}
	return lo, hi
}

// clampIndex returns v, which must not be NaN, truncated to an integer and
// limited to the range [lo, hi].
func clampIndex(v float64, lo, hi int) int {
	switch {
	case v <= float64(lo):
		return lo
	case v >= float64(hi):
		return hi
	}
	return int(v)
}

// bandExtent returns the range of x-coordinates of the part of the convex
// polygon h which lies in the horizontal band y0 <= y <= y1.  The result ok
// is false if the polygon does not meet the band.
func bandExtent(h poly, y0, y1 float64) (xLo, xHi float64, ok bool) {
	xLo, xHi = math.Inf(1), math.Inf(-1)
	add := func(x float64) {
		xLo = min(xLo, x)
		xHi = max(xHi, x)
		ok = true
	}
	n := len(h)
	for i, p := range h {
		if p.Y >= y0 && p.Y <= y1 {
			add(p.X)
		}
		if n < 2 || (n == 2 && i == 1) {
			continue
		}
		q := h[(i+1)%n]
		for _, y := range [2]float64{y0, y1} {
			if (p.Y < y) != (q.Y < y) {
				// the convex combination cannot overflow
				t := (y - p.Y) / (q.Y - p.Y)
				add((1-t)*p.X + t*q.X)
			}
		}
	}
	return xLo, xHi, ok
}
