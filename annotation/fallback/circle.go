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

	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/state"
)

func (s *Style) addCircleAppearance(a *annotation.Circle) *form.Form {
	lw := getBorderLineWidth(a.Common.Border, a.BorderStyle)
	dashPattern := getBorderDashPattern(a.Common.Border, a.BorderStyle)
	col := a.Color

	rect := applyMargins(a.Rect, a.Margin)
	bbox := rect // TODO(voss): implement boundary effects

	if m := min(rect.Dx(), rect.Dy()); lw > m/2 {
		lw = m / 2
	}

	a.Rect = rect

	hasOutline := col != nil && col != annotation.Transparent && lw > 0
	hasFill := a.FillColor != nil
	if !(hasOutline || hasFill) {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    bbox,
		}
	}

	b := builder.New(content.Form, nil)

	b.SetExtGState(s.reset)
	if a.StrokingTransparency != 0 || a.NonStrokingTransparency != 0 {
		gs := &extgstate.ExtGState{
			Set:         state.StrokeAlpha | state.FillAlpha,
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
			b.SetLineCap(graphics.LineCapButt)
		}
	}
	if hasFill {
		b.SetFillColor(a.FillColor)
	}

	xMid := (rect.LLx + rect.URx) / 2
	yMid := (rect.LLy + rect.URy) / 2
	rx := (rect.Dx() - lw) / 2
	ry := (rect.Dy() - lw) / 2

	k := (math.Sqrt2 - 1.0) * 4 / 3 // control point offset for cubic Bezier approximation of a circle

	b.MoveTo(xMid+rx, yMid)
	b.CurveTo(xMid+rx, yMid+ry*k, xMid+rx*k, rect.URy-lw, xMid, rect.URy-lw)
	b.CurveTo(xMid-rx*k, rect.URy-lw, rect.LLx+lw, yMid+ry*k, rect.LLx+lw, yMid)
	b.CurveTo(rect.LLx+lw, yMid-ry*k, xMid-rx*k, rect.LLy+lw, xMid, rect.LLy+lw)
	b.CurveTo(xMid+rx*k, rect.LLy+lw, xMid+rx, yMid-ry*k, xMid+rx, yMid)
	b.ClosePath()

	switch {
	case hasOutline && hasFill:
		b.FillAndStroke()
	case hasFill:
		b.Fill()
	default: // hasOutline
		b.Stroke()
	}

	return &form.Form{
		Content: b.Stream,
		Res:     b.Resources,
		BBox:    bbox,
	}
}
