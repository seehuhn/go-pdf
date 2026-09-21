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
	"seehuhn.de/go/geom/path"
	"seehuhn.de/go/geom/polygon"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// maxSamplePoints bounds the number of points the polygon boundary is sampled
// at.  The spacing follows the line width, so without a bound the sample count
// grows with the size of the polygon, and a file can ask for an unbounded
// amount of memory, and a content stream to match, from a handful of vertices
// with large coordinates.  The bound is far above what a cloud drawn round the
// largest page PDF allows needs at the finest spacing, so it takes effect only
// for a polygon which could not be shown anyway; from there on the bulges grow
// larger instead of more numerous.
const maxSamplePoints = 20000

// minBulges is the fewest bulges a cloud is drawn with: below three the
// outline reads as a rounded polygon rather than as a cloud.  An outline
// with no room for three of the size asked for gets three smaller ones.
const minBulges = 3

// cloudGiveUp is the share of a shape's size, as [cloudSize] measures it,
// that a border may take before a cloud is given up on.  Past it the stroke
// leaves too little inside to read as a shape with a border round it,
// whatever is drawn along that border.
const cloudGiveUp = 0.54

// cloudTickLimit is the share of a shape's size that a border may take
// before the cusp ticks are dropped.  Past it the pen is wide enough to blot
// a tick into the two bulges it sits between, so that it reads as a blob at
// the join rather than as the mark of a pen lifted and set down again.
const cloudTickLimit = 0.33

// cloudSize measures the room a shape has for a border drawn round it, as
// the geometric mean of two widths: 4*area/perimeter, which a long thin
// shape takes mostly from its shorter side, and sqrt(area), which it takes
// from its overall extent.  Neither alone predicts the width at which detail
// along the border stops reading -- the first grows too slowly as a shape
// stretches, the second too fast -- while their mean holds from square to
// sixteen-to-one, and across rectangles, ellipses and triangles alike.
func cloudSize(vertices []vec.Vec2, perimeter float64) float64 {
	area := math.Abs(polygon.SignedArea(vertices))
	return math.Sqrt((4 * area / perimeter) * math.Sqrt(area))
}

// pathExtent returns the rectangle the points of a path lie in.
//
// It is not [path.Path.BBox], which includes the control points of a curve:
// a cloud's bulges reach only about three quarters as far as the control
// points which shape them, which is too coarse to fit a cloud into a
// rectangle with.
func pathExtent(p path.Path) pdf.Rectangle {
	ext := pdf.Rectangle{
		LLx: math.Inf(1), LLy: math.Inf(1),
		URx: math.Inf(-1), URy: math.Inf(-1),
	}
	add := func(pt vec.Vec2) {
		ext.LLx = min(ext.LLx, pt.X)
		ext.LLy = min(ext.LLy, pt.Y)
		ext.URx = max(ext.URx, pt.X)
		ext.URy = max(ext.URy, pt.Y)
	}

	var cur vec.Vec2
	for cmd, pts := range p {
		switch cmd {
		case path.CmdCubeTo:
			for _, t := range cubicExtrema(cur, pts[0], pts[1], pts[2]) {
				add(path.EvalCubic(cur, pts[0], pts[1], pts[2], t))
			}
			add(pts[2])
		default:
			// a command whose shape this does not know contributes its
			// points as they stand, control points included
			for _, pt := range pts {
				add(pt)
			}
		}
		if n := len(pts); n > 0 {
			cur = pts[n-1]
		}
	}
	return ext
}

// cubicExtrema returns the parameters in (0, 1) at which a cubic Bezier
// curve turns round in x or in y, which is where it reaches furthest.
func cubicExtrema(p0, c1, c2, p3 vec.Vec2) []float64 {
	var ts []float64
	for _, v := range [][4]float64{
		{p0.X, c1.X, c2.X, p3.X},
		{p0.Y, c1.Y, c2.Y, p3.Y},
	} {
		// the derivative of the curve, divided by three
		a := -v[0] + 3*v[1] - 3*v[2] + v[3]
		b := 2 * (v[0] - 2*v[1] + v[2])
		c := -v[0] + v[1]

		if math.Abs(a) < 1e-12 {
			if b != 0 {
				ts = appendIfInside(ts, -c/b)
			}
			continue
		}
		disc := b*b - 4*a*c
		if disc < 0 {
			continue
		}
		root := math.Sqrt(disc)
		ts = appendIfInside(ts, (-b+root)/(2*a))
		ts = appendIfInside(ts, (-b-root)/(2*a))
	}
	return ts
}

