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
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/form"
)

func (s *Style) addFreeTextAppearance(a *annotation.FreeText, bgCol color.Color) {
	// a.Color = nil
	// a.Border = nil
	// a.BorderStyle = nil
	// a.BorderEffect = nil
	// a.DefaultAppearance = ""
	// a.Align = annotation.FreeTextAlignLeft
	// a.DefaultStyle = ""

	// TODO(voss): is the next one correct?
	// a.LineEndingStyle = annotation.LineEndingStyleNone

	// We don't generate dicts with different states.
	a.AppearanceState = ""

	bg := bgCol
	if bg == nil {
		bg = stickyYellow
	}

	draw := func(w *graphics.Writer) error {
		lw := 0.5
		w.SetLineWidth(lw)
		w.SetStrokeColor(color.DeviceGray(0.2))
		w.SetFillColor(bg)
		w.Rectangle(a.Rect.LLx+lw/2, a.Rect.LLy+lw/2, a.Rect.Dx()-lw, a.Rect.Dy()-lw)
		w.CloseFillAndStroke()

		return nil
	}

	xObj := &form.Form{
		Draw: draw,
		BBox: a.Rect,
	}
	res := &appearance.Dict{
		Normal: xObj,
	}
	a.Appearance = res
}
