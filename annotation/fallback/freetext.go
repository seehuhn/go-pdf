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
	"fmt"
	"slices"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/text"
)

const (
	freeTextFontSize = 12
	freeTextPadding  = 2
)

func (s *Style) addFreeTextAppearance(a *annotation.FreeText) *form.Form {
	// extract information from the pre-set fields
	lw := a.BorderWidth()
	bgCol := a.Color

	calloutLine := a.CalloutLine
	hasCallout := a.Intent == annotation.FreeTextIntentCallout && len(calloutLine) >= 2

	be := a.BorderEffect
	isCloudy := be != nil && be.Style == "C" && be.Intensity > 0

	inner := applyMargins(a.Rect, a.Margin)

	outer := inner
	if hasCallout {
		reversed := slices.Clone(calloutLine)
		slices.Reverse(reversed)
		clBBox := openPolylineBBox(reversed, lw, annotation.LineEndingStyleNone, a.LineEndingStyle)
		outer.Extend(&clBBox)
	}

	// Set some relevant ignored fields: even if they are not used
	// for rendering, these may be useful in case the appearance stream
	// needs to be re-generated after edits.
	a.Border = &annotation.Border{Width: lw}
	a.BorderStyle = nil
	if !isCloudy {
		a.BorderEffect = nil
	}

	a.Align = annotation.TextAlignLeft
	a.DefaultStyle = ""

	// generate the appearance stream
	b := builder.New(content.Form, nil)

	b.SetExtGState(s.reset)

	// precompute cloud outline if applicable
	var co *cloudOutline
	if isCloudy && a.Intent != annotation.FreeTextIntentTypeWriter {
		x0 := inner.LLx + lw/2
		y0 := inner.LLy + lw/2
		x1 := inner.URx - lw/2
		y1 := inner.URy - lw/2
		verts := []vec.Vec2{
			{X: x0, Y: y0},
			{X: x1, Y: y0},
			{X: x1, Y: y1},
			{X: x0, Y: y1},
		}
		co = newCloudOutline(verts, be.Intensity, lw)
	}

	// draw border
	if a.Intent != annotation.FreeTextIntentTypeWriter {
		b.SetLineWidth(lw)
		b.SetStrokeColor(color.Black)
		if co != nil {
			if bgCol != nil {
				b.SetFillColor(bgCol)
				fillBBox := co.fillPath(b)
				b.Fill()
				outer.Extend(&fillBBox)
			}
			b.SetLineCap(graphics.LineCapRound)
			strokeBBox := co.strokePath(b)
			b.Stroke()
			outer.Extend(&strokeBBox)
			// expand for stroke width
			outer = pdf.Rectangle{
				LLx: outer.LLx - lw/2,
				LLy: outer.LLy - lw/2,
				URx: outer.URx + lw/2,
				URy: outer.URy + lw/2,
			}
		} else {
			if bgCol != nil {
				b.SetFillColor(bgCol)
				b.Rectangle(inner.LLx+lw/2, inner.LLy+lw/2, inner.Dx()-lw, inner.Dy()-lw)
				b.FillAndStroke()
			} else {
				b.Rectangle(inner.LLx+lw/2, inner.LLy+lw/2, inner.Dx()-lw, inner.Dy()-lw)
				b.Stroke()
			}
		}
	}

	if hasCallout {
		b.SetLineWidth(lw)
		b.SetStrokeColor(color.Black)
		reversed := slices.Clone(calloutLine)
		slices.Reverse(reversed)
		drawOpenPolyline(b, reversed, annotation.LineEndingStyleNone, a.LineEndingStyle, bgCol)
	}

	// render text content if present
	if a.Contents != "" {
		F := s.ContentFont

		clipLeft := inner.LLx + lw + freeTextPadding
		clipBottom := inner.LLy + lw + freeTextPadding
		clipWidth := inner.Dx() - 2*lw - 2*freeTextPadding
		clipHeight := inner.Dy() - 2*lw - 2*freeTextPadding

		lineHeight := pdf.Round(F.GetGeometry().Leading*freeTextFontSize, 2)

		b.PushGraphicsState()
		if co != nil {
			co.fillPath(b)
		} else {
			b.Rectangle(clipLeft, clipBottom, clipWidth, clipHeight)
		}
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

	// finalize outer rectangle
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

	// set DA to match the font/size/color used in the appearance stream
	fontName := b.FontName(s.ContentFont)
	a.DefaultAppearance = fmt.Sprintf("/%s %d Tf 0 g", fontName, freeTextFontSize)

	return &form.Form{
		Content: b.Stream,
		Res:     b.Resources,
		BBox:    outer,
	}
}
