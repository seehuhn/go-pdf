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
	"math"

	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addLineAppearance(a *annotation.Line) {
	// extract line properties
	lw := getLineWidth(a)
	dashPattern := getDashPattern(a)

	// calculate bounding box
	bbox := calculateLineBBox(a, lw)

	// create drawing function
	draw := func(w *graphics.Writer) error {
		// set line properties
		w.SetLineWidth(lw)
		w.SetStrokeColor(color.Black)
		if len(dashPattern) > 0 {
			w.SetLineDash(dashPattern, 0)
		}

		// draw the line based on whether we have leader lines
		if a.LL != 0 {
			drawLineWithLeaderLines(w, a, lw)
		} else {
			drawSimpleLine(w, a, lw)
		}

		return nil
	}

	// create appearance stream
	xObj := &form.Form{
		Draw: draw,
		BBox: bbox,
	}
	a.Appearance = &appearance.Dict{
		Normal: xObj,
	}

	// set annotation rect
	a.Rect = bbox
}

// getLineWidth returns the line width from BorderStyle, Border, or default
func getLineWidth(a *annotation.Line) float64 {
	if a.BorderStyle != nil {
		return a.BorderStyle.Width
	}
	if a.Common.Border != nil {
		return a.Common.Border.Width
	}
	return 1 // default
}

// getDashPattern returns the dash pattern if any
func getDashPattern(a *annotation.Line) []float64 {
	if a.BorderStyle != nil {
		if a.BorderStyle.Style != "D" {
			return nil
		}
		return a.BorderStyle.DashArray
	} else if a.Common.Border != nil {
		return a.Common.Border.DashArray
	}
	return nil // solid line
}

// calculateLineBBox calculates the bounding box for the line annotation
func calculateLineBBox(a *annotation.Line, lw float64) pdf.Rectangle {
	x1, y1 := a.Coords[0], a.Coords[1]
	x2, y2 := a.Coords[2], a.Coords[3]

	// start with basic line bounds
	bbox := pdf.Rectangle{
		LLx: min(x1, x2),
		LLy: min(y1, y2),
		URx: max(x1, x2),
		URy: max(y1, y2),
	}

	// expand for line width
	bbox.LLx -= lw / 2
	bbox.LLy -= lw / 2
	bbox.URx += lw / 2
	bbox.URy += lw / 2

	// calculate direction vector
	dx := x2 - x1
	dy := y2 - y1

	// expand for line endings
	if a.LineEndingStyle[0] != "" && a.LineEndingStyle[0] != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:  vec.Vec2{X: x1, Y: y1},
			Dir: vec.Vec2{X: -dx, Y: -dy},
		}
		lineEndingBBox(&bbox, a.LineEndingStyle[0], info, lw)
	}

	if a.LineEndingStyle[1] != "" && a.LineEndingStyle[1] != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:  vec.Vec2{X: x2, Y: y2},
			Dir: vec.Vec2{X: dx, Y: dy},
		}
		lineEndingBBox(&bbox, a.LineEndingStyle[1], info, lw)
	}

	// expand for leader lines if present
	if a.LL != 0 {
		expandBBoxForLeaderLines(&bbox, a, lw)
	}

	bbox.Round(1)
	return bbox
}

// expandBBoxForLeaderLines expands the bounding box to include leader lines
func expandBBoxForLeaderLines(bbox *pdf.Rectangle, a *annotation.Line, lw float64) {
	x1, y1 := a.Coords[0], a.Coords[1]
	x2, y2 := a.Coords[2], a.Coords[3]

	// calculate perpendicular direction (left when looking from start to end)
	dx := x2 - x1
	dy := y2 - y1
	length := math.Sqrt(dx*dx + dy*dy)
	if length < 0.1 {
		return
	}

	// perpendicular vector (rotated 90° counter-clockwise)
	perpX := -dy / length
	perpY := dx / length

	// calculate leader line endpoints
	// start point with offset
	startX := x1 + a.LLO*dx/length
	startY := y1 + a.LLO*dy/length

	// end point with offset
	endX := x2 - a.LLO*dx/length
	endY := y2 - a.LLO*dy/length

	// shifted line endpoints
	shiftedStartX := startX + a.LL*perpX
	shiftedStartY := startY + a.LL*perpY
	shiftedEndX := endX + a.LL*perpX
	shiftedEndY := endY + a.LL*perpY

	// extension points
	extStartX := shiftedStartX - a.LLE*perpX
	extStartY := shiftedStartY - a.LLE*perpY
	extEndX := shiftedEndX - a.LLE*perpX
	extEndY := shiftedEndY - a.LLE*perpY

	// include all points in bbox
	points := []float64{
		startX, startY,
		endX, endY,
		shiftedStartX, shiftedStartY,
		shiftedEndX, shiftedEndY,
		extStartX, extStartY,
		extEndX, extEndY,
	}

	for i := 0; i < len(points); i += 2 {
		x, y := points[i], points[i+1]
		bbox.LLx = min(bbox.LLx, x-lw/2)
		bbox.LLy = min(bbox.LLy, y-lw/2)
		bbox.URx = max(bbox.URx, x+lw/2)
		bbox.URy = max(bbox.URy, y+lw/2)
	}
}

