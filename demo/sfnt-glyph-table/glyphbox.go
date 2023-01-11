// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package main

import (
	"fmt"

	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/sfnt/glyph"
)

type glyphBox glyph.ID

func (g glyphBox) Extent() *boxes.BoxExtent {
	ht := theFont.Ascent
	if theFont.GlyphExtents != nil {
		bbox := theFont.GlyphExtents[g]
		ht = bbox.URy
	}
	return &boxes.BoxExtent{
		Width:  glyphBoxWidth,
		Height: 4 + float64(ht)*glyphFontSize/1000,
		Depth:  8 - float64(theFont.Descent)*glyphFontSize/1000,
	}
}

func (g glyphBox) Draw(page *graphics.Page, xPos, yPos float64) {
	q := float64(glyphFontSize) / 1000
	glyphWidth := float64(theFont.Widths[g]) * q
	shift := (glyphBoxWidth - glyphWidth) / 2

	if theFont.GlyphExtents != nil {
		ext := theFont.GlyphExtents[glyph.ID(g)]
		page.PushGraphicsState()
		page.SetFillColor(color.RGB(.4, 1, .4))
		page.Rectangle(
			xPos+float64(ext.LLx)*q+shift, yPos+float64(ext.LLy)*q,
			float64(ext.URx-ext.LLx)*q, float64(ext.URy-ext.LLy)*q)
		page.Fill()
		page.PopGraphicsState()
	}

	yLow := yPos + float64(theFont.Descent)*q
	yHigh := yPos + float64(theFont.Ascent)*q
	page.PushGraphicsState()
	page.SetStrokeColor(color.RGB(1, 0, 0))
	page.SetLineWidth(.5)
	x := xPos + shift
	page.MoveTo(x, yLow)
	page.LineTo(x, yHigh)
	x += glyphWidth
	page.MoveTo(x, yLow)
	page.LineTo(x, yHigh)
	page.Stroke()
	page.PopGraphicsState()

	r := rev[glyph.ID(g)]
	var label string
	if r != 0 {
		label = fmt.Sprintf("%04X", r)
	} else {
		label = "—"
	}
	lBox := boxes.Text(courier, 8, label)
	lBox.Draw(page,
		xPos+(glyphBoxWidth-lBox.Extent().Width)/2,
		yPos+float64(theFont.Descent)*q-6)

	if gdefInfo != nil {
		class := gdefInfo.GlyphClass[glyph.ID(g)]
		var classLabel string
		switch class {
		case 1:
			classLabel = "b" // Base glyph (single character, spacing glyph)
		case 2:
			classLabel = "l" // Ligature glyph (multiple character, spacing glyph)
		case 3:
			classLabel = "m" // Mark glyph (non-spacing, combining glyph)
		case 4:
			classLabel = "c" // Component glyph (part of single character, spacing glyph)
		}
		if classLabel != "" {
			cBox := boxes.Text(courier, 8, classLabel)
			page.PushGraphicsState()
			page.SetFillColor(color.Gray(.5))
			cBox.Draw(page,
				xPos+glyphBoxWidth-cBox.Extent().Width-1,
				yPos+float64(theFont.Descent)*q-6)
			page.PopGraphicsState()
		}
	}

	page.PushGraphicsState()
	page.BeginText()
	page.SetFont(theFont, glyphFontSize)
	page.StartLine(xPos+shift, yPos)
	gg := []glyph.Info{
		{
			Gid:     glyph.ID(g),
			Advance: theFont.Widths[glyph.ID(g)],
		},
	}
	page.ShowGlyphs(gg)
	page.EndText()
	page.PopGraphicsState()
}

type rules struct{}

func (r rules) Extent() *boxes.BoxExtent {
	return &boxes.BoxExtent{
		Width:  0,
		Height: float64(theFont.Ascent) * glyphFontSize / 1000,
		Depth:  -float64(theFont.Descent) * glyphFontSize / 1000,
	}
}

func (r rules) Draw(page *graphics.Page, xPos, yPos float64) {
	yLow := yPos + float64(theFont.Descent)*glyphFontSize/1000
	yHigh := yPos + float64(theFont.Ascent)*glyphFontSize/1000

	page.PushGraphicsState()
	page.SetStrokeColor(color.RGB(.3, .3, 1))
	page.SetLineWidth(.5)
	for _, y := range []float64{
		yLow,
		yPos,
		yHigh,
	} {
		page.MoveTo(xPos, y)
		page.LineTo(xPos+10*glyphBoxWidth, y)
	}
	for i := 0; i <= 10; i++ {
		x := xPos + float64(i)*glyphBoxWidth
		page.MoveTo(x, yLow)
		page.LineTo(x, yHigh)
	}
	page.Stroke()
	page.PopGraphicsState()
}
