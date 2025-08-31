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
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addSquareAppearance(a *annotation.Square) *form.Form {
	lw := getSquareLineWidth(a)
	dashPattern := getSquareDashPattern(a)
	col := a.Color

	rect := calculateSquareRect(a)
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
		if hasOutline {
			w.SetLineWidth(lw)
			w.SetStrokeColor(col)
			if len(dashPattern) > 0 {
				w.SetLineDash(dashPattern, 0)
			}
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

// getSquareLineWidth returns the line width from BorderStyle, Border, or default
func getSquareLineWidth(a *annotation.Square) float64 {
	if a.BorderStyle != nil {
		return a.BorderStyle.Width
	}
	if a.Common.Border != nil {
		return a.Common.Border.Width
	}
	return 1.0 // default line width
}

// getSquareDashPattern returns the dash pattern from BorderStyle or Border
func getSquareDashPattern(a *annotation.Square) []float64 {
	if a.BorderStyle != nil && len(a.BorderStyle.DashArray) > 0 {
		return a.BorderStyle.DashArray
	}
	if a.Common.Border != nil && len(a.Common.Border.DashArray) > 0 {
		return a.Common.Border.DashArray
	}
	return nil
}

// calculateSquareRect returns the effective rectangle considering margins
func calculateSquareRect(a *annotation.Square) pdf.Rectangle {
	rect := a.Rect

	// apply margins (RD array) if specified
	if len(a.Margin) == 4 {
		// RD format: [left, bottom, right, top]
		rect.LLx += a.Margin[0] // left margin
		rect.LLy += a.Margin[1] // bottom margin
		rect.URx -= a.Margin[2] // right margin
		rect.URy -= a.Margin[3] // top margin
	}

	return rect
}

// calculateSquareBBox calculates the bounding box including border width
func calculateSquareBBox(rect pdf.Rectangle, lineWidth float64) pdf.Rectangle {
	// expand by half the line width in all directions
	halfWidth := lineWidth / 2
	return pdf.Rectangle{
		LLx: rect.LLx - halfWidth,
		LLy: rect.LLy - halfWidth,
		URx: rect.URx + halfWidth,
		URy: rect.URy + halfWidth,
	}
}
