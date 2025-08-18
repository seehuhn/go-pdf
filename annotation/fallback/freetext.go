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
	"seehuhn.de/go/pdf/graphics/text"
)

const (
	freeTextFontSize = 12
	freeTextPadding  = 2
)

func (s *Style) addFreeTextAppearance(a *annotation.FreeText) {
	// TODO(voss): implement border effects

	// extract information from the pre-set fields
	lw := a.BorderWidth()
	bgCol := a.Color

	calloutLine := a.CalloutLine
	if k := len(calloutLine); k%2 != 0 {
		calloutLine = calloutLine[:k-1] // ignore last value if odd
	}
	hasCallout := a.Intent == annotation.FreeTextIntentCallout && len(calloutLine) >= 4

	inner := a.Rect
	if len(a.Margin) >= 4 {
		inner.LLx += a.Margin[0]
		inner.LLy += a.Margin[1]
		inner.URx -= a.Margin[2]
		inner.URy -= a.Margin[3]
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
		leInfo := LineEndingInfo{
			AtX:  calloutLine[0],
			AtY:  calloutLine[1],
			DirX: calloutLine[0] - calloutLine[2],
			DirY: calloutLine[1] - calloutLine[3],
		}
		LineEndingBBox(&outer, a.LineEndingStyle, leInfo, lw)
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
	// We don't generate dicts with different states.
	a.AppearanceState = ""

	outer.Round(1)
	a.Rect = outer
	if inner.NearlyEqual(&outer, 0.01) {
		a.Margin = nil
	} else {
		a.Margin = []float64{
			pdf.Round(inner.LLx-outer.LLx, 4),
			pdf.Round(inner.LLy-outer.LLy, 4),
			pdf.Round(inner.URx-outer.URx, 4),
			pdf.Round(inner.URy-outer.URy, 4),
		}
	}

	// generate the appearance stream
	draw := func(w *graphics.Writer) error {
		if a.Intent != annotation.FreeTextIntentTypeWriter {
			w.SetLineWidth(lw)
			w.SetStrokeColor(color.Black)
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
			w.SetStrokeColor(color.Black)
			k := len(calloutLine)
			w.MoveTo(calloutLine[k-2], calloutLine[k-1])
			for i := k - 4; i >= 2; i -= 2 {
				w.LineTo(calloutLine[i], calloutLine[i+1])
			}
			leInfo := LineEndingInfo{
				FillColor: bgCol,
				AtX:       calloutLine[0],
				AtY:       calloutLine[1],
				DirX:      calloutLine[0] - calloutLine[2],
				DirY:      calloutLine[1] - calloutLine[3],
			}
			DrawLineEnding(w, a.LineEndingStyle, leInfo)
		}

		// render text content if present
		if a.Contents != "" {
			F := s.contentFont

			clipLeft := inner.LLx + lw + freeTextPadding
			clipBottom := inner.LLy + lw + freeTextPadding
			clipWidth := inner.Dx() - 2*lw - 2*freeTextPadding
			clipHeight := inner.Dy() - 2*lw - 2*freeTextPadding

			lineHeight := pdf.Round(F.GetGeometry().Leading*freeTextFontSize, 2)

			w.PushGraphicsState()
			w.Rectangle(clipLeft, clipBottom, clipWidth, clipHeight)
			w.ClipNonZero()
			w.EndPath()

			w.TextBegin()
			w.TextSetFont(F, freeTextFontSize)
			w.SetFillColor(color.Black)
			wrapper := text.Wrap(clipWidth, a.Contents)
			yPos := inner.URy - lw - freeTextPadding - freeTextFontSize
			lineNo := 0
			for line := range wrapper.Lines(F, freeTextFontSize) {
				switch lineNo {
				case 0:
					w.TextFirstLine(clipLeft, yPos)
				case 1:
					w.TextSecondLine(0, -lineHeight)
				default:
					w.TextNextLine()
				}

				switch a.Align {
				case annotation.FreeTextAlignCenter:
					line.Align(clipWidth, 0.5)
				case annotation.FreeTextAlignRight:
					line.Align(clipWidth, 1.0)
				default:
					// no adjustment needed for left alignment
				}
				w.TextShowGlyphs(line)

				yPos -= lineHeight
				lineNo++
			}
			w.TextEnd()

			w.PopGraphicsState()
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
