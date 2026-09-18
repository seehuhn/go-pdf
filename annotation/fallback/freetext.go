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
	"strconv"

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/text"
)

const (
	freeTextFontSize = 12
	freeTextPadding  = 2
)

// freeTextFont returns the font a FreeText annotation's text is drawn in:
// Courier for the Typewriter intent, and the shared content font for
// everything else.
func (g *Generator) freeTextFont(a *annotation.FreeText) font.Layouter {
	if a.Intent == annotation.FreeTextIntentTypeWriter {
		return g.typewriter()
	}
	return g.ContentFont()
}

func (g *Generator) addFreeTextAppearance(a *annotation.FreeText) (*form.Form, error) {
	// extract information from the pre-set fields
	lw := annotation.EffectiveBorderWidth(a)
	bgCol := paint(a.Color)

	calloutLine := a.CalloutLine
	hasCallout := a.Intent == annotation.FreeTextIntentCallout && len(calloutLine) >= 2

	be := a.BorderEffect
	isCloudy := be != nil && be.Style == "C" && be.Intensity > 0

	// the annotation's own default appearance says what colour its text is
	// drawn in, and what size; the generator's ink stands in where it names
	// no colour, and the default size where it names none or an invalid one
	size, textCol := parseDA(a.DefaultAppearance)
	if size <= 0 {
		size = freeTextFontSize
	}

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
	// needs to be re-generated after edits.  A nil Border is the "no border"
	// value; a zero width cannot be expressed as a border array.
	if lw > 0 {
		a.Border = &annotation.Border{Width: lw}
	} else {
		a.Border = nil
	}
	a.BorderStyle = nil
	if !isCloudy {
		a.BorderEffect = nil
	}

	a.DefaultStyle = ""

	// generate the appearance stream
	b := builder.New(content.Form, nil, g.version)

	g.reset(b)

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

	// draw border and background
	if a.Intent != annotation.FreeTextIntentTypeWriter {
		// a border width of 0 asks for no border
		hasBorder := lw > 0
		if hasBorder {
			b.SetLineWidth(lw)
			b.SetStrokeColor(quireInk)
		}
		if co != nil {
			if bgCol != nil {
				b.SetFillColor(bgCol)
				fillBBox := co.fillPath(b)
				b.Fill()
				outer.Extend(&fillBBox)
			}
			if hasBorder {
				b.SetLineCap(graphics.LineCapRound)
				strokeBBox := co.strokePath(b)
				b.Stroke()
				outer.Extend(&strokeBBox)
				// expand for stroke width
				outer = outer.Grow(lw / 2)
			}
		} else if bgCol != nil || hasBorder {
			if bgCol != nil {
				b.SetFillColor(bgCol)
			}
			b.Rectangle(inner.LLx+lw/2, inner.LLy+lw/2, inner.Dx()-lw, inner.Dy()-lw)
			switch {
			case bgCol != nil && hasBorder:
				b.FillAndStroke()
			case bgCol != nil:
				b.Fill()
			default:
				b.Stroke()
			}
		}
	}

	if hasCallout {
		b.SetLineWidth(lw)
		b.SetStrokeColor(quireInk)
		reversed := slices.Clone(calloutLine)
		slices.Reverse(reversed)
		if co != nil {
			reversed[0] = co.trimToCloud(reversed[1], reversed[0])
		}
		drawOpenPolyline(b, reversed, annotation.LineEndingStyleNone, a.LineEndingStyle, bgCol)
	}

	// render text content if present
	if a.Contents != "" {
		F := g.freeTextFont(a)

		clipLeft := inner.LLx + lw + freeTextPadding
		clipBottom := inner.LLy + lw + freeTextPadding
		clipWidth := inner.Dx() - 2*lw - 2*freeTextPadding
		clipHeight := inner.Dy() - 2*lw - 2*freeTextPadding

		lineHeight := pdf.Round(F.GetGeometry().Leading*size, 2)

		b.PushGraphicsState()
		if co != nil {
			co.fillPath(b)
		} else {
			b.Rectangle(clipLeft, clipBottom, clipWidth, clipHeight)
		}
		b.ClipNonZero()
		b.EndPath()

		b.TextBegin()
		b.TextSetFont(F, size)
		b.SetFillColor(textCol)
		b.TextSetHorizontalScaling(1)
		b.TextSetRise(0)
		wrapper := text.WrapWith(g.breaker(), clipWidth, a.Contents)
		// The first baseline sits one ascent below the top of the content
		// area.  This leaves the rest of the leading below the last line,
		// where the descenders need it, instead of above the first line.
		yPos := inner.URy - lw - freeTextPadding - pdf.Round(F.GetGeometry().Ascent*size, 2)
		lineNo := 0
		for line := range wrapper.Lines(F, size) {
			switch lineNo {
			case 0:
				b.TextFirstLine(clipLeft, yPos)
			case 1:
				b.TextSecondLine(0, -lineHeight)
			default:
				b.TextNextLine()
			}

			switch a.Align {
			case pdf.TextAlignCenter:
				line.Align(clipWidth, 0.5)
			case pdf.TextAlignRight:
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

	// Finalize the outer rectangle.  It is rounded outwards, so that it still
	// contains the text box: /RD records the box as insets from this
	// rectangle, and an inset may not be negative.
	outer = roundOut(outer)
	a.Rect = outer
	if inner.NearlyEqual(&outer, 0.01) {
		a.Margin = nil
	} else {
		// insets from the outer rectangle: left, bottom, right, top
		a.Margin = []float64{
			pdf.Round(inner.LLx-outer.LLx, 4),
			pdf.Round(inner.LLy-outer.LLy, 4),
			pdf.Round(outer.URx-inner.URx, 4),
			pdf.Round(outer.URy-inner.URy, 4),
		}
	}

	// set DA to match the font/size/color used in the appearance stream
	fontName := b.FontName(g.freeTextFont(a))
	a.DefaultAppearance = fmt.Sprintf("/%s %s Tf %s",
		fontName, strconv.FormatFloat(size, 'f', -1, 64), daColorOperator(textCol))

	return harvest(b, outer)
}
