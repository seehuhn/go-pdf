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
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// cloudOutline holds precomputed cloud border geometry.
type cloudOutline struct {
	points  []vec.Vec2 // equidistant boundary points (CCW)
	cusps   []int      // point indices for bulge start/end
	hasBase bool       // whether a flat base was detected
}

// newCloudOutline computes the cloud outline for a polygon.
// Returns nil if the polygon is too small for cloud bulges (< 3 bulges).
func newCloudOutline(vertices []vec.Vec2, intensity, lw float64) *cloudOutline {
	n := len(vertices)
	if n < 3 {
		return nil
	}

	// ensure CCW winding
	if signedArea(vertices) < 0 {
		rev := make([]vec.Vec2, n)
		for i, v := range vertices {
			rev[n-1-i] = v
		}
		vertices = rev
	}

	// step 1: sample equidistant points
	points := sampleEquidistant(vertices, lw)
	if len(points) < 3 {
		return nil
	}

	// step 2: detect flat base
	baseStart, baseSeg := findFlatBase(points)

	// step 3: determine bulge count
	ppb := max(2, int(math.Round(3*intensity)))
	nPoints := len(points)

	hasBase := baseSeg > 0
	cloudLen := nPoints
	if hasBase {
		cloudLen = nPoints - baseSeg
	}
	nBulges := int(math.Round(float64(cloudLen) / float64(ppb)))

	if nBulges < 3 {
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

// fillPath draws the closed fill path and returns the bounding box.
func (co *cloudOutline) fillPath(b *builder.Builder) pdf.Rectangle {
	var bbox pdf.Rectangle
	nBulges := co.numBulges()

	if co.hasBase {
		// base line
		baseStart := co.points[co.cusps[nBulges]]
		baseEnd := co.points[co.cusps[0]]
		b.MoveTo(pdf.Round(baseStart.X, 2), pdf.Round(baseStart.Y, 2))
		b.LineTo(pdf.Round(baseEnd.X, 2), pdf.Round(baseEnd.Y, 2))
		bbox.ExtendVec(baseStart)
		bbox.ExtendVec(baseEnd)
	} else {
		start := co.points[co.cusps[0]]
		b.MoveTo(pdf.Round(start.X, 2), pdf.Round(start.Y, 2))
		bbox.ExtendVec(start)
	}

	for i := range nBulges {
		cp1, cp2, endPt := co.bulgeControlPoints(i)
		bbox.ExtendVec(cp1)
		bbox.ExtendVec(cp2)
		bbox.ExtendVec(endPt)
		b.CurveTo(
			pdf.Round(cp1.X, 2), pdf.Round(cp1.Y, 2),
			pdf.Round(cp2.X, 2), pdf.Round(cp2.Y, 2),
			pdf.Round(endPt.X, 2), pdf.Round(endPt.Y, 2),
		)
	}

	b.ClosePath()
	return bbox
}

// strokePath draws the open stroke path with cusp crossings.
func (co *cloudOutline) strokePath(b *builder.Builder) pdf.Rectangle {
	var bbox pdf.Rectangle
	nBulges := co.numBulges()

	start := co.points[co.cusps[0]]
	b.MoveTo(pdf.Round(start.X, 2), pdf.Round(start.Y, 2))
	bbox.ExtendVec(start)

	for i := range nBulges {
		endIdx := co.bulgeEnd(i)
		startPt := co.points[co.cusps[i]]
		cp1, cp2, endPt := co.bulgeControlPoints(i)
		chord := endPt.Sub(startPt).Length()

		bbox.ExtendVec(cp1)
		bbox.ExtendVec(cp2)
		bbox.ExtendVec(endPt)
		b.CurveTo(
			pdf.Round(cp1.X, 2), pdf.Round(cp1.Y, 2),
			pdf.Round(cp2.X, 2), pdf.Round(cp2.Y, 2),
			pdf.Round(endPt.X, 2), pdf.Round(endPt.Y, 2),
		)

		// cusp crossing at non-base-transition cusps
		isBaseTrans := co.hasBase && (endIdx == 0 || endIdx == nBulges)
		if !isBaseTrans {
			theta := tangentAngle(co.points, co.cusps[endIdx])
			extAngle := theta + 3*math.Pi/4
			ext := 0.1 * chord
			extPt := endPt.Add(vec.Vec2{
				X: ext * math.Cos(extAngle),
				Y: ext * math.Sin(extAngle),
			})
			b.MoveTo(pdf.Round(extPt.X, 2), pdf.Round(extPt.Y, 2))
			b.LineTo(pdf.Round(endPt.X, 2), pdf.Round(endPt.Y, 2))
			bbox.ExtendVec(extPt)
		}
	}

	// base line
	if co.hasBase {
		baseEnd := co.points[co.cusps[0]]
		b.LineTo(pdf.Round(baseEnd.X, 2), pdf.Round(baseEnd.Y, 2))
		bbox.ExtendVec(baseEnd)
	}

	return bbox
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

// tangentAngle computes the forward tangent angle at point k
// from its neighbors.
func tangentAngle(points []vec.Vec2, k int) float64 {
	n := len(points)
	prev := (k - 1 + n) % n
	next := (k + 1) % n
	d := points[next].Sub(points[prev])
	return math.Atan2(d.Y, d.X)
}

// sampleEquidistant places equidistant points along the polygon boundary.
func sampleEquidistant(vertices []vec.Vec2, lw float64) []vec.Vec2 {
	n := len(vertices)
	if n < 3 {
		return nil
	}

	// compute edge lengths and total perimeter
	edgeLens := make([]float64, n)
	var perimeter float64
	for i := range n {
		j := (i + 1) % n
		edgeLens[i] = vertices[j].Sub(vertices[i]).Length()
		perimeter += edgeLens[i]
	}

	if perimeter < 1e-9 {
		return nil
	}

	targetDist := min(4*(lw+1), 20)
	nPoints := max(3, int(math.Round(perimeter/targetDist)))
	actualDist := perimeter / float64(nPoints)

	points := make([]vec.Vec2, 0, nPoints)

	for i := range nPoints {
		dist := float64(i) * actualDist

		// find which edge this distance falls on
		d := dist
		for e := range n {
			if d <= edgeLens[e]+1e-9 {
				var t float64
				if edgeLens[e] > 1e-9 {
					t = min(d/edgeLens[e], 1)
				}
				a := vertices[e]
				b := vertices[(e+1)%n]
				p := a.Add(b.Sub(a).Mul(t))
				points = append(points, p)
				break
			}
			d -= edgeLens[e]
		}
	}

	return points
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
		b.SetLineCap(graphics.LineCapRound)
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
	var bbox pdf.Rectangle
	for i, v := range vertices {
		bbox.ExtendVec(v)
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

// signedArea returns twice the signed area of a polygon. Positive means CCW.
func signedArea(vertices []vec.Vec2) float64 {
	n := len(vertices)
	var area float64
	for i := range n {
		j := (i + 1) % n
		area += vertices[i].X*vertices[j].Y - vertices[j].X*vertices[i].Y
	}
	return area
}
