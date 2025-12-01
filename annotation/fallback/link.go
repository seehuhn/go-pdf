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
	"seehuhn.de/go/pdf/internal/colconv"
)

func (s *Style) addLinkAppearance(a *annotation.Link) *form.Form {
	borderWidth := 0.0
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

	col := a.Color
	if col == nil {
		col = color.Black
	}

	bbox := a.Rect

	if borderWidth <= 0 || col == annotation.Transparent {
		return &form.Form{
			Draw: func(w *graphics.Writer) error { return nil },
			BBox: bbox,
		}
	}

	var draw func(w *graphics.Writer) error
	switch style {
	case "U": // underline
		draw = func(w *graphics.Writer) error {
			w.SetExtGState(s.reset)
			w.SetStrokeColor(col)
			w.SetLineWidth(borderWidth)
			w.MoveTo(pdf.Round(bbox.LLx, 2), pdf.Round(bbox.LLy+borderWidth/2, 2))
			w.LineTo(pdf.Round(bbox.URx, 2), pdf.Round(bbox.LLy+borderWidth/2, 2))
			w.Stroke()
			return nil
		}
	case "D": // dashed
		draw = func(w *graphics.Writer) error {
			w.SetExtGState(s.reset)
			w.SetStrokeColor(col)
			w.SetLineWidth(borderWidth)
			w.SetLineDash(dashPattern, 0)
			w.Rectangle(
				pdf.Round(bbox.LLx+borderWidth/2, 2),
				pdf.Round(bbox.LLy+borderWidth/2, 2),
				pdf.Round(bbox.URx-bbox.LLx-borderWidth, 2),
				pdf.Round(bbox.URy-bbox.LLy-borderWidth, 2))
			w.Stroke()
			return nil
		}
	case "B":
		dark, light := getDarkLightCol(col)
		draw = func(w *graphics.Writer) error {
			w.SetExtGState(s.reset)
			w.SetFillColor(dark)
			w.MoveTo(pdf.Round(bbox.LLx, 2), pdf.Round(bbox.LLy, 2))
			w.LineTo(pdf.Round(bbox.URx, 2), pdf.Round(bbox.LLy, 2))
			w.LineTo(pdf.Round(bbox.URx, 2), pdf.Round(bbox.URy, 2))
			w.LineTo(pdf.Round(bbox.URx-borderWidth, 2), pdf.Round(bbox.URy-borderWidth, 2))
			w.LineTo(pdf.Round(bbox.URx-borderWidth, 2), pdf.Round(bbox.LLy+borderWidth, 2))
			w.LineTo(pdf.Round(bbox.LLx+borderWidth, 2), pdf.Round(bbox.LLy+borderWidth, 2))
			w.Fill()

			w.SetFillColor(light)
			w.MoveTo(pdf.Round(bbox.LLx, 2), pdf.Round(bbox.LLy, 2))
			w.LineTo(pdf.Round(bbox.LLx+borderWidth, 2), pdf.Round(bbox.LLy+borderWidth, 2))
			w.LineTo(pdf.Round(bbox.LLx+borderWidth, 2), pdf.Round(bbox.URy-borderWidth, 2))
			w.LineTo(pdf.Round(bbox.URx-borderWidth, 2), pdf.Round(bbox.URy-borderWidth, 2))
			w.LineTo(pdf.Round(bbox.URx, 2), pdf.Round(bbox.URy, 2))
			w.LineTo(pdf.Round(bbox.LLx, 2), pdf.Round(bbox.URy, 2))
			w.Fill()
			return nil
		}
	case "I":
		dark, light := getDarkLightCol(col)
		draw = func(w *graphics.Writer) error {
			w.SetExtGState(s.reset)
			w.SetFillColor(light)
			w.MoveTo(pdf.Round(bbox.LLx, 2), pdf.Round(bbox.LLy, 2))
			w.LineTo(pdf.Round(bbox.URx, 2), pdf.Round(bbox.LLy, 2))
			w.LineTo(pdf.Round(bbox.URx, 2), pdf.Round(bbox.URy, 2))
			w.LineTo(pdf.Round(bbox.URx-borderWidth, 2), pdf.Round(bbox.URy-borderWidth, 2))
			w.LineTo(pdf.Round(bbox.URx-borderWidth, 2), pdf.Round(bbox.LLy+borderWidth, 2))
			w.LineTo(pdf.Round(bbox.LLx+borderWidth, 2), pdf.Round(bbox.LLy+borderWidth, 2))
			w.Fill()

			w.SetFillColor(dark)
			w.MoveTo(pdf.Round(bbox.LLx, 2), pdf.Round(bbox.LLy, 2))
			w.LineTo(pdf.Round(bbox.LLx+borderWidth, 2), pdf.Round(bbox.LLy+borderWidth, 2))
			w.LineTo(pdf.Round(bbox.LLx+borderWidth, 2), pdf.Round(bbox.URy-borderWidth, 2))
			w.LineTo(pdf.Round(bbox.URx-borderWidth, 2), pdf.Round(bbox.URy-borderWidth, 2))
			w.LineTo(pdf.Round(bbox.URx, 2), pdf.Round(bbox.URy, 2))
			w.LineTo(pdf.Round(bbox.LLx, 2), pdf.Round(bbox.URy, 2))
			w.Fill()
			return nil
		}
	default: // solid or unknown
		draw = func(w *graphics.Writer) error {
			w.SetExtGState(s.reset)
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
	return &form.Form{
		Draw: draw,
		BBox: bbox,
	}
}

func getDarkLightCol(col color.Color) (dark, light color.Color) {
	if col == annotation.Transparent {
		return col, col
	}

	components, _, _ := color.Operator(col)
	s := col.ColorSpace()
	switch s.Family() {
	case color.FamilyDeviceGray:
		L := colconv.DeviceGrayToL(components[0])
		darkL, lightL := getDarkLightL(L)
		dark = color.DeviceGray(pdf.Round(colconv.LToDeviceGray(darkL), 2))
		light = color.DeviceGray(pdf.Round(colconv.LToDeviceGray(lightL), 2))
	case color.FamilyDeviceRGB:
		L, a, b := colconv.DeviceRGBToLAB(components[0], components[1], components[2])
		darkL, lightL := getDarkLightL(L)
		r1, g1, b1 := colconv.LABToDeviceRGB(darkL, a, b)
		r2, g2, b2 := colconv.LABToDeviceRGB(lightL, a, b)
		dark = color.DeviceRGB{pdf.Round(r1, 2), pdf.Round(g1, 2), pdf.Round(b1, 2)}
		light = color.DeviceRGB{pdf.Round(r2, 2), pdf.Round(g2, 2), pdf.Round(b2, 2)}
	case color.FamilyDeviceCMYK:
		L, a, b := colconv.DeviceCMYKToLAB(components[0], components[1], components[2], components[3])
		darkL, lightL := getDarkLightL(L)
		c1, m1, y1, k1 := colconv.LABToDeviceCMYK(darkL, a, b)
		c2, m2, y2, k2 := colconv.LABToDeviceCMYK(lightL, a, b)
		dark = color.DeviceCMYK{pdf.Round(c1, 2), pdf.Round(m1, 2), pdf.Round(y1, 2), pdf.Round(k1, 2)}
		light = color.DeviceCMYK{pdf.Round(c2, 2), pdf.Round(m2, 2), pdf.Round(y2, 2), pdf.Round(k2, 2)}
	default:
		return col, col
	}
	return dark, light
}

func getDarkLightL(L float64) (dark, light float64) {
	if L < deltaMin {
		L = deltaMin
	} else if L > 100-deltaMin {
		L = 100 - deltaMin
	}
	delta := min(deltaMax, L, 100-L)
	return L - delta, L + delta
}

const (
	deltaMin = 10
	deltaMax = 20
)
