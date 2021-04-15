// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
	"log"
	"os"
	"strconv"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/pages"
)

const (
	glyphBoxWidth = 36
	glyphFontSize = 24
)

var Courier, Font *font.Font
var rev map[font.GlyphIndex]rune

type rules struct{}

func (r rules) Extent() *boxes.BoxExtent {
	return &boxes.BoxExtent{
		Width:  0,
		Height: Font.Ascent * glyphFontSize / 1000,
		Depth:  -Font.Descent * glyphFontSize / 1000,
	}
}

func (r rules) Draw(page *pages.Page, xPos, yPos float64) {
	yLow := yPos + Font.Descent*glyphFontSize/1000
	yHigh := yPos + Font.Ascent*glyphFontSize/1000

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

type glyphBox font.GlyphIndex

func (g glyphBox) Extent() *boxes.BoxExtent {
	bbox := Font.GlyphExtent[g]
	return &boxes.BoxExtent{
		Width:  glyphBoxWidth,
		Height: 4 + float64(bbox.URy)*glyphFontSize/1000,
		Depth:  8 - Font.Descent*glyphFontSize/1000,
	}
}

func (g glyphBox) Draw(page *pages.Page, xPos, yPos float64) {
	q := float64(glyphFontSize) / 1000
	glyphWidth := float64(Font.Width[g]) * q
	shift := (glyphBoxWidth - glyphWidth) / 2

	ext := Font.GlyphExtent[font.GlyphIndex(g)]
	page.Println("q")
	page.Println(".3 1 .3 rg")
	page.Printf("%.2f %.2f %.2f %.2f re\n",
		xPos+float64(ext.LLx)*q+shift, yPos+float64(ext.LLy)*q,
		float64(ext.URx-ext.LLx)*q, float64(ext.URy-ext.LLy)*q)
	page.Println("f")
	page.Println("Q")

	yLow := yPos + Font.Descent*q
	yHigh := yPos + Font.Ascent*q
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

	r := rev[font.GlyphIndex(g)]
	var label string
	if r != 0 {
		label = fmt.Sprintf("%04X", r)
	} else {
		label = "—"
	}
	lBox := boxes.NewText(Courier, 8, label)
	lBox.Draw(page,
		xPos+(glyphBoxWidth-lBox.Extent().Width)/2,
		yPos+Font.Descent*q-6)

	page.Println("q")
	page.Println("BT")
	Font.Name.PDF(page)
	fmt.Fprintf(page, " %f Tf\n", float64(glyphFontSize))
	fmt.Fprintf(page, "%f %f Td\n",
		xPos+shift,
		yPos)
	buf := Font.Enc(font.GlyphIndex(g))
	pdf.String(buf).PDF(page)
	page.Println(" Tj")
	page.Println("ET")
	page.Println("Q")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: tt-glyph-table font.ttf")
		os.Exit(1)
	}
	fontFileName := os.Args[1]

	tt, err := sfnt.Open(fontFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer tt.Close()

	// TODO(voss): remove this
	fmt.Println(fontFileName + ":")

	out, err := pdf.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}

	Courier, err = builtin.Embed(out, "C", "Courier", nil)
	if err != nil {
		log.Fatal(err)
	}
	Italic, err := builtin.Embed(out, "I", "Times-Italic", nil)
	if err != nil {
		log.Fatal(err)
	}
	Font, err = truetype.EmbedFont(out, "X", tt, nil)
	// Font, err = builtin.Embed(out, "X", "Times-Roman", font.AdobeStandardLatin)
	if err != nil {
		log.Fatal(err)
	}
	pageTree := pages.NewPageTree(out, &pages.DefaultAttributes{
		Resources: pdf.Dict{
			"Font": pdf.Dict{
				Courier.Name: Courier.Ref,
				Italic.Name:  Italic.Ref,
				Font.Name:    Font.Ref,
			},
		},
	})

	p := &boxes.Parameters{}
	stretch := boxes.NewGlue(0, 1, 1, 1, 1)

	page := 1
	var rowBoxes []boxes.Box
	flush := func() {
		rowBoxes = append(rowBoxes,
			stretch,
			boxes.NewHBoxTo(pages.A4.URx,
				stretch,
				boxes.NewText(Courier, 10, "- "+strconv.Itoa(page)+" -"),
				stretch),
			boxes.Kern(36))
		box := p.NewVBoxTo(pages.A4.URy, rowBoxes...)
		boxes.Ship(pageTree, box)
		rowBoxes = nil
		page++
	}

	numGlyph := len(Font.Width)

	p.BaseLineSkip = 13
	rowBoxes = append(rowBoxes,
		boxes.Kern(72),
		boxes.NewHBox(
			boxes.Kern(72),
			boxes.NewText(Courier, 10, "input file: "),
			boxes.NewText(Courier, 10, fontFileName),
		),
		boxes.Kern(13),
		boxes.NewHBox(
			boxes.Kern(72),
			boxes.NewText(Courier, 10, "number of glyphs: "),
			boxes.NewText(Courier, 10, strconv.Itoa(numGlyph)),
		),
		boxes.NewHBox(
			boxes.Kern(72),
			boxes.NewText(Courier, 10, "number of ligatures: "),
			boxes.NewText(Courier, 10, strconv.Itoa(len(Font.Ligatures))),
		),
		boxes.NewHBox(
			boxes.Kern(72),
			boxes.NewText(Courier, 10, "number of kerns: "),
			boxes.NewText(Courier, 10, strconv.Itoa(len(Font.Kerning))),
		),
	)
	flush()

	p.BaseLineSkip = 46
	rev = make(map[font.GlyphIndex]rune)
	for r, idx := range Font.CMap {
		r2, seen := rev[idx]
		if seen {
			// fmt.Printf("duplicate: %d -> 0x%04x 0x%04x\n",
			// 	idx, r2, r)
		}
		if r2 == 0 || r < r2 {
			rev[idx] = r
		}
	}
	for row := 0; 10*row < numGlyph; row++ {
		if len(rowBoxes) == 0 {
			rowBoxes = append(rowBoxes, boxes.Kern(36))
			headerBoxes := []boxes.Box{stretch, boxes.Kern(40)}
			for i := 0; i < 10; i++ {
				h := boxes.NewHBoxTo(glyphBoxWidth,
					stretch,
					boxes.NewText(Courier, 10, strconv.Itoa(i)),
					stretch)
				headerBoxes = append(headerBoxes, h)
			}
			headerBoxes = append(headerBoxes, stretch)
			rowBoxes = append(rowBoxes,
				boxes.NewHBoxTo(pages.A4.URx, headerBoxes...))
			rowBoxes = append(rowBoxes, boxes.Kern(8))
		}

		colBoxes := []boxes.Box{stretch}
		label := strconv.Itoa(row)
		if label == "0" {
			label = ""
		}
		h := boxes.NewHBoxTo(20,
			stretch,
			boxes.NewText(Courier, 10, label),
			boxes.NewText(Italic, 10, "x"),
		)
		colBoxes = append(colBoxes, h, boxes.Kern(20), rules{})
		for col := 0; col < 10; col++ {
			idx := col + 10*row
			if idx < numGlyph {
				colBoxes = append(colBoxes, glyphBox(idx))
			} else {
				colBoxes = append(colBoxes, boxes.Kern(glyphBoxWidth))
			}
		}
		colBoxes = append(colBoxes, stretch)
		rowBoxes = append(rowBoxes,
			boxes.NewHBoxTo(pages.A4.URx, colBoxes...))

		if row%16 == 15 || 10*(row+1) >= numGlyph {
			flush()
		}
	}

	pagesRef, err := pageTree.Flush()
	if err != nil {
		log.Fatal(err)
	}

	err = out.SetCatalog(pdf.Struct(&pdf.Catalog{
		Pages: pagesRef,
	}))
	if err != nil {
		log.Fatal(err)
	}

	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}
