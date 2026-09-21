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
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation/colorenc"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// iconSize is the side of the square an annotation icon is drawn in.  The
// types which show one pin their rectangle to a square this size, so that
// the appearance is not scaled.
const iconSize = 24

// stickyYellow is the card colour of an icon the document gives no colour.
var stickyYellow = quireAmber100

// darkCardL is the L* below which a card is too dark for the design inks.
const darkCardL = 55

// iconInks are the colours an icon's marks are drawn in: the symbol and the
// outline of the card, and the grey of the strokes which stand for lines of
// text.
type iconInks struct {
	glyph, outline, lines color.Color
}

// linesFraction is how far the grey of the text strokes stands from the glyph
// ink on the way to the card.  It is the distance the design's two greys keep
// on the two cards the generator draws by default, so the icons look as they
// did when the grey came from a two-entry table.
const linesFraction = 0.45

// inksFor returns the inks for a card filled with bg: the design inks on a
// light card, or on no card at all, and paper white on a dark one.
//
// The grey of the text strokes is not a colour of its own but the glyph ink
// muted towards the card, so that it keeps its distance from whatever the
// card is.  A fixed grey collapses against a card of about its own lightness:
// the two the design used left barely seven L* between the strokes and a card
// in the fifties.
//
// A card with no fill is taken to be light, and the strokes are muted towards
// paper.  The annotation cannot see what it is drawn on, so a transparent
// icon on a dark page is faint, which is what asking for no background means.
func inksFor(bg color.Color) iconInks {
	card := bg
	if card == nil {
		card = quireWhite
	}

	glyph, outline := color.Color(quireInk), color.Color(quireInk2)
	if lightness(card) < darkCardL {
		glyph, outline = quireAmber50, quireAmber50
	}
	return iconInks{
		glyph:   glyph,
		outline: outline,
		lines:   mix(glyph, card, linesFraction),
	}
}

// iconBackground returns the colour the card behind an icon is filled with,
// given the annotation's /C entry: the sticky yellow where the document
// names no colour, and nothing at all where it asks for none.
//
// §12.5.2 makes /C "the background of the annotation's icon when closed", so
// it paints the card rather than the symbol on it.
func iconBackground(col color.Color) color.Color {
	switch col {
	case nil:
		return stickyYellow
	case colorenc.Transparent:
		return nil
	default:
		return col
	}
}

// drawIcon draws the named annotation icon in the square from (0, 0) to
// (iconSize, iconSize): a card filled with bg, or outlined only where bg is
// nil, with the symbol on it.  A name the generator has no drawing for gets
// the bare card.
//
// The symbols use two inks and no more: the glyph ink for lines and marks,
// and the grey for the strokes which stand for lines of text.  Both follow
// the card, so that a dark card is drawn on in light ink.
func (g *Generator) drawIcon(b *builder.Builder, name pdf.Name, bg color.Color) {
	ink := inksFor(bg)

	// the Note icon's card has a folded corner, so it draws its own
	if name == "Note" {
		g.drawNoteIcon(b, bg, ink)
		return
	}

	drawCard(b, bg, ink.outline)

	switch name {
	case "Comment":
		g.symbol(b, ink, 23, 6, 2, 0.9, "“")
	case "Key":
		g.symbol(b, ink, 25, 5, 2, 1, "*")
	case "Help":
		g.symbol(b, ink, 23, 6, 4, 1, "?")
	case "Paragraph":
		g.symbol(b, ink, 16, 6, 8, 1.4, "¶")
	case "Insert":
		g.symbol(b, ink, 16, 5.5, 4, 1.4, "^")
	case "NewParagraph":
		drawNewParagraphIcon(b, ink)
	case "Graph":
		drawGraphIcon(b, ink)
	case "PushPin":
		drawPushPinIcon(b, ink)
	case "Paperclip":
		drawPaperclipIcon(b, ink)
	case "Tag":
		drawTagIcon(b, ink)
	case "Speaker":
		drawSpeakerIcon(b, ink)
	case "Mic":
		drawMicIcon(b, ink)
	}
}

// symbol draws a single character from the icon font at the given size and
// position, scaled horizontally by hScale.
func (g *Generator) symbol(b *builder.Builder, ink iconInks, size, x, y, hScale float64, s string) {
	b.TextBegin()
	b.TextSetFont(g.icons(), size)
	b.TextSetRise(0)
	b.TextSetHorizontalScaling(1)
	b.SetFillColor(ink.glyph)
	b.TextFirstLine(x, y)
	b.TextSetHorizontalScaling(hScale)
	b.TextShow(s)
	b.TextEnd()
}

