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
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/form"
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
			Draw: func(w *graphics.Writer) error { return nil },
			BBox: bbox,
		}
	}

	draw := func(w *graphics.Writer) error {
		w.SetExtGState(s.reset)
		if a.StrokingTransparency != 0 || a.NonStrokingTransparency != 0 {
			gs := &graphics.ExtGState{
				Set:         graphics.StateStrokeAlpha | graphics.StateFillAlpha,
				StrokeAlpha: 1 - a.StrokingTransparency,
				FillAlpha:   1 - a.NonStrokingTransparency,
				SingleUse:   true,
			}
			w.SetExtGState(gs)
		}

		if hasOutline {
			w.SetLineWidth(lw)
			w.SetStrokeColor(col)
			w.SetLineDash(dashPattern, 0)
		}
		if hasFill {
			w.SetFillColor(a.FillColor)
		}

		w.Rectangle(rect.LLx+lw/2, rect.LLy+lw/2, rect.Dx()-lw, rect.Dy()-lw)

		switch {
		case hasOutline && hasFill:
			w.FillAndStroke()
		case hasFill:
			w.Fill()
		default: // hasOutline
			w.Stroke()
		}

		return nil
	}

	return &form.Form{
		Draw: draw,
		BBox: bbox,
	}
}