func appendIfInside(ts []float64, t float64) []float64 {
	if t > 0 && t < 1 {
		return append(ts, t)
	}
	return ts
}

// cloudOutline holds precomputed cloud border geometry.
type cloudOutline struct {
	points  []vec.Vec2 // equidistant boundary points (CCW)
	cusps   []int      // point indices for bulge start/end
	hasBase bool       // whether a flat base was detected
	noTicks bool       // whether the pen is too wide for the cusp ticks
}

// newCloudOutline computes the cloud outline for a polygon.  It returns nil
// where no cloud is drawn: a degenerate polygon, or one whose border is so
// wide that nothing is left inside it (see [cloudGiveUp]).
func newCloudOutline(vertices []vec.Vec2, intensity, lw float64) *cloudOutline {
	n := len(vertices)
	if n < 3 {
		return nil
	}

	perimeter := polygon.Perimeter(vertices)
	if !(perimeter > 0) {
		return nil
	}

	// ensure CCW winding
	if polygon.SignedArea(vertices) < 0 {
		rev := make([]vec.Vec2, n)
		for i, v := range vertices {
			rev[n-1-i] = v
		}
		vertices = rev
	}

	// step 1: sample the outline.  The spacing follows the line width, but
	// only weakly: a pen twice as thick asks for a bulge about a quarter
	// larger, not twice as large, so that the bulge count keeps falling as
	// the pen grows rather than freezing once the pen passes some size.
	requestedDist := 8 * math.Cbrt(max(lw, 0.1))
	nSamples := math.Round(perimeter / requestedDist)
	if !(nSamples >= 3) { // false as well for a line width which is not a number
		nSamples = 3
	}
	// the outline is sampled at least this finely whatever the pen asks
	// for, so that there are always enough points to describe the fewest
	// bulges a cloud is drawn with.  Sampling and bulge size are separate:
	// sampling more finely than the pen asked for must not invent bulges.
	nSamples = max(nSamples, 4*minBulges*2)
	points := polygon.Resample(vertices, int(min(nSamples, maxSamplePoints)))

	// step 2: detect flat base
	baseStart, baseSeg := findFlatBase(points)

	ppb := max(2, int(math.Round(3*intensity)))
	nPoints := len(points)

	hasBase := baseSeg > 0
	cloudLen := nPoints
	if hasBase {
		cloudLen = nPoints - baseSeg
	}

	// step 3: the bulge count.
	//
	// The intensity and the line width together ask for a bulge of a
	// certain length of outline: a thicker pen wants a larger bulge, or the
	// cloud turns to mush.  The count is how many such bulges the cloud's
	// share of the perimeter has room for, taken from that arc length
	// rather than from the sample count, which may be finer.
	//
	// An outline too short for even minBulges of them gets that many
	// smaller ones instead: a cloud is given up on for having no room to
	// enclose anything, below, and not for wanting bulges it cannot fit.
	cloudPerimeter := perimeter * float64(cloudLen) / float64(nPoints)
	nBulges := int(math.Round(cloudPerimeter / (float64(ppb) * requestedDist)))
	nBulges = max(minBulges, nBulges)
	// no more bulges than the sampled outline can describe, which also
	// bounds the count for a polygon whose coordinates are far larger than
	// any page: the count follows the real perimeter, which nothing caps
	nBulges = min(nBulges, cloudLen/2)
	if nBulges < minBulges {
		return nil
	}

	// A border wider than this leaves too little inside the shape to read
	// as a shape with a border round it, whatever is drawn along it, so the
	// cloud gives way to the plain polygon.  This is the only thing a cloud
	// is given up for, and it depends on the shape and the pen alone -- not
	// on the intensity, which changes how a cloud looks rather than whether
	// one can be drawn at all.
	//
	// It is the second of two steps, the first being the cusp ticks: as the
	// pen grows the cloud loses its ticks, then its bulges, rather than
	// going from a cloud to a bare outline in one jump.
	size := cloudSize(vertices, perimeter)
	if lw > cloudGiveUp*size {
		return nil
	}

	// step 4: assign bulge boundaries
	var cusps []int
	if hasBase {
		cloudStart := (baseStart + baseSeg) % nPoints
		cusps = make([]int, nBulges+1)
		for i := range nBulges + 1 {
			offset := i * cloudLen / nBulges
			cusps[i] = (cloudStart + offset) % nPoints
		}
	} else {
		cusps = make([]int, nBulges)
		for i := range nBulges {
			cusps[i] = i * nPoints / nBulges
		}
	}

	return &cloudOutline{
		points:  points,
		cusps:   cusps,
		hasBase: hasBase,
		noTicks: lw > cloudTickLimit*size,
	}
}