// drawCard draws the card an icon is set on: a square outlined in outline
// and filled with bg, or left unfilled where bg is nil.
func drawCard(b *builder.Builder, bg, outline color.Color) {
	setCardPaint(b, bg, outline)
	b.Rectangle(0.25, 0.25, 23.5, 23.5)
	closeCard(b, bg)
}

// setCardPaint sets the line width and the colours a card is painted with.
func setCardPaint(b *builder.Builder, bg, outline color.Color) {
	b.SetLineWidth(0.5)
	b.SetStrokeColor(outline)
	if bg != nil {
		b.SetFillColor(bg)
	}
}

// closeCard closes the current path and paints it as a card: filled and
// stroked, or only stroked where bg is nil.
func closeCard(b *builder.Builder, bg color.Color) {
	if bg == nil {
		b.CloseAndStroke()
		return
	}
	b.CloseFillAndStroke()
}

// drawNoteIcon draws a card with a folded lower-right corner, ruled with
// lines of text.
func (g *Generator) drawNoteIcon(b *builder.Builder, bg color.Color, ink iconInks) {
	const delta = 7.0

	setCardPaint(b, bg, ink.outline)
	b.MoveTo(23.5-delta, 0.25)
	b.LineTo(0.25, 0.25)
	b.LineTo(0.25, 23.5)
	b.LineTo(23.5, 23.5)
	b.LineTo(23.5, 0.25+delta)
	b.LineTo(23.5-delta, 0.25)
	b.LineTo(23.5-delta, 0.25+delta)
	b.LineTo(23.5, 0.25+delta)
	closeCard(b, bg)

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
}

// drawNewParagraphIcon draws lines of text with a pilcrow-like arrow marking
// where the new paragraph begins.
func drawNewParagraphIcon(b *builder.Builder, ink iconInks) {
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
}

// drawPushPinIcon draws a push pin seen from the front: a fluted head over
// a needle running down to a point.
func drawPushPinIcon(b *builder.Builder, ink iconInks) {
	b.SetFillColor(ink.glyph)

	b.MoveTo(12+5, 20)
	b.LineTo(12+5, 18)
	b.LineTo(12+3, 16)
	b.LineTo(12+3, 13)
	b.LineTo(12+5, 11)
	b.LineTo(12+5, 9)
	b.LineTo(12+0.8, 9)
	b.LineTo(12+0.4, 3)
	b.LineTo(12, 2) // needle tip
	b.LineTo(12-0.4, 3)
	b.LineTo(12-0.8, 9)
	b.LineTo(12-5, 9)
	b.LineTo(12-5, 11)
	b.LineTo(12-3, 13)
	b.LineTo(12-3, 16)
	b.LineTo(12-5, 18)
	b.LineTo(12-5, 20)

	b.Fill()
}

// drawPaperclipIcon draws a Gem-style paperclip: two nested rounded loops
// with fully-semicircular ends (radius = half the short side).  The whole
// glyph is rotated 45° clockwise so the open tip -- the end where paper
// slides in -- sits in the bottom-left corner.  The inner loop is offset
// toward the clamp end, echoing real paperclip asymmetry.
func drawPaperclipIcon(b *builder.Builder, ink iconInks) {
	b.SetLineJoin(graphics.LineJoinRound)
	b.SetLineCap(graphics.LineCapRound)

	const w = 6

	// The loops are not symmetric about the diagonal they are built on, so
	// the ink comes out to the right of the card's middle: without dx it is
	// centred on 12.43 rather than on 12.  The arcs reach the file as curves
	// with one decimal, so the correction lands within half of that.
	const dx = -0.4

	// The clip is drawn larger than the other symbols -- 20.7 across, where
	// the tag, the other diagonal one, is 19 -- so it is scaled towards the
	// middle of the card to bring it into line.  The pen is left at the
	// width the other symbols use.
	const scale = 0.92
	at := func(v float64) float64 { return 12 + (v-12)*scale }

	b.SetLineWidth(1)
	b.SetStrokeColor(ink.glyph)
	b.MoveTo(at(19-w+dx), at(19))
	const R = w / math.Sqrt2
	b.LineToArc(at(3+w/2+dx), at(3+w/2), (R-0.3)*scale, 135/180.0*math.Pi, 315/180.0*math.Pi)
	b.LineToArc(at(22+.5-w/2+dx), at(22-.5-w/2), (R-1)*scale, -45/180.0*math.Pi, 135/180.0*math.Pi)
	b.LineToArc(at(6+w/2+dx), at(6+w/2), (R-2)*scale, 135/180.0*math.Pi, 315/180.0*math.Pi)
	b.LineTo(at(19-1.5+dx), at(19+1.5-w))
	b.Stroke()
}

