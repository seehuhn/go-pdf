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
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addTextAppearance(a *annotation.Text) *form.Form {
	bgCol := a.Color
	if bgCol == nil {
		bgCol = stickyYellow
	}

	b := builder.New(content.Form, nil)

	switch a.Icon {
	case annotation.TextIconComment:
		b.SetExtGState(s.reset)
		b.SetLineWidth(0.5)
		b.SetStrokeColor(color.DeviceGray(0.2))
		if bgCol != annotation.Transparent {
			b.SetFillColor(bgCol)
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseFillAndStroke()
		} else {
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseAndStroke()
		}

		b.TextBegin()
		b.TextSetFont(s.iconFont, 23)
		b.TextSetRise(0)
		b.TextSetHorizontalScaling(1)
		b.SetFillColor(color.DeviceGray(0.0))
		b.TextFirstLine(6, 2)
		b.TextSetHorizontalScaling(0.9)
		b.TextShow("\u201C")
		b.TextEnd()

	case annotation.TextIconKey:
		b.SetExtGState(s.reset)
		b.SetLineWidth(0.5)
		b.SetStrokeColor(color.DeviceGray(0.2))
		if bgCol != annotation.Transparent {
			b.SetFillColor(bgCol)
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseFillAndStroke()
		} else {
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseAndStroke()
		}

		b.TextBegin()
		b.TextSetFont(s.iconFont, 25)
		b.TextSetRise(0)
		b.TextSetHorizontalScaling(1)
		b.SetFillColor(color.DeviceGray(0.0))
		b.TextFirstLine(5, 2)
		b.TextShow("*")
		b.TextEnd()

	case annotation.TextIconNote, "":
		delta := 7.0

		b.SetExtGState(s.reset)
		b.SetLineWidth(0.5)
		b.SetStrokeColor(color.DeviceGray(0.2))
		if bgCol != annotation.Transparent {
			b.SetFillColor(bgCol)
		}
		b.MoveTo(23.5-delta, 0.25)
		b.LineTo(0.25, 0.25)
		b.LineTo(0.25, 23.5)
		b.LineTo(23.5, 23.5)
		b.LineTo(23.5, 0.25+delta)
		b.LineTo(23.5-delta, 0.25)
		b.LineTo(23.5-delta, 0.25+delta)
		b.LineTo(23.5, 0.25+delta)
		if bgCol != annotation.Transparent {
			b.CloseFillAndStroke()
		} else {
			b.CloseAndStroke()
		}

		b.SetLineWidth(1.5)
		b.SetStrokeColor(color.DeviceGray(0.5))
		for y := 19.; y > 6; y -= 3.5 {
			b.MoveTo(4, y)
			if y > 10 {
				b.LineTo(17, y)
			} else {
				b.LineTo(12, y)
			}
		}
		b.Stroke()

	case annotation.TextIconHelp:
		b.SetExtGState(s.reset)

		b.SetLineWidth(0.5)
		b.SetStrokeColor(color.DeviceGray(0.2))
		if bgCol != annotation.Transparent {
			b.SetFillColor(bgCol)
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseFillAndStroke()
		} else {
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseAndStroke()
		}

		b.TextBegin()
		b.TextSetFont(s.iconFont, 23)
		b.TextSetRise(0)
		b.TextSetHorizontalScaling(1)
		b.SetFillColor(color.DeviceGray(0.0))
		b.TextFirstLine(6, 4)
		b.TextShow("?")
		b.TextEnd()

	case annotation.TextIconNewParagraph:
		b.SetExtGState(s.reset)

		b.SetLineWidth(0.5)
		b.SetStrokeColor(color.DeviceGray(0.2))
		if bgCol != annotation.Transparent {
			b.SetFillColor(bgCol)
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseFillAndStroke()
		} else {
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseAndStroke()
		}

		b.SetStrokeColor(color.DeviceGray(0.7))
		b.SetLineWidth(1.5)
		b.MoveTo(4, 19)
		b.LineTo(17, 19)
		b.MoveTo(4, 15.5)
		b.LineTo(12, 15.5)
		b.MoveTo(4, 5)
		b.LineTo(17, 5)
		b.Stroke()

		m := (15.5 + 5) / 2

		b.SetStrokeColor(color.Black)
		b.SetFillColor(color.Black)
		b.SetLineWidth(1.8)
		b.MoveTo(17.5-0.75, 15.5)
		b.LineTo(17.5-0.75, m)
		b.LineTo(5, m)
		b.Stroke()
		b.MoveTo(3, m)
		b.LineTo(7, m+2.8)
		b.LineTo(7, m-2.8)
		b.Fill()

	case annotation.TextIconParagraph:
		b.SetExtGState(s.reset)

		b.SetLineWidth(0.5)
		b.SetStrokeColor(color.DeviceGray(0.2))
		if bgCol != annotation.Transparent {
			b.SetFillColor(bgCol)
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseFillAndStroke()
		} else {
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseAndStroke()
		}

		b.TextBegin()
		b.TextSetFont(s.iconFont, 16)
		b.TextSetRise(0)
		b.TextSetHorizontalScaling(1)
		b.SetFillColor(color.DeviceGray(0.0))
		b.TextFirstLine(6, 8)
		b.TextSetHorizontalScaling(1.4)
		b.TextShow("¶")
		b.TextEnd()

	case annotation.TextIconInsert:
		b.SetExtGState(s.reset)

		b.SetLineWidth(0.5)
		b.SetStrokeColor(color.DeviceGray(0.2))
		if bgCol != annotation.Transparent {
			b.SetFillColor(bgCol)
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseFillAndStroke()
		} else {
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseAndStroke()
		}

		b.TextBegin()
		b.TextSetFont(s.iconFont, 16)
		b.TextSetRise(0)
		b.TextSetHorizontalScaling(1)
		b.SetFillColor(color.DeviceGray(0.0))
		b.TextFirstLine(5.5, 4)
		b.TextSetHorizontalScaling(1.4)
		b.TextShow("^")
		b.TextEnd()

	default:
		b.SetExtGState(s.reset)

		b.SetLineWidth(0.5)
		b.SetStrokeColor(color.DeviceGray(0.2))
		if bgCol != annotation.Transparent {
			b.SetFillColor(bgCol)
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseFillAndStroke()
		} else {
			b.Rectangle(0.25, 0.25, 23.5, 23.5)
			b.CloseAndStroke()
		}
	}

	return &form.Form{
		Content: b.Stream,
		Res:     b.Resources,
		BBox:    pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
	}
}

var (
	stickyYellow = color.DeviceRGB{0.98, 0.96, 0.75}
)
