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
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

// addWidgetAppearance generates a fallback appearance for a widget annotation.
//
// This is a placeholder: it fills the field area with a light panel and a
// hairline border. It does not yet render the field's value or use the
// appearance characteristics (MK). A complete implementation will draw the
// field according to its type and value.
func (s *Style) addWidgetAppearance(a *annotation.Widget) *form.Form {
	// hairline border, inset by half the line width so it stays inside the Rect
	const lw = 0.5

	rect := a.Rect
	w := rect.Dx()
	h := rect.Dy()
	if w < lw || h < lw {
		return &form.Form{Content: nil, Res: &content.Resources{}, BBox: rect}
	}

	b := builder.New(content.Form, nil, s.Version)
	b.SetExtGState(s.reset)

	// panel
	b.SetFillColor(quireSlate1)
	b.Rectangle(rect.LLx, rect.LLy, w, h)
	b.Fill()

	b.SetLineWidth(lw)
	b.SetStrokeColor(quireSlate3)
	b.Rectangle(rect.LLx+lw/2, rect.LLy+lw/2, w-lw, h-lw)
	b.Stroke()

	return &form.Form{
		Content: builder.Must(b.Harvest()),
		Res:     b.Resources,
		BBox:    rect,
		Matrix:  matrix.Identity,
	}
}
