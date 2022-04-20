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
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/pages"
)

type glyphBox font.GlyphID

func (g glyphBox) Extent() *boxes.BoxExtent {
	ht := theFont.Ascent
	if theFont.GlyphExtents != nil {
		bbox := theFont.GlyphExtents[g]
		ht = int(bbox.URy)
	}
	return &boxes.BoxExtent{
		Width:  glyphBoxWidth,
		Height: 4 + float64(ht)*glyphFontSize/1000,
		Depth:  8 - float64(theFont.Descent)*glyphFontSize/1000,
	}
}

func (g glyphBox) Draw(page *pages.Page, xPos, yPos float64) {
	q := float64(glyphFontSize) / 1000
	glyphWidth := float64(theFont.Widths[g]) * q
	shift := (glyphBoxWidth - glyphWidth) / 2

	if theFont.GlyphExtents != nil {
		ext := theFont.GlyphExtents[font.GlyphID(g)]
		page.Println("q")
		page.Println(".4 1 .4 rg")
		page.Printf("%.2f %.2f %.2f %.2f re\n",
			xPos+float64(ext.LLx)*q+shift, yPos+float64(ext.LLy)*q,
			float64(ext.URx-ext.LLx)*q, float64(ext.URy-ext.LLy)*q)
		page.Println("f")
		page.Println("Q")
	}

	yLow := yPos + float64(theFont.Descent)*q
	yHigh := yPos + float64(theFont.Ascent)*q
	page.Println("q")
	page.Println("1 0 0 RG")
	page.Println(".5 w")
	x := xPos + shift
	page.Printf("%.2f %.2f m\n", x, yLow)
	page.Printf("%.2f %.2f l\n", x, yHigh)
	x += glyphWidth
	page.Printf("%.2f %.2f m\n", x, yLow)
	page.Printf("%.2f %.2f l\n", x, yHigh)
	page.Println("s")
	page.Println("Q")

	r := rev[font.GlyphID(g)]
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
		class := gdefInfo.GlyphClass[font.GlyphID(g)]
		var classLabel string
		switch class {
		case 1:
			classLabel = "b" // Base glyph (single character, spacing glyph)
		case 2:
			classLabel = "l" // Ligature glyph (multiple character, spacing glyph)
		case 3:
			classLabel = "m" // Mark glyph (non-spacing combining glyph)
		case 4:
			classLabel = "c" // Component glyph (part of single character, spacing glyph)
		}
		if classLabel != "" {
			cBox := boxes.Text(courier, 8, classLabel)
			page.Println("q")
			page.Println("0.5 g")
			cBox.Draw(page,
				xPos+glyphBoxWidth-cBox.Extent().Width-1,
				yPos+float64(theFont.Descent)*q-6)
			page.Println("Q")
		}
	}

	page.Println("q")
	page.Println("BT")
	_ = theFont.InstName.PDF(page)
	fmt.Fprintf(page, " %f Tf\n", float64(glyphFontSize))
	fmt.Fprintf(page, "%f %f Td\n",
		xPos+shift,
		yPos)
	buf := theFont.Enc(font.GlyphID(g))
	_ = buf.PDF(page)
	page.Println(" Tj")
	page.Println("ET")
	page.Println("Q")
}

type rules struct{}

func (r rules) Extent() *boxes.BoxExtent {
	return &boxes.BoxExtent{
		Width:  0,
		Height: float64(theFont.Ascent) * glyphFontSize / 1000,
		Depth:  -float64(theFont.Descent) * glyphFontSize / 1000,
	}
}

func (r rules) Draw(page *pages.Page, xPos, yPos float64) {
	yLow := yPos + float64(theFont.Descent)*glyphFontSize/1000
	yHigh := yPos + float64(theFont.Ascent)*glyphFontSize/1000

	page.Println("q")
	page.Println(".3 .3 1 RG")
	page.Println(".5 w")
	for _, y := range []float64{
		yLow,
		yPos,
		yHigh,
	} {
		page.Printf("%.2f %.2f m\n", xPos, y)
		page.Printf("%.2f %.2f l\n", xPos+10*glyphBoxWidth, y)
	}
	for i := 0; i <= 10; i++ {
		x := xPos + float64(i)*glyphBoxWidth
		page.Printf("%.2f %.2f m\n", x, yLow)
		page.Printf("%.2f %.2f l\n", x, yHigh)
	}

	page.Println("s")
	page.Println("Q")
}
