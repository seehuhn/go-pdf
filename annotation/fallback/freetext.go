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

func (s *Style) addFreeTextAppearance(a *annotation.FreeText) {
	// extract information from the pre-set fields
	lw := a.BorderWidth()
	bgCol := a.Color

	calloutLine := a.CalloutLine
	if k := len(calloutLine); k%2 != 0 {
		calloutLine = calloutLine[:k-1] // ignore last point if odd
	}
	hasCallout := a.Intent == annotation.FreeTextIntentCallout && len(calloutLine) >= 4
	var le LineEnding
	if hasCallout {
		le = NewLineEnding(
			calloutLine[0], calloutLine[1],
			calloutLine[2], calloutLine[3],
			lw, a.LineEndingStyle,
		)
	}

	inner := a.Rect
	if len(a.Margin) >= 4 {
		inner.LLx += a.Margin[0]
		inner.URy -= a.Margin[1]
		inner.URx -= a.Margin[2]
		inner.LLy += a.Margin[3]
	}

	outer := inner
	if hasCallout {
		for i := 0; i+1 < len(calloutLine); i += 2 {
			x, y := calloutLine[i], calloutLine[i+1]
			joint := pdf.Rectangle{
				LLx: x - lw/2,
				LLy: y - lw/2,
				URx: x + lw/2,
				URy: y + lw/2,
			}
			outer.Extend(&joint)
		}
		le.Enlarge(&outer)
	}

	// Set some relevant ignored fields: even if they are not used
	// for rendering, these may be useful in case the appearance stream
	// needs to be re-generated after edits.
	a.Border = &annotation.Border{Width: lw}
	a.BorderStyle = nil
	a.BorderEffect = nil

	// zero out the remaining ignored fields
	// TODO(voss): is this the right thing to do?
	a.DefaultAppearance = ""
	a.Align = annotation.FreeTextAlignLeft
	a.DefaultStyle = ""
	a.LineEndingStyle = annotation.LineEndingStyleNone
	// We don't generate dicts with different states.
	a.AppearanceState = ""

	a.Rect = outer
	if inner.NearlyEqual(&outer, 0.01) {
		a.Margin = nil
	} else {
		a.Margin = []float64{
			inner.LLx - outer.LLx,
			outer.URy - inner.URy,
			outer.URx - inner.URx,
			inner.LLy - outer.LLy,
		}
	}

	// generate the appearance stream
	draw := func(w *graphics.Writer) error {
		if a.Intent != annotation.FreeTextIntentTypeWriter {
			w.SetLineWidth(lw)
			w.SetStrokeColor(color.DeviceGray(0))
			if bgCol != nil {
				w.SetFillColor(bgCol)
				w.Rectangle(inner.LLx+lw/2, inner.LLy+lw/2, inner.Dx()-lw, inner.Dy()-lw)
				w.FillAndStroke()
			} else {
				w.Rectangle(inner.LLx+lw/2, inner.LLy+lw/2, inner.Dx()-lw, inner.Dy()-lw)
				w.Stroke()
			}
		}

		if hasCallout {
			w.SetLineWidth(lw)
			w.SetStrokeColor(color.DeviceGray(0))
			k := len(calloutLine)
			w.MoveTo(calloutLine[k-2], calloutLine[k-1])
			for i := k - 4; i >= 2; i -= 2 {
				w.LineTo(calloutLine[i], calloutLine[i+1])
			}
			le.Draw(w, bgCol)
		}

		return nil
	}

	xObj := &form.Form{
		Draw: draw,
		BBox: outer,
	}
	res := &appearance.Dict{
		Normal: xObj,
	}
	a.Appearance = res
}