// drawSimpleLine draws a line without leader lines
func drawSimpleLine(w *graphics.Writer, a *annotation.Line, lw float64) {
	x1, y1 := a.Coords[0], a.Coords[1]
	x2, y2 := a.Coords[2], a.Coords[3]

	// draw start ending if present
	if a.LineEndingStyle[0] != "" && a.LineEndingStyle[0] != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:        vec.Vec2{X: x1, Y: y1},
			Dir:       vec.Vec2{X: x1 - x2, Y: y1 - y2},
			FillColor: a.FillColor,
			IsStart:   true,
		}
		drawLineEnding(w, a.LineEndingStyle[0], info)
	} else {
		w.MoveTo(pdf.Round(x1, 2), pdf.Round(y1, 2))
	}

	// draw end ending if present
	if a.LineEndingStyle[1] != "" && a.LineEndingStyle[1] != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:        vec.Vec2{X: x2, Y: y2},
			Dir:       vec.Vec2{X: x2 - x1, Y: y2 - y1},
			FillColor: a.FillColor,
			IsStart:   false,
		}
		drawLineEnding(w, a.LineEndingStyle[1], info)
	} else {
		w.LineTo(pdf.Round(x2, 2), pdf.Round(y2, 2))
		w.Stroke()
	}
}

// drawLineWithLeaderLines draws a line with leader lines (dimension line style)
func drawLineWithLeaderLines(w *graphics.Writer, a *annotation.Line, lw float64) {
	x1, y1 := a.Coords[0], a.Coords[1]
	x2, y2 := a.Coords[2], a.Coords[3]

	// calculate direction and perpendicular vectors
	dx := x2 - x1
	dy := y2 - y1
	length := math.Sqrt(dx*dx + dy*dy)
	if length < 0.1 {
		// line too short, fall back to simple line
		drawSimpleLine(w, a, lw)
		return
	}

	// unit vectors
	dirX := dx / length
	dirY := dy / length

	// perpendicular vector (rotated 90° counter-clockwise)
	perpX := -dirY
	perpY := dirX

	// calculate key points
	// start point with offset
	startX := x1 + a.LLO*dirX
	startY := y1 + a.LLO*dirY

	// end point with offset
	endX := x2 - a.LLO*dirX
	endY := y2 - a.LLO*dirY

	// shifted line endpoints
	shiftedStartX := startX + a.LL*perpX
	shiftedStartY := startY + a.LL*perpY
	shiftedEndX := endX + a.LL*perpX
	shiftedEndY := endY + a.LL*perpY

	// extension points (for leader line extensions)
	extStartX := shiftedStartX - a.LLE*perpX
	extStartY := shiftedStartY - a.LLE*perpY
	extEndX := shiftedEndX - a.LLE*perpX
	extEndY := shiftedEndY - a.LLE*perpY

	// draw the leader lines (perpendicular segments)
	// start leader line
	w.MoveTo(pdf.Round(extStartX, 2), pdf.Round(extStartY, 2))
	w.LineTo(pdf.Round(startX, 2), pdf.Round(startY, 2))
	w.Stroke()

	// end leader line
	w.MoveTo(pdf.Round(extEndX, 2), pdf.Round(extEndY, 2))
	w.LineTo(pdf.Round(endX, 2), pdf.Round(endY, 2))
	w.Stroke()

	// draw the main line with endings
	// start ending
	if a.LineEndingStyle[0] != "" && a.LineEndingStyle[0] != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:        vec.Vec2{X: shiftedStartX, Y: shiftedStartY},
			Dir:       vec.Vec2{X: shiftedStartX - shiftedEndX, Y: shiftedStartY - shiftedEndY},
			FillColor: a.FillColor,
			IsStart:   true,
		}
		drawLineEnding(w, a.LineEndingStyle[0], info)
	} else {
		w.MoveTo(pdf.Round(shiftedStartX, 2), pdf.Round(shiftedStartY, 2))
	}

	// end ending
	if a.LineEndingStyle[1] != "" && a.LineEndingStyle[1] != annotation.LineEndingStyleNone {
		info := lineEndingInfo{
			At:        vec.Vec2{X: shiftedEndX, Y: shiftedEndY},
			Dir:       vec.Vec2{X: shiftedEndX - shiftedStartX, Y: shiftedEndY - shiftedStartY},
			FillColor: a.FillColor,
			IsStart:   false,
		}
		drawLineEnding(w, a.LineEndingStyle[1], info)
	} else {
		w.LineTo(pdf.Round(shiftedEndX, 2), pdf.Round(shiftedEndY, 2))
		w.Stroke()
	}
}
