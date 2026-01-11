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
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/state"
)

func (s *Style) addSquareAppearance(a *annotation.Square) *form.Form {
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
		b.SetLineDash(dashPattern, 0)
	}
	if hasFill {
		b.SetFillColor(a.FillColor)
	}

	b.Rectangle(rect.LLx+lw/2, rect.LLy+lw/2, rect.Dx()-lw, rect.Dy()-lw)

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
