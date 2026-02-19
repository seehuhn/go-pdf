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
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addCaretAppearance(a *annotation.Caret) *form.Form {
	col := a.Color
	if col == nil {
		return &form.Form{
			Content: nil,
			Res:     &content.Resources{},
			BBox:    a.Rect,
		}
	}

	inner := applyMargins(a.Rect, a.Margin)

	// when Sy=P, expand Rect and Margin downward to make room for the pilcrow
	if a.Symbol == "P" {
		fontSize := inner.Dx() * 0.6
		if fontSize < 3 {
			fontSize = 3
		}
		extra := fontSize * 0.85
		a.Rect.LLy -= extra
		if len(a.Margin) == 4 {
			a.Margin = []float64{a.Margin[0], a.Margin[1] + extra, a.Margin[2], a.Margin[3]}
		} else {
			a.Margin = []float64{0, extra, 0, 0}
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

	x0 := inner.LLx
	y0 := inner.LLy
	w := inner.Dx()
	h := inner.Dy()

	// filled caret: flat base with straight sides curving to a narrow tip
	b.SetFillColor(col)
	b.MoveTo(x0, y0)
	b.LineTo(x0+w, y0)
	b.LineTo(x0+w, y0+0.1*h)
	b.CurveTo(
		x0+0.52*w, y0+0.1*h,
		x0+0.52*w, y0+0.1*h,
		x0+0.52*w, y0+h,
	)
	b.LineTo(x0+0.48*w, y0+h)
	b.CurveTo(
		x0+0.48*w, y0+0.1*h,
		x0+0.48*w, y0+0.1*h,
		x0, y0+0.1*h,
	)
	b.ClosePath()
	b.Fill()

	// pilcrow below the caret base
	if a.Symbol == "P" {
		fontSize := w * 0.6
		if fontSize < 3 {
			fontSize = 3
		}
		cx := (inner.LLx + inner.URx) / 2
		b.TextBegin()
		b.TextSetFont(s.iconFont, fontSize)
		b.TextSetHorizontalScaling(1)
		b.TextFirstLine(cx-fontSize*0.25, y0-fontSize*0.85)
		b.TextShow("\u00B6")
		b.TextEnd()
	}

	return &form.Form{
		Content: b.Stream,
		Res:     b.Resources,
		BBox:    a.Rect,
	}
}
