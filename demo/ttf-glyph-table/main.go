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
	"seehuhn.de/go/pdf/font/sfnt/parser"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/locale"
	"seehuhn.de/go/pdf/pages"
)

const (
	glyphBoxWidth = 36
	glyphFontSize = 24
)

var courier, theFont *font.Font
var rev map[font.GlyphID]rune
var gdef *parser.GdefInfo

type rules struct{}

func (r rules) Extent() *boxes.BoxExtent {
	return &boxes.BoxExtent{
		Width:  0,
		Height: theFont.Ascent * glyphFontSize / float64(theFont.GlyphUnits),
		Depth:  -theFont.Descent * glyphFontSize / float64(theFont.GlyphUnits),
	}
}

func (r rules) Draw(page *pages.Page, xPos, yPos float64) {
	yLow := yPos + theFont.Descent*glyphFontSize/float64(theFont.GlyphUnits)
	yHigh := yPos + theFont.Ascent*glyphFontSize/float64(theFont.GlyphUnits)

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

type glyphBox font.GlyphID

func (g glyphBox) Extent() *boxes.BoxExtent {
	bbox := theFont.GlyphExtent[g]
	return &boxes.BoxExtent{
		Width:  glyphBoxWidth,
		Height: 4 + float64(bbox.URy)*glyphFontSize/float64(theFont.GlyphUnits),
		Depth:  8 - theFont.Descent*glyphFontSize/float64(theFont.GlyphUnits),
	}
}

func (g glyphBox) Draw(page *pages.Page, xPos, yPos float64) {
	q := float64(glyphFontSize) / float64(theFont.GlyphUnits)
	glyphWidth := float64(theFont.Width[g]) * q
	shift := (glyphBoxWidth - glyphWidth) / 2

	ext := theFont.GlyphExtent[font.GlyphID(g)]
	page.Println("q")
	page.Println(".4 1 .4 rg")
	page.Printf("%.2f %.2f %.2f %.2f re\n",
		xPos+float64(ext.LLx)*q+shift, yPos+float64(ext.LLy)*q,
		float64(ext.URx-ext.LLx)*q, float64(ext.URy-ext.LLy)*q)
	page.Println("f")
	page.Println("Q")

	yLow := yPos + theFont.Descent*q
	yHigh := yPos + theFont.Ascent*q
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
		yPos+theFont.Descent*q-6)

	if gdef != nil {
		class := gdef.GlyphClassDef[font.GlyphID(g)]
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
				yPos+theFont.Descent*q-6)
			page.Println("Q")
		}
	}

	page.Println("q")
	page.Println("BT")
	theFont.InstName.PDF(page)
	fmt.Fprintf(page, " %f Tf\n", float64(glyphFontSize))
	fmt.Fprintf(page, "%f %f Td\n",
		xPos+shift,
		yPos)
	buf := theFont.Enc(font.GlyphID(g))
	buf.PDF(page)
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

	pars := parser.New(tt)
	gdef, err = pars.ReadGdefTable()
	if err != nil && !table.IsMissing(err) {
		log.Fatal(err)
	}

	gsub, err := pars.ReadGsubTable(locale.EnGB)
	if err != nil && !table.IsMissing(err) {
		log.Fatal(err)
	}

	out, err := pdf.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}

	courier, err = builtin.Embed(out, "C", "Courier")
	if err != nil {
		log.Fatal(err)
	}
	Italic, err := builtin.Embed(out, "I", "Times-Italic")
	if err != nil {
		log.Fatal(err)
	}
	theFont, err = truetype.EmbedFontCID(out, "X", tt, nil)
	// Font, err = builtin.Embed(out, "X", "Times-Roman", font.AdobeStandardLatin)
	if err != nil {
		log.Fatal(err)
	}
	pageTree := pages.NewPageTree(out, &pages.DefaultAttributes{
		Resources: pdf.Dict{
			"Font": pdf.Dict{
				courier.InstName: courier.Ref,
				Italic.InstName:  Italic.Ref,
				theFont.InstName: theFont.Ref,
			},
		},
	})

	p := &boxes.Parameters{}
	stretch := boxes.Glue(0, 1, 1, 1, 1)

	page := 1
	var rowBoxes []boxes.Box
	flush := func() {
		rowBoxes = append(rowBoxes,
			stretch,
			boxes.HBoxTo(pages.A4.URx,
				stretch,
				boxes.Text(courier, 10, "- "+strconv.Itoa(page)+" -"),
				stretch),
			boxes.Kern(36))
		box := p.VBoxTo(pages.A4.URy, rowBoxes...)
		err = boxes.Ship(pageTree, box)
		if err != nil {
			log.Fatal(err)
		}
		rowBoxes = nil
		page++
	}

	numGlyph := len(theFont.Width)

	p.BaseLineSkip = 13
	rowBoxes = append(rowBoxes,
		boxes.Kern(72),
		boxes.HBox(
			boxes.Kern(72),
			boxes.Text(courier, 10, "input file: "),
			boxes.Text(courier, 10, fontFileName),
		),
		boxes.Kern(13),
		boxes.HBox(
			boxes.Kern(72),
			boxes.Text(courier, 10, "number of glyphs: "),
			boxes.Text(courier, 10, strconv.Itoa(numGlyph)),
		),
		boxes.HBox(
			boxes.Kern(72),
			boxes.Text(courier, 10, "number of GSUB lookups: "),
			boxes.Text(courier, 10, strconv.Itoa(len(gsub))),
		),
	)
	flush()

	p.BaseLineSkip = 46
	rev = make(map[font.GlyphID]rune)
	cmap, err := tt.SelectCMap()
	if err != nil {
		log.Fatal(err)
	}
	for r, idx := range cmap {
		r2 := rev[idx]
		if r2 == 0 || r < r2 {
			rev[idx] = r
		}
	}
	for row := 0; 10*row < numGlyph; row++ {
		if len(rowBoxes) == 0 {
			rowBoxes = append(rowBoxes, boxes.Kern(36))
			headerBoxes := []boxes.Box{stretch, boxes.Kern(40)}
			for i := 0; i < 10; i++ {
				h := boxes.HBoxTo(glyphBoxWidth,
					stretch,
					boxes.Text(courier, 10, strconv.Itoa(i)),
					stretch)
				headerBoxes = append(headerBoxes, h)
			}
			headerBoxes = append(headerBoxes, stretch)
			rowBoxes = append(rowBoxes,
				boxes.HBoxTo(pages.A4.URx, headerBoxes...))
			rowBoxes = append(rowBoxes, boxes.Kern(8))
		}

		colBoxes := []boxes.Box{stretch}
		label := strconv.Itoa(row)
		if label == "0" {
			label = ""
		}
		h := boxes.HBoxTo(20,
			stretch,
			boxes.Text(courier, 10, label),
			boxes.Text(Italic, 10, "x"),
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
			boxes.HBoxTo(pages.A4.URx, colBoxes...))

		if row%16 == 15 || 10*(row+1) >= numGlyph {
			flush()
		}
	}

	pagesRef, err := pageTree.Flush()
	if err != nil {
		log.Fatal(err)
	}

	out.SetCatalog(&pdf.Catalog{
		Pages: pagesRef,
	})

	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}