// numBulges returns the number of cloud bulges.
func (co *cloudOutline) numBulges() int {
	if co.hasBase {
		return len(co.cusps) - 1
	}
	return len(co.cusps)
}

// bulgeEnd returns the cusp array index for the end of bulge i.
func (co *cloudOutline) bulgeEnd(i int) int {
	if co.hasBase {
		return i + 1
	}
	return (i + 1) % len(co.cusps)
}

// CloudOutline returns the paths the generator draws a cloudy border with
// round the polygon vertices: the closed fill path and the open stroke path.
// lineWidth is the border's stroke width, which sets the bulge size together
// with intensity (0 to 2). The stroke path carries a small tick across each
// cusp, except where the pen is wide enough to blot the ticks into the
// bulges, when they are left out. ok is false where the border is too wide
// for the polygon to enclose anything, in which case a caller draws the
// plain polygon instead.
func CloudOutline(vertices []vec.Vec2, intensity, lineWidth float64) (fill, stroke path.Path, ok bool) {
	co := newCloudOutline(vertices, intensity, lineWidth)
	if co == nil {
		return path.Empty, path.Empty, false
	}
	return co.fill(), co.stroke(), true
}

// fill returns the closed fill path, at full precision.
func (co *cloudOutline) fill() path.Path {
	var d path.Data
	nBulges := co.numBulges()

	if co.hasBase {
		// base line
		baseStart := co.points[co.cusps[nBulges]]
		baseEnd := co.points[co.cusps[0]]
		d.MoveTo(baseStart)
		d.LineTo(baseEnd)
	} else {
		start := co.points[co.cusps[0]]
		d.MoveTo(start)
	}

	for i := range nBulges {
		cp1, cp2, endPt := co.bulgeControlPoints(i)
		d.CubeTo(cp1, cp2, endPt)
	}

	d.Close()
	return d.Iter()
}

// stroke returns the open stroke path with cusp crossings, at full precision.
func (co *cloudOutline) stroke() path.Path {
	var d path.Data
	nBulges := co.numBulges()

	start := co.points[co.cusps[0]]
	d.MoveTo(start)

	for i := range nBulges {
		endIdx := co.bulgeEnd(i)
		startPt := co.points[co.cusps[i]]
		cp1, cp2, endPt := co.bulgeControlPoints(i)
		chord := endPt.Sub(startPt).Length()

		d.CubeTo(cp1, cp2, endPt)

		// cusp crossing at non-base-transition cusps
		isBaseTrans := co.hasBase && (endIdx == 0 || endIdx == nBulges)
		if !isBaseTrans && !co.noTicks {
			theta := tangentAngle(co.points, co.cusps[endIdx])
			extAngle := theta + 3*math.Pi/4
			ext := 0.1 * chord
			extPt := endPt.Add(vec.Vec2{
				X: ext * math.Cos(extAngle),
				Y: ext * math.Sin(extAngle),
			})
			d.MoveTo(extPt)
			d.LineTo(endPt)
		}
	}

	// base line
	if co.hasBase {
		baseEnd := co.points[co.cusps[0]]
		d.LineTo(baseEnd)
	}

	return d.Iter()
}

// drawPath draws path p and returns the rectangle its ink lies in, at full
// precision.
func drawPath(b *builder.Builder, p path.Path) pdf.Rectangle {
	b.DrawPath(p, 2)
	return pathExtent(p)
}

