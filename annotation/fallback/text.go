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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addTextAppearance(a *annotation.Text) *form.Form {
	bgCol := a.Color
	a.Border = nil

	// We don't generate dicts with different states.
	a.AppearanceState = ""

	bg := bgCol
	if bg == nil {
		bg = stickyYellow
	}

	var draw func(*graphics.Writer) error
	switch a.Icon {
	case annotation.TextIconComment:
		draw = func(w *graphics.Writer) error {
			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.SetFillColor(bg)
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.CloseFillAndStroke()

			w.TextBegin()
			w.TextSetFont(s.iconFont, 23)
			w.SetFillColor(color.DeviceGray(0.0))
			w.TextFirstLine(6, 2)
			w.TextSetHorizontalScaling(0.9)
			w.TextShow("“")
			w.TextEnd()

			return nil
		}

	case annotation.TextIconKey:
		draw = func(w *graphics.Writer) error {
			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.SetFillColor(bg)
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.CloseFillAndStroke()

			w.TextBegin()
			w.TextSetFont(s.iconFont, 25)
			w.SetFillColor(color.DeviceGray(0.0))
			w.TextFirstLine(5, 2)
			w.TextShow("*")
			w.TextEnd()

			return nil
		}

	case annotation.TextIconNote, "":
		draw = func(w *graphics.Writer) error {
			delta := 7.0

			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.SetFillColor(bg)
			w.MoveTo(23.5-delta, 0.25)
			w.LineTo(0.25, 0.25)
			w.LineTo(0.25, 23.5)
			w.LineTo(23.5, 23.5)
			w.LineTo(23.5, 0.25+delta)
			w.LineTo(23.5-delta, 0.25)
			w.LineTo(23.5-delta, 0.25+delta)
			w.LineTo(23.5, 0.25+delta)
			w.CloseFillAndStroke()

			w.SetLineWidth(1.5)
			w.SetStrokeColor(color.DeviceGray(0.5))
			for y := 19.; y > 6; y -= 3.5 {
				w.MoveTo(4, y)
				if y > 10 {
					w.LineTo(17, y)
				} else {
					w.LineTo(12, y)
				}
			}
			w.Stroke()

			return nil
		}

	case annotation.TextIconHelp:
		draw = func(w *graphics.Writer) error {
			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.SetFillColor(bg)
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.CloseFillAndStroke()

			w.TextBegin()
			w.TextSetFont(s.iconFont, 23)
			w.SetFillColor(color.DeviceGray(0.0))
			w.TextFirstLine(6, 4)
			w.TextShow("?")
			w.TextEnd()

			return nil
		}

	case annotation.TextIconNewParagraph:
		draw = func(w *graphics.Writer) error {
			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.SetFillColor(bg)
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.CloseFillAndStroke()

			w.SetStrokeColor(color.DeviceGray(0.7))
			w.SetLineWidth(1.5)
			w.MoveTo(4, 19)
			w.LineTo(17, 19)
			w.MoveTo(4, 15.5)
			w.LineTo(12, 15.5)
			w.MoveTo(4, 5)
			w.LineTo(17, 5)
			w.Stroke()

			m := (15.5 + 5) / 2

			w.SetStrokeColor(color.Black)
			w.SetFillColor(color.Black)
			w.SetLineWidth(2)
			w.MoveTo(17.5-0.75, 15.5)
			w.LineTo(17.5-0.75, m)
			w.LineTo(5, m)
			w.Stroke()
			w.MoveTo(3, m)
			w.LineTo(7, m+3)
			w.LineTo(7, m-3)
			w.Fill()

			return nil
		}

	case annotation.TextIconParagraph:
		draw = func(w *graphics.Writer) error {
			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.SetFillColor(bg)
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.CloseFillAndStroke()

			w.TextBegin()
			w.TextSetFont(s.iconFont, 16)
			w.SetFillColor(color.DeviceGray(0.0))
			w.TextFirstLine(6, 8)
			w.TextSetHorizontalScaling(1.4)
			w.TextShow("¶")
			w.TextEnd()

			return nil
		}

	case annotation.TextIconInsert:
		draw = func(w *graphics.Writer) error {
			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.SetFillColor(bg)
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.CloseFillAndStroke()

			w.TextBegin()
			w.TextSetFont(s.iconFont, 16)
			w.SetFillColor(color.DeviceGray(0.0))
			w.TextFirstLine(5.5, 4)
			w.TextSetHorizontalScaling(1.4)
			w.TextShow("^")
			w.TextEnd()

			return nil
		}

	default:
		draw = func(w *graphics.Writer) error {
			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.SetFillColor(bg)
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.CloseFillAndStroke()

			return nil
		}
	}

	return &form.Form{
		Draw: draw,
		BBox: pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
	}
}

var (
	stickyYellow = color.DeviceRGB(0.98, 0.96, 0.75)
)
