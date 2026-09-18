// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

func (g *Generator) addLineAppearance(a *annotation.Line) (*form.Form, error) {
	lw := annotation.EffectiveBorderWidth(a)
	dashPattern := annotation.EffectiveBorderDash(a)

	bbox := calculateLineBBox(a, lw)
	a.Rect = bbox

	b := builder.New(content.Form, nil, g.version)

	// the border width is the thickness of the line itself, so a width of 0
	// leaves the annotation with nothing to draw
	if lw <= 0 {
		g.reset(b)
		return harvest(b, bbox)
	}

	g.reset(b)
	b.SetLineWidth(lw)
	b.SetStrokeColor(quireInk)
	b.SetLineDash(dashPattern, 0)

	if a.LL != 0 {
		drawLineWithLeaderLinesBuilder(b, a)
	} else {
		drawSimpleLineBuilder(b, a)
	}

	return harvest(b, bbox)
}

// calculateLineBBox calculates the bounding box for the line annotation
func calculateLineBBox(a *annotation.Line, lw float64) pdf.Rectangle {
	x1, y1 := a.Coords[0], a.Coords[1]
	x2, y2 := a.Coords[2], a.Coords[3]

	// the line itself; it has two points, so it carries caps and no join
	segment := []vec.Vec2{{X: x1, Y: y1}, {X: x2, Y: y2}}
	bbox, _ := strokeBounds([][]vec.Vec2{segment}, false, lw,
		graphics.LineJoinMiter, graphics.DefaultMiterLimit)

	// expand for line endings
	le0 := normalizeLE(a.LineEndingStyle[0])
	le1 := normalizeLE(a.LineEndingStyle[1])
	if le0 != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:  vec.Vec2{X: x1, Y: y1},
			Dir: vec.Vec2{X: x1 - x2, Y: y1 - y2},
		}
		lineEndingBBox(&bbox, le0, info, lw)
	}
	if le1 != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:  vec.Vec2{X: x2, Y: y2},
			Dir: vec.Vec2{X: x2 - x1, Y: y2 - y1},
		}
		lineEndingBBox(&bbox, le1, info, lw)
	}

	// expand for leader lines if present
	if a.LL != 0 {
		expandBBoxForLeaderLines(&bbox, a, lw)
	}

	bbox.IRound(2)
	return bbox
}

// expandBBoxForLeaderLines expands the bounding box to include leader lines
func expandBBoxForLeaderLines(bbox *pdf.Rectangle, a *annotation.Line, lw float64) {
	from := vec.Vec2{X: a.Coords[0], Y: a.Coords[1]}
	to := vec.Vec2{X: a.Coords[2], Y: a.Coords[3]}

	d := to.Sub(from)
	if d.Length() < 0.1 {
		return
	}
	dir := d.Normalize()
	perp := d.Normal() // left when looking from start to end

	// leader line endpoints, offset along the line
	start := from.Add(dir.Mul(a.LLO))
	end := to.Sub(dir.Mul(a.LLO))

	// the line proper, drawn LL away from the coordinates, and the extensions
	// which reach LLE back from it towards them
	shiftedStart := start.Add(perp.Mul(a.LL))
	shiftedEnd := end.Add(perp.Mul(a.LL))
	extStart := shiftedStart.Sub(perp.Mul(a.LLE))
	extEnd := shiftedEnd.Sub(perp.Mul(a.LLE))

	points := []vec.Vec2{start, end, shiftedStart, shiftedEnd, extStart, extEnd}
	for _, p := range points {
		bbox.LLx = min(bbox.LLx, p.X-lw/2)
		bbox.LLy = min(bbox.LLy, p.Y-lw/2)
		bbox.URx = max(bbox.URx, p.X+lw/2)
		bbox.URy = max(bbox.URy, p.Y+lw/2)
	}
}

// drawSimpleLineBuilder draws a line without leader lines
func drawSimpleLineBuilder(b *builder.Builder, a *annotation.Line) {
	points := []vec.Vec2{
		{X: a.Coords[0], Y: a.Coords[1]},
		{X: a.Coords[2], Y: a.Coords[3]},
	}
	le0 := normalizeLE(a.LineEndingStyle[0])
	le1 := normalizeLE(a.LineEndingStyle[1])
	drawOpenPolyline(b, points, le0, le1, paint(a.FillColor))
}

// drawLineWithLeaderLinesBuilder draws a line with leader lines (dimension line style)
func drawLineWithLeaderLinesBuilder(b *builder.Builder, a *annotation.Line) {
	from := vec.Vec2{X: a.Coords[0], Y: a.Coords[1]}
	to := vec.Vec2{X: a.Coords[2], Y: a.Coords[3]}

	d := to.Sub(from)
	if d.Length() < 0.1 {
		// line too short, fall back to simple line
		drawSimpleLineBuilder(b, a)
		return
	}
	dir := d.Normalize()
	perp := d.Normal() // left when looking from start to end

	// leader line endpoints, offset along the line
	start := from.Add(dir.Mul(a.LLO))
	end := to.Sub(dir.Mul(a.LLO))

	// the line proper, drawn LL away from the coordinates, and the extensions
	// which reach LLE back from it towards them
	shiftedStart := start.Add(perp.Mul(a.LL))
	shiftedEnd := end.Add(perp.Mul(a.LL))
	extStart := shiftedStart.Sub(perp.Mul(a.LLE))
	extEnd := shiftedEnd.Sub(perp.Mul(a.LLE))

	// draw the leader lines (perpendicular segments)
	// start leader line
	b.MoveTo(pdf.Round(extStart.X, 2), pdf.Round(extStart.Y, 2))
	b.LineTo(pdf.Round(start.X, 2), pdf.Round(start.Y, 2))
	b.Stroke()

	// end leader line
	b.MoveTo(pdf.Round(extEnd.X, 2), pdf.Round(extEnd.Y, 2))
	b.LineTo(pdf.Round(end.X, 2), pdf.Round(end.Y, 2))
	b.Stroke()

	// draw the main line with endings
	// start ending
	if a.LineEndingStyle[0] != "" && a.LineEndingStyle[0] != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:        shiftedStart,
			Dir:       shiftedStart.Sub(shiftedEnd),
			FillColor: paint(a.FillColor),
			IsStart:   true,
		}
		drawLineEndingBuilder(b, a.LineEndingStyle[0], info)
	} else {
		b.MoveTo(pdf.Round(shiftedStart.X, 2), pdf.Round(shiftedStart.Y, 2))
	}

	// end ending
	if a.LineEndingStyle[1] != "" && a.LineEndingStyle[1] != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:        shiftedEnd,
			Dir:       shiftedEnd.Sub(shiftedStart),
			FillColor: paint(a.FillColor),
			IsStart:   false,
		}
		drawLineEndingBuilder(b, a.LineEndingStyle[1], info)
	} else {
		b.LineTo(pdf.Round(shiftedEnd.X, 2), pdf.Round(shiftedEnd.Y, 2))
		b.Stroke()
	}
}
