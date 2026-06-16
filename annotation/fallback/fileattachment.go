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
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
)

// The icon is drawn in a fixed 24×24 local frame.  The annotation's Rect
// is pinned to a 24×24 square anchored at the caller's upper-left corner
// (the (LLx, URy) point of the supplied Rect), and NoZoom | NoRotate are
// forced — matching the fixed-icon convention that §12.5.6.4 imposes on
// text annotations implicitly, but which FileAttachment does not get for
// free.  See §12.5.3: "the annotation's position is defined by the
// coordinates of the upper-left corner of its annotation rectangle".
func (s *Style) addFileAttachmentAppearance(a *annotation.FileAttachment) (*form.Form, error) {
	a.Rect = pdf.Rectangle{
		LLx: a.Rect.LLx,
		LLy: a.Rect.URy - 24,
		URx: a.Rect.LLx + 24,
		URy: a.Rect.URy,
	}
	a.Flags |= annotation.FlagNoZoom | annotation.FlagNoRotate

	col := a.Color
	if col == nil {
		col = quireInk2
	}

	b := builder.New(content.Form, nil, s.version)
	b.SetExtGState(s.reset)

	// card background: hairline slate-3 border + slate-1 fill.  Cool
	// chrome on the warm page makes the icon read as viewer-added
	// metadata rather than editorial content.
	b.SetLineWidth(0.5)
	b.SetStrokeColor(quireSlate3)
	b.SetFillColor(quireSlate1)
	b.Rectangle(0.5, 0.5, 23, 23)
	b.FillAndStroke()

	switch a.Icon {
	case annotation.FileAttachmentIconPaperclip:
		drawPaperclipIcon(b, col)
	case annotation.FileAttachmentIconGraph:
		drawGraphIcon(b, col)
	case annotation.FileAttachmentIconTag:
		drawTagIcon(b, col)
	default:
		// default; also handles unknown icon names
		drawPushPinIcon(b, col)
	}

	return harvest(b, pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24})
}

// icons fit inside the slate card background drawn by addFileAttachmentAppearance

// drawPushPinIcon draws a thumbtack as a single symmetric polygon: a broad
// head at the top, a flared base plate beneath it, and a tapered needle
// ending at y=2.
func drawPushPinIcon(b *builder.Builder, col color.Color) {
	b.SetFillColor(col)

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

// drawPaperclipIcon draws a Gem-style paperclip: two nested rounded
// loops with fully-semicircular ends (radius = half the short side).
// The whole glyph is rotated 45° clockwise so the open tip — the end
// where paper slides in — sits in the bottom-left corner.  The inner
// loop is offset toward the clamp end, echoing real paperclip
// asymmetry.
func drawPaperclipIcon(b *builder.Builder, col color.Color) {
	b.SetLineJoin(graphics.LineJoinRound)
	b.SetLineCap(graphics.LineCapRound)

	const w = 6
	b.SetLineWidth(1)
	b.SetStrokeColor(col)
	b.MoveTo(19-w, 19)
	const R = w / math.Sqrt2
	b.LineToArc(3+w/2, 3+w/2, R-0.3, 135/180.0*math.Pi, 315/180.0*math.Pi)
	b.LineToArc(22+.5-w/2, 22-.5-w/2, R-1, -45/180.0*math.Pi, 135/180.0*math.Pi)
	b.LineToArc(6+w/2, 6+w/2, R-2, 135/180.0*math.Pi, 315/180.0*math.Pi)
	b.LineTo(19-1.5, 19+1.5-w)
	b.Stroke()
}

// drawGraphIcon draws a three-bar chart with L-shaped axes. The bars
// deliberately render in amber — editorially interesting data is the one
// place the Quire palette reserves amber for inside UI chrome.
func drawGraphIcon(b *builder.Builder, col color.Color) {
	b.SetLineWidth(1)
	b.SetLineCap(graphics.LineCapSquare)

	// bars of increasing height, amber
	b.SetFillColor(quireAmber400)
	b.Rectangle(7, 5, 2.5, 4)
	b.Rectangle(11, 5, 2.5, 8)
	b.Rectangle(15, 5, 2.5, 12)
	b.Fill()

	// axes in the icon colour
	b.SetStrokeColor(col)
	b.MoveTo(5, 19)
	b.LineTo(5, 5)
	b.LineTo(19, 5)
	b.Stroke()
}

// drawTagIcon draws a luggage/price tag rotated 45° counter-clockwise
// so the pointed end with the string hole lands in the bottom-left
// corner.  The tip itself has a right angle with edges at ±45° slopes
// in local coordinates, so after rotation those two tip-adjacent
// edges are axis-aligned.  Three short gray strokes across the body
// emulate text on the rotated label.
func drawTagIcon(b *builder.Builder, col color.Color) {
	b.SetLineJoin(graphics.LineJoinRound)

	// pentagon: 12×12 square body + triangular point.  The point's two
	// edges have slopes ±1 so they become axis-aligned after the 45°
	// rotation.
	const w = 6
	b.SetStrokeColor(col)
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
	b.SetStrokeColor(quireInk3)
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
