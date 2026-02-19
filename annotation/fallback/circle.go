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
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addCircleAppearance(a *annotation.Circle) *form.Form {
	lw := getBorderLineWidth(a.Common.Border, a.BorderStyle)
	dashPattern := getBorderDashPattern(a.Common.Border, a.BorderStyle)
	col := a.Color

	rect := applyMargins(a.Rect, a.Margin)

	if m := min(rect.Dx(), rect.Dy()); lw > m/2 {
		lw = m / 2
	}

	be := a.BorderEffect
	isCloudy := be != nil && be.Style == "C" && be.Intensity > 0

	hasOutline := col != nil && lw > 0
	hasFill := a.FillColor != nil
	if !(hasOutline || hasFill) {
		a.Rect = rect
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    rect,
		}
	}

	b := builder.New(content.Form, nil)

	b.SetExtGState(s.reset)
	if a.StrokingTransparency != 0 || a.NonStrokingTransparency != 0 {
		gs := &extgstate.ExtGState{
			Set:         graphics.StateStrokeAlpha | graphics.StateFillAlpha,
			StrokeAlpha: 1 - a.StrokingTransparency,
			FillAlpha:   1 - a.NonStrokingTransparency,
			SingleUse:   true,
		}
		b.SetExtGState(gs)
	}

	if hasOutline {
		b.SetLineWidth(lw)
		b.SetStrokeColor(col)
		if len(dashPattern) > 0 {
			b.SetLineDash(dashPattern, 0)
		}
	}
	if hasFill {
		b.SetFillColor(a.FillColor)
	}

	var cloudVerts []vec.Vec2
	if isCloudy {
		cloudVerts = flattenEllipse(rect, lw)
		isCloudy = cloudVerts != nil
	}

	var bbox pdf.Rectangle
	if isCloudy {
		cloudBBox := drawCloudyBorder(b, cloudVerts, be.Intensity, lw, hasFill, hasOutline)
		bbox = pdf.Rectangle{
			LLx: cloudBBox.LLx - lw/2,
			LLy: cloudBBox.LLy - lw/2,
			URx: cloudBBox.URx + lw/2,
			URy: cloudBBox.URy + lw/2,
		}
		bbox.IRound(1)
		a.Margin = []float64{
			max(0, rect.LLx-bbox.LLx),
			max(0, rect.LLy-bbox.LLy),
			max(0, bbox.URx-rect.URx),
			max(0, bbox.URy-rect.URy),
		}
		a.Rect = bbox
	} else {
		xMid := (rect.LLx + rect.URx) / 2
		yMid := (rect.LLy + rect.URy) / 2
		rx := (rect.Dx() - lw) / 2
		ry := (rect.Dy() - lw) / 2

		k := (math.Sqrt2 - 1.0) * 4 / 3

		b.MoveTo(xMid+rx, yMid)
		b.CurveTo(xMid+rx, yMid+ry*k, xMid+rx*k, yMid+ry, xMid, yMid+ry)
		b.CurveTo(xMid-rx*k, yMid+ry, xMid-rx, yMid+ry*k, xMid-rx, yMid)
		b.CurveTo(xMid-rx, yMid-ry*k, xMid-rx*k, yMid-ry, xMid, yMid-ry)
		b.CurveTo(xMid+rx*k, yMid-ry, xMid+rx, yMid-ry*k, xMid+rx, yMid)
		b.ClosePath()
		bbox = rect
		a.Rect = rect
		switch {
		case hasOutline && hasFill:
			b.FillAndStroke()
		case hasFill:
			b.Fill()
		default: // hasOutline
			b.Stroke()
		}
	}

	return &form.Form{
		Content: b.Stream,
		Res:     b.Resources,
		BBox:    bbox,
	}
}

// flattenEllipse approximates the ellipse inscribed in rect (inset by lw/2) as
// a polygon with enough vertices for smooth cloud curls.
func flattenEllipse(rect pdf.Rectangle, lw float64) []vec.Vec2 {
	xMid := (rect.LLx + rect.URx) / 2
	yMid := (rect.LLy + rect.URy) / 2
	rx := (rect.Dx() - lw) / 2
	ry := (rect.Dy() - lw) / 2

	if rx < 0.5 || ry < 0.5 {
		return nil
	}

	// approximate perimeter (Ramanujan)
	h := (rx - ry) * (rx - ry) / ((rx + ry) * (rx + ry))
	perimeter := math.Pi * (rx + ry) * (1 + 3*h/(10+math.Sqrt(4-3*h)))

	// target segment length ~4 points
	n := max(12, int(math.Ceil(perimeter/4)))

	verts := make([]vec.Vec2, n)
	for i := range n {
		theta := 2 * math.Pi * float64(i) / float64(n)
		verts[i] = vec.Vec2{
			X: xMid + rx*math.Cos(theta),
			Y: yMid + ry*math.Sin(theta),
		}
	}
	return verts
}