// fillPath draws the closed fill path and returns the bounding box.
func (co *cloudOutline) fillPath(b *builder.Builder) pdf.Rectangle {
	return drawPath(b, co.fill())
}

// strokePath draws the open stroke path with cusp crossings.
func (co *cloudOutline) strokePath(b *builder.Builder) pdf.Rectangle {
	return drawPath(b, co.stroke())
}

// bulgeControlPoints computes the Bezier control points for bulge i.
func (co *cloudOutline) bulgeControlPoints(i int) (cp1, cp2, endPt vec.Vec2) {
	nBulges := co.numBulges()
	endIdx := co.bulgeEnd(i)

	p0 := co.points[co.cusps[i]]
	p3 := co.points[co.cusps[endIdx]]
	chord := p3.Sub(p0).Length()
	cpDist := 0.5 * chord

	thetaStart := tangentAngle(co.points, co.cusps[i])
	thetaEnd := tangentAngle(co.points, co.cusps[endIdx])

	// 45° offset toward exterior for normal cusps;
	// at base transitions, use the base line direction for a smooth join
	// and extend control points 20% further for a fuller curve
	outAngle := thetaStart - math.Pi/4
	cpDistStart := cpDist
	if co.hasBase && i == 0 {
		outAngle = co.baseAngle()
		cpDistStart = cpDist * 1.2
	}
	inAngle := thetaEnd + math.Pi/4
	cpDistEnd := cpDist
	if co.hasBase && endIdx == nBulges {
		inAngle = co.baseAngle()
		cpDistEnd = cpDist * 1.2
	}

	cp1 = p0.Add(vec.Vec2{
		X: cpDistStart * math.Cos(outAngle),
		Y: cpDistStart * math.Sin(outAngle),
	})
	cp2 = p3.Sub(vec.Vec2{
		X: cpDistEnd * math.Cos(inAngle),
		Y: cpDistEnd * math.Sin(inAngle),
	})
	endPt = p3
	return
}

// baseAngle returns the direction angle of the flat base line,
// from cusps[nBulges] to cusps[0].
func (co *cloudOutline) baseAngle() float64 {
	nBulges := co.numBulges()
	from := co.points[co.cusps[nBulges]]
	to := co.points[co.cusps[0]]
	d := to.Sub(from)
	return math.Atan2(d.Y, d.X)
}

// trimToCloud returns where the segment from outside towards inside first
// crosses the cloud's stroke outline, or inside itself when it never does.
func (co *cloudOutline) trimToCloud(outside, inside vec.Vec2) vec.Vec2 {
	best := inside
	bestT := math.Inf(1)

	consider := func(a, b vec.Vec2) {
		p, t, _, ok := linalg.SegmentIntersection(outside, inside, a, b)
		if ok && t < bestT {
			bestT = t
			best = p
		}
	}

	const flattenSteps = 16
	nBulges := co.numBulges()
	for i := range nBulges {
		p0 := co.points[co.cusps[i]]
		cp1, cp2, endPt := co.bulgeControlPoints(i)
		flattenCubic(p0, cp1, cp2, endPt, flattenSteps, consider)
	}
	if co.hasBase {
		baseStart := co.points[co.cusps[nBulges]]
		baseEnd := co.points[co.cusps[0]]
		consider(baseStart, baseEnd)
	}

	return best
}

// flattenCubic approximates a cubic Bezier curve with n line segments and
// calls consider on each segment's endpoints.
func flattenCubic(p0, cp1, cp2, p3 vec.Vec2, n int, consider func(a, b vec.Vec2)) {
	prev := p0
	for i := 1; i <= n; i++ {
		pt := path.EvalCubic(p0, cp1, cp2, p3, float64(i)/float64(n))
		consider(prev, pt)
		prev = pt
	}
}

// tangentAngle computes the forward tangent angle at point k
// from its neighbors.
func tangentAngle(points []vec.Vec2, k int) float64 {
	n := len(points)
	prev := (k - 1 + n) % n
	next := (k + 1) % n
	d := points[next].Sub(points[prev])
	return math.Atan2(d.Y, d.X)
}

