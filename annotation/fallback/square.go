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
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addSquareAppearance(a *annotation.Square) *form.Form {
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

	b := builder.New(content.Form, nil, s.Version)

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
		b.SetLineDash(dashPattern, 0)
	}
	if hasFill {
		b.SetFillColor(a.FillColor)
	}

	var bbox pdf.Rectangle
	if isCloudy {
		// CCW rectangle inset by half the line width
		x0 := rect.LLx + lw/2
		y0 := rect.LLy + lw/2
		x1 := rect.URx - lw/2
		y1 := rect.URy - lw/2
		verts := []vec.Vec2{
			{X: x0, Y: y0},
			{X: x1, Y: y0},
			{X: x1, Y: y1},
			{X: x0, Y: y1},
		}
		cloudBBox := drawCloudyBorder(b, verts, be.Intensity, lw, hasFill, hasOutline)
		// expand by half line width for stroke
		bbox = pdf.Rectangle{
			LLx: cloudBBox.LLx - lw/2,
			LLy: cloudBBox.LLy - lw/2,
			URx: cloudBBox.URx + lw/2,
			URy: cloudBBox.URy + lw/2,
		}
		bbox.IRound(2)
	} else {
		b.Rectangle(rect.LLx+lw/2, rect.LLy+lw/2, rect.Dx()-lw, rect.Dy()-lw)
		bbox = rect
		switch {
		case hasOutline && hasFill:
			b.FillAndStroke()
		case hasFill:
			b.Fill()
		default: // hasOutline
			b.Stroke()
		}
	}

	if isCloudy {
		// update Rect and Margin to reflect the expanded bounding box
		a.Margin = []float64{
			max(0, rect.LLx-bbox.LLx),
			max(0, rect.LLy-bbox.LLy),
			max(0, bbox.URx-rect.URx),
			max(0, bbox.URy-rect.URy),
		}
		a.Rect = bbox
	} else {
		a.Rect = rect
	}

	return &form.Form{
		Content: builder.Must(b.Harvest()),
		Res:     b.Resources,
		BBox:    bbox,
	}
}