// drawSpeakerIcon draws a loudspeaker silhouette as a single filled polygon
// (back box on the left + trapezoidal cone flaring to the right), followed
// by sound-wave arcs to the right of the cone front.
func drawSpeakerIcon(b *builder.Builder, ink iconInks) {
	b.SetFillColor(ink.glyph)
	b.MoveTo(4, 9)
	b.LineTo(6, 9)
	b.LineTo(10, 5)
	b.LineTo(10, 19)
	b.LineTo(6, 15)
	b.LineTo(4, 15)
	b.ClosePath()
	b.Fill()

	b.SetStrokeColor(ink.glyph)
	b.SetLineWidth(1)
	b.SetLineCap(graphics.LineCapRound)

	// concentric arcs centred inside the cone, opening to the right
	const cx, cy = 8.0, 12.0
	const sweep = math.Pi / 4
	for _, r := range []float64{5.5, 8, 10.5} {
		b.MoveTo(cx+r*math.Cos(-sweep), cy+r*math.Sin(-sweep))
		b.LineToArc(cx, cy, r, -sweep, sweep)
	}
	b.Stroke()
}

// drawMicIcon draws a stadium-shaped microphone capsule with a stand and
// base bar.
func drawMicIcon(b *builder.Builder, ink iconInks) {
	b.SetStrokeColor(ink.glyph)
	b.SetLineWidth(1)
	b.SetLineCap(graphics.LineCapRound)
	b.SetLineJoin(graphics.LineJoinRound)

	// capsule (stadium): vertical pill centred on x=12, total y from 8 to 20
	b.MoveTo(15, 17)
	b.LineToArc(12, 17, 3, 0, math.Pi)
	b.LineTo(9, 11)
	b.LineToArc(12, 11, 3, math.Pi, 2*math.Pi)
	b.ClosePath()
	b.Stroke()

	// stand and base bar
	b.MoveTo(12, 8)
	b.LineTo(12, 4)
	b.MoveTo(8, 4)
	b.LineTo(16, 4)
	b.Stroke()
}

// drawGraphIcon draws a three-bar chart with L-shaped axes.
func drawGraphIcon(b *builder.Builder, ink iconInks) {
	b.SetLineWidth(1)
	b.SetLineCap(graphics.LineCapSquare)

	// bars of increasing height
	b.SetFillColor(ink.glyph)
	b.Rectangle(7, 5, 2.5, 4)
	b.Rectangle(11, 5, 2.5, 8)
	b.Rectangle(15, 5, 2.5, 12)
	b.Fill()

	// axes
	b.SetStrokeColor(ink.glyph)
	b.MoveTo(5, 19)
	b.LineTo(5, 5)
	b.LineTo(19, 5)
	b.Stroke()
}

// drawTagIcon draws a luggage/price tag rotated 45° counter-clockwise so the
// pointed end with the string hole lands in the bottom-left corner.  The tip
// itself has a right angle with edges at ±45° slopes in local coordinates,
// so after rotation those two tip-adjacent edges are axis-aligned.  Three
// short grey strokes across the body stand for text on the rotated label.
func drawTagIcon(b *builder.Builder, ink iconInks) {
	b.SetLineJoin(graphics.LineJoinRound)

	// pentagon: 12×12 square body + triangular point.  The point's two
	// edges have slopes ±1 so they become axis-aligned after the 45°
	// rotation.
	const w = 6
	b.SetStrokeColor(ink.glyph)
	b.SetLineWidth(1)
	b.MoveTo(3, 3) // tip
	b.LineTo(3+w, 3)
	b.LineTo(22, 22-w)
	b.LineTo(22-w, 22)
	b.LineTo(3, 3+w)
	b.ClosePath()
	b.Stroke()

	// string hole, centred in the triangular point
	b.SetLineWidth(0.8)
	b.Circle(6, 6, 1.2)
	b.Stroke()

	// three "text" lines across the body: two long, one short
	b.SetStrokeColor(ink.lines)
	b.SetLineWidth(0.7)
	b.SetLineCap(graphics.LineCapButt)
	b.MoveTo(5+0.3*w, 5+w-0.3*w)
	b.LineTo(20-0.7*w, 20-w+0.7*w)
	b.MoveTo(5+0.5*w, 5+w-0.5*w)
	b.LineTo(20-0.5*w, 20-w+0.5*w)
	b.MoveTo(5+0.7*w, 5+w-0.7*w)
	b.LineTo(15-0.3*w, 15-w+0.3*w)
	b.Stroke()
}