// findFlatBase detects a flat base edge in the equidistant points.
// Returns the start point index and the number of horizontal segments.
func findFlatBase(points []vec.Vec2) (start, segCount int) {
	n := len(points)
	if n < 4 {
		return 0, 0
	}

	const maxAngle = 15.0 * math.Pi / 180

	// classify each segment as near-horizontal
	isHoriz := make([]bool, n)
	for i := range n {
		j := (i + 1) % n
		d := points[j].Sub(points[i])
		angle := math.Atan2(d.Y, d.X)
		isHoriz[i] = math.Abs(angle) <= maxAngle ||
			math.Abs(angle-math.Pi) <= maxAngle ||
			math.Abs(angle+math.Pi) <= maxAngle
	}

	// find minimum and maximum y
	minY := math.Inf(1)
	maxY := math.Inf(-1)
	for _, p := range points {
		minY = min(minY, p.Y)
		maxY = max(maxY, p.Y)
	}
	yRange := maxY - minY

	// find the longest horizontal run near minimum y,
	// using a double pass for circular wrap-around
	bestStart := 0
	bestLen := 0
	runStart := 0
	runLen := 0

	for i := range 2 * n {
		idx := i % n
		if isHoriz[idx] {
			if runLen == 0 {
				runStart = idx
			}
			runLen++
			if runLen > n {
				runLen = n
			}
		} else {
			if runLen > bestLen && runNearMinY(points, runStart, runLen, minY, yRange) {
				bestStart = runStart
				bestLen = runLen
			}
			runLen = 0
		}
	}
	if runLen > bestLen && runNearMinY(points, runStart, runLen, minY, yRange) {
		bestStart = runStart
		bestLen = runLen
	}

	if bestLen < n/4 {
		return 0, 0
	}

	return bestStart, bestLen
}

// runNearMinY checks whether the average y of a run of points is near
// the minimum y of the polygon.
func runNearMinY(points []vec.Vec2, start, length int, minY, yRange float64) bool {
	if yRange < 1e-9 {
		return true
	}
	n := len(points)
	var sumY float64
	for i := range length {
		idx := (start + i) % n
		sumY += points[idx].Y
	}
	avgY := sumY / float64(length)
	return avgY-minY <= 0.1*yRange
}

// drawCloudyBorder draws a cloudy border for a polygon.
// Returns the bounding box. Falls back to a plain polygon if too few bulges.
func drawCloudyBorder(b *builder.Builder, vertices []vec.Vec2,
	intensity, lw float64, hasFill, hasStroke bool) pdf.Rectangle {

	co := newCloudOutline(vertices, intensity, lw)
	if co == nil {
		bbox := drawPlainPolygon(b, vertices)
		switch {
		case hasFill && hasStroke:
			b.FillAndStroke()
		case hasFill:
			b.Fill()
		case hasStroke:
			b.Stroke()
		}
		return bbox
	}

	var bbox pdf.Rectangle

	if hasFill {
		fillBBox := co.fillPath(b)
		b.Fill()
		bbox = fillBBox
	}

	if hasStroke {
		// A cloud has no square corner anywhere, and the outside of a cusp
		// is where the path turns almost back on itself: a miter there
		// blows the limit and falls back to a bevel, which cuts a wedge out
		// of the stroke once the pen is wide.
		b.SetLineCap(graphics.LineCapRound)
		b.SetLineJoin(graphics.LineJoinRound)
		strokeBBox := co.strokePath(b)
		b.Stroke()
		if hasFill {
			bbox.Extend(&strokeBBox)
		} else {
			bbox = strokeBBox
		}
	}

	return bbox
}

// drawPlainPolygon draws the vertices as a simple closed polygon path.
func drawPlainPolygon(b *builder.Builder, vertices []vec.Vec2) pdf.Rectangle {
	bbox := pdf.RectangleFromPoints(vertices...)
	for i, v := range vertices {
		x := pdf.Round(v.X, 2)
		y := pdf.Round(v.Y, 2)
		if i == 0 {
			b.MoveTo(x, y)
		} else {
			b.LineTo(x, y)
		}
	}
	b.ClosePath()
	return bbox
}
