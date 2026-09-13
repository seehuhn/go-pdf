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
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

func (g *Generator) addTextAppearance(a *annotation.Text) (*form.Form, error) {
	// §12.5.6.4: text annotations behave as if NoZoom and NoRotate were
	// always set, anchored at the upper-left corner of Rect.  Pin Rect to
	// a 24×24 square at that corner so the §12.5.5 scale-to-Rect algorithm
	// is a no-op on viewers that don't honour the implicit flags.
	a.Rect = pdf.Rectangle{
		LLx: a.Rect.LLx,
		LLy: a.Rect.URy - 24,
		URx: a.Rect.LLx + 24,
		URy: a.Rect.URy,
	}

	// the card is filled with the annotation's colour: the sticky yellow
	// where the document gives none, and nothing where it asks for no colour
	var bgCol color.Color
	switch a.Color {
	case nil:
		bgCol = stickyYellow
	case colorenc.Transparent:
		// no fill
	default:
		bgCol = a.Color
	}
	ink := noteInksFor(bgCol)

	b := builder.New(content.Form, nil, g.version)

	switch a.Icon {
	case annotation.TextIconComment:
		g.reset(b)
		drawNoteCard(b, bgCol, ink.outline)

		b.TextBegin()
		b.TextSetFont(g.icons(), 23)
		b.TextSetRise(0)
		b.TextSetHorizontalScaling(1)
		b.SetFillColor(ink.glyph)
		b.TextFirstLine(6, 2)
		b.TextSetHorizontalScaling(0.9)
		b.TextShow("\u201C")
		b.TextEnd()

	case annotation.TextIconKey:
		g.reset(b)
		drawNoteCard(b, bgCol, ink.outline)

		b.TextBegin()
		b.TextSetFont(g.icons(), 25)
		b.TextSetRise(0)
		b.TextSetHorizontalScaling(1)
		b.SetFillColor(ink.glyph)
		b.TextFirstLine(5, 2)
		b.TextShow("*")
		b.TextEnd()

	case annotation.TextIconNote, "":
		delta := 7.0

		g.reset(b)
		setCardPaint(b, bgCol, ink.outline)
		b.MoveTo(23.5-delta, 0.25)
		b.LineTo(0.25, 0.25)
		b.LineTo(0.25, 23.5)
		b.LineTo(23.5, 23.5)
		b.LineTo(23.5, 0.25+delta)
		b.LineTo(23.5-delta, 0.25)
		b.LineTo(23.5-delta, 0.25+delta)
		b.LineTo(23.5, 0.25+delta)
		closeCard(b, bgCol)

		b.SetLineWidth(1.5)
		b.SetStrokeColor(ink.lines)
		for y := 19.; y > 6; y -= 3.5 {
			b.MoveTo(4, y)
			if y > 10 {
				b.LineTo(17, y)
			} else {
				b.LineTo(12, y)
			}
		}
		b.Stroke()

	case annotation.TextIconHelp:
		g.reset(b)

		drawNoteCard(b, bgCol, ink.outline)

		b.TextBegin()
		b.TextSetFont(g.icons(), 23)
		b.TextSetRise(0)
		b.TextSetHorizontalScaling(1)
		b.SetFillColor(ink.glyph)
		b.TextFirstLine(6, 4)
		b.TextShow("?")
		b.TextEnd()

	case annotation.TextIconNewParagraph:
		g.reset(b)

		drawNoteCard(b, bgCol, ink.outline)

		b.SetStrokeColor(ink.lines)
		b.SetLineWidth(1.5)
		b.MoveTo(4, 19)
		b.LineTo(17, 19)
		b.MoveTo(4, 15.5)
		b.LineTo(12, 15.5)
		b.MoveTo(4, 5)
		b.LineTo(17, 5)
		b.Stroke()

		m := (15.5 + 5) / 2

		b.SetStrokeColor(ink.glyph)
		b.SetFillColor(ink.glyph)
		b.SetLineWidth(1.8)
		b.MoveTo(17.5-0.75, 15.5)
		b.LineTo(17.5-0.75, m)
		b.LineTo(5, m)
		b.Stroke()
		b.MoveTo(3, m)
		b.LineTo(7, m+2.8)
		b.LineTo(7, m-2.8)
		b.Fill()

	case annotation.TextIconParagraph:
		g.reset(b)

		drawNoteCard(b, bgCol, ink.outline)

		b.TextBegin()
		b.TextSetFont(g.icons(), 16)
		b.TextSetRise(0)
		b.TextSetHorizontalScaling(1)
		b.SetFillColor(ink.glyph)
		b.TextFirstLine(6, 8)
		b.TextSetHorizontalScaling(1.4)
		b.TextShow("¶")
		b.TextEnd()

	case annotation.TextIconInsert:
		g.reset(b)

		drawNoteCard(b, bgCol, ink.outline)

		b.TextBegin()
		b.TextSetFont(g.icons(), 16)
		b.TextSetRise(0)
		b.TextSetHorizontalScaling(1)
		b.SetFillColor(ink.glyph)
		b.TextFirstLine(5.5, 4)
		b.TextSetHorizontalScaling(1.4)
		b.TextShow("^")
		b.TextEnd()

	default:
		g.reset(b)

		drawNoteCard(b, bgCol, ink.outline)
	}

	return harvest(b, pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24})
}

// drawNoteCard draws the card a note's icon is set on: a square outlined in
// outline and filled with bgCol, or left unfilled where bgCol is nil.
func drawNoteCard(b *builder.Builder, bgCol, outline color.Color) {
	setCardPaint(b, bgCol, outline)
	b.Rectangle(0.25, 0.25, 23.5, 23.5)
	closeCard(b, bgCol)
}

// setCardPaint sets the line width and the colours a card is painted with.
func setCardPaint(b *builder.Builder, bgCol, outline color.Color) {
	b.SetLineWidth(0.5)
	b.SetStrokeColor(outline)
	if bgCol != nil {
		b.SetFillColor(bgCol)
	}
}

// closeCard closes the current path and paints it as a card: filled and
// stroked, or only stroked where bgCol is nil.
func closeCard(b *builder.Builder, bgCol color.Color) {
	if bgCol == nil {
		b.CloseAndStroke()
		return
	}
	b.CloseFillAndStroke()
}

// noteInks are the colours a note's marks are drawn in: the glyph, the
// outline of the card and the ruled lines of the Note icon.
type noteInks struct {
	glyph, outline, lines color.Color
}

// darkCardL is the L* below which a card is too dark for the design inks.
const darkCardL = 55

// noteInksFor returns the inks for a card filled with bgCol: the design inks
// on a light card, or on no card at all, and paper white with the lightest
// ink neutral on a dark one.
func noteInksFor(bgCol color.Color) noteInks {
	if bgCol != nil && lightness(bgCol) < darkCardL {
		return noteInks{glyph: quireAmber50, outline: quireAmber50, lines: quireInk4}
	}
	return noteInks{glyph: quireInk, outline: quireInk2, lines: quireInk3}
}

// stickyYellow is the card colour of a note the document gives no colour.
var stickyYellow = quireAmber100
