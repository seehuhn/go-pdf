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
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addLinkAppearance(a *annotation.Link) {
	borderWidth := 1.0
	var dashPattern []float64
	style := pdf.Name("S")
	if a.Common.Border != nil {
		borderWidth = a.Common.Border.Width
		dashPattern = a.Common.Border.DashArray
		if len(dashPattern) > 0 {
			style = "D"
		}
	}
	if a.BorderStyle != nil {
		borderWidth = a.BorderStyle.Width
		style = a.BorderStyle.Style
		if style == "D" {
			dashPattern = a.BorderStyle.DashArray
			if len(dashPattern) == 0 {
				dashPattern = []float64{3}
			}
		}
	}

	if borderWidth <= 0 {
		a.Appearance = nil
		a.AppearanceState = ""
		return
	}

	col := a.Color
	if col == nil {
		col = color.Black
	}
	bbox := a.Rect

	var draw func(w *graphics.Writer) error
	switch style {
	case "U": // underline
		draw = func(w *graphics.Writer) error {
			w.SetStrokeColor(col)
			w.SetLineWidth(borderWidth)
			w.MoveTo(pdf.Round(bbox.LLx, 2), pdf.Round(bbox.LLy+borderWidth/2, 2))
			w.LineTo(pdf.Round(bbox.URx, 2), pdf.Round(bbox.LLy+borderWidth/2, 2))
			w.Stroke()
			return nil
		}
	case "D": // dashed
		draw = func(w *graphics.Writer) error {
			w.SetStrokeColor(col)
			w.SetLineDash(dashPattern, 0)
			w.SetLineWidth(borderWidth)
			w.Rectangle(
				pdf.Round(bbox.LLx+borderWidth/2, 2),
				pdf.Round(bbox.LLy+borderWidth/2, 2),
				pdf.Round(bbox.URx-bbox.LLx-borderWidth, 2),
				pdf.Round(bbox.URy-bbox.LLy-borderWidth, 2))
			w.Stroke()
			return nil
		}
	// case "B": // beveled // TODO(voss): implement
	// case "I": // inset // TODO(voss): implement
	default: // solid or unknown
		draw = func(w *graphics.Writer) error {
			w.SetStrokeColor(col)
			w.SetLineWidth(borderWidth)
			w.Rectangle(
				pdf.Round(bbox.LLx+borderWidth/2, 2),
				pdf.Round(bbox.LLy+borderWidth/2, 2),
				pdf.Round(bbox.URx-bbox.LLx-borderWidth, 2),
				pdf.Round(bbox.URy-bbox.LLy-borderWidth, 2))
			w.Stroke()
			return nil
		}
	}

	// create appearance stream
	xObj := &form.Form{
		Draw: draw,
		BBox: bbox,
	}
	a.Appearance = &appearance.Dict{
		Normal: xObj,
	}
	a.AppearanceState = "N"

	// set style fields to what we have used
	a.Color = col
}
