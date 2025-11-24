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
	"seehuhn.de/go/pdf/graphics/text"
)

const (
	freeTextFontSize = 12
	freeTextPadding  = 2
)

func (s *Style) addFreeTextAppearance(a *annotation.FreeText) *form.Form {
	// TODO(voss): implement border effects

	// extract information from the pre-set fields
	lw := a.BorderWidth()
	bgCol := a.Color

	calloutLine := a.CalloutLine
	hasCallout := a.Intent == annotation.FreeTextIntentCallout && len(calloutLine) >= 2

	inner := applyMargins(a.Rect, a.Margin)

	outer := inner
	if hasCallout {
		for _, point := range calloutLine {
			joint := pdf.Rectangle{
				LLx: point.X - lw/2,
				LLy: point.Y - lw/2,
				URx: point.X + lw/2,
				URy: point.Y + lw/2,
			}
			outer.Extend(&joint)
		}
		leInfo := lineEndingInfo{
			At:  calloutLine[0],
			Dir: calloutLine[0].Sub(calloutLine[1]),
		}
		lineEndingBBox(&outer, a.LineEndingStyle, leInfo, lw)
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
	a.Align = annotation.TextAlignLeft
	a.DefaultStyle = ""

	outer.IRound(1)
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
	b := graphics.NewContentStreamBuilder()
	b.SetExtGState(s.reset)

	if a.Intent != annotation.FreeTextIntentTypeWriter {
		b.SetLineWidth(lw)
		b.SetStrokeColor(color.Black)
		if bgCol != nil {
			b.SetFillColor(bgCol)
			b.Rectangle(inner.LLx+lw/2, inner.LLy+lw/2, inner.Dx()-lw, inner.Dy()-lw)
			b.FillAndStroke()
		} else {
			b.Rectangle(inner.LLx+lw/2, inner.LLy+lw/2, inner.Dx()-lw, inner.Dy()-lw)
			b.Stroke()
		}
	}

	if hasCallout {
		b.SetLineWidth(lw)
		b.SetStrokeColor(color.Black)
		k := len(calloutLine)
		lastPoint := calloutLine[k-1]
		b.MoveTo(lastPoint.X, lastPoint.Y)
		for i := k - 2; i >= 1; i-- {
			b.LineTo(calloutLine[i].X, calloutLine[i].Y)
		}
		leInfo := lineEndingInfo{
			FillColor: bgCol,
			At:        calloutLine[0],
			Dir:       calloutLine[0].Sub(calloutLine[1]),
		}
		drawLineEnding(b, a.LineEndingStyle, leInfo)
	}

	// render text content if present
	if a.Contents != "" {
		F := s.contentFont

		clipLeft := inner.LLx + lw + freeTextPadding
		clipBottom := inner.LLy + lw + freeTextPadding
		clipWidth := inner.Dx() - 2*lw - 2*freeTextPadding
		clipHeight := inner.Dy() - 2*lw - 2*freeTextPadding

		lineHeight := pdf.Round(F.GetGeometry().Leading*freeTextFontSize, 2)

		b.PushGraphicsState()
		b.Rectangle(clipLeft, clipBottom, clipWidth, clipHeight)
		b.ClipNonZero()
		b.EndPath()

		b.TextBegin()
		b.TextSetFont(F, freeTextFontSize)
		b.SetFillColor(color.Black)
		b.TextSetHorizontalScaling(1)
		b.TextSetRise(0)
		wrapper := text.Wrap(clipWidth, a.Contents)
		yPos := inner.URy - lw - freeTextPadding - freeTextFontSize
		lineNo := 0
		for line := range wrapper.Lines(F, freeTextFontSize) {
			switch lineNo {
			case 0:
				b.TextFirstLine(clipLeft, yPos)
			case 1:
				b.TextSecondLine(0, -lineHeight)
			default:
				b.TextNextLine()
			}

			switch a.Align {
			case annotation.TextAlignCenter:
				line.Align(clipWidth, 0.5)
			case annotation.TextAlignRight:
				line.Align(clipWidth, 1.0)
			default:
				// no adjustment needed for left alignment
			}
			b.TextShowGlyphs(line)

			yPos -= lineHeight
			lineNo++
		}
		b.TextEnd()

		b.PopGraphicsState()
	}

	return &form.Form{
		Content: b.Build(),
		BBox:    outer,
	}
}
