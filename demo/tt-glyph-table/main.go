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
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/pages"
)

const (
	glyphBoxWidth = 36
	glyphFontSize = 24
)

var F1, F3 *font.Font
var rev map[font.GlyphIndex]rune

type glyphBox font.GlyphIndex

func (g glyphBox) Extent() *boxes.BoxExtent {
	bbox := F3.GlyphExtent[g]
	return &boxes.BoxExtent{
		Width:  glyphBoxWidth,
		Height: 4 + float64(bbox.URy)*glyphFontSize/1000,
		Depth:  8 - F3.Descent*glyphFontSize/1000,
	}
}

func (g glyphBox) Draw(page *pages.Page, xPos, yPos float64) {
	q := float64(glyphFontSize) / 1000
	glyphWidth := float64(F3.Width[g]) * q
	shift := (glyphBoxWidth - glyphWidth) / 2

	ext := F3.GlyphExtent[font.GlyphIndex(g)]
	page.Println("q")
	page.Println(".2 1 .2 rg")
	page.Printf("%.2f %.2f %.2f %.2f re\n",
		xPos+float64(ext.LLx)*q+shift, yPos+float64(ext.LLy)*q,
		float64(ext.URx-ext.LLx)*q, float64(ext.URy-ext.LLy)*q)
	page.Println("f")
	page.Println("Q")

	r := rev[font.GlyphIndex(g)]
	var label string
	if r != 0 {
		label = fmt.Sprintf("%04X", r)
	} else {
		label = "—"
	}
	lBox := boxes.NewText(F1, 8, label)
	lBox.Draw(page,
		xPos+(glyphBoxWidth-lBox.Extent().Width)/2,
		yPos+F3.Descent*q-6)

	page.Println("q")
	page.Println("BT")
	F3.Name.PDF(page)
	fmt.Fprintf(page, " %f Tf\n", float64(glyphFontSize))
	fmt.Fprintf(page, "%f %f Td\n",
		xPos+shift,
		yPos)
	buf := F3.Enc(font.GlyphIndex(g))
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

	out, err := pdf.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}

	charset1 := make(map[rune]bool)
	for _, r := range "0123456789ABCDEF- —" {
		charset1[r] = true
	}
	F1, err = builtin.Embed(out, "F1", "Courier", charset1)
	if err != nil {
		log.Fatal(err)
	}

	F2, err := builtin.Embed(out, "F2", "Times-Italic", nil)
	if err != nil {
		log.Fatal(err)
	}

	F3, err = truetype.Embed(out, "F3", fontFileName, nil)
	if err != nil {
		log.Fatal(err)
	}

	rev = make(map[font.GlyphIndex]rune)
	for r, idx := range F3.CMap {
		r2, seen := rev[idx]
		if seen {
			fmt.Println("duplicate:", idx, "->", r2, r)
		}
		if r2 == 0 || r < r2 {
			rev[idx] = r
		}
	}

	pageTree := pages.NewPageTree(out, &pages.DefaultAttributes{
		Resources: pdf.Dict{
			"Font": pdf.Dict{
				F1.Name: F1.Ref,
				F2.Name: F2.Ref,
				F3.Name: F3.Ref,
			},
		},
	})

	stretch := boxes.NewGlue(0, 1, 1, 1, 1)

	p := &boxes.Parameters{
		BaseLineSkip: 42,
	}

	numGlyph := len(F3.Width)

	var rowBoxes []boxes.Box
	page := 1
	for row := 0; 10*row < numGlyph; row++ {
		if len(rowBoxes) == 0 {
			rowBoxes = append(rowBoxes, boxes.Kern(72))
			headerBoxes := []boxes.Box{stretch, boxes.Kern(40)}
			for i := 0; i < 10; i++ {
				h := boxes.NewHBoxTo(glyphBoxWidth,
					stretch,
					boxes.NewText(F1, 10, strconv.Itoa(i)),
					stretch)
				headerBoxes = append(headerBoxes, h)
			}
			headerBoxes = append(headerBoxes, stretch)
			rowBoxes = append(rowBoxes,
				boxes.NewHBoxTo(pages.A4.URx, headerBoxes...))
		}

		colBoxes := []boxes.Box{stretch}
		label := strconv.Itoa(row)
		if label == "0" {
			label = ""
		}
		h := boxes.NewHBoxTo(20,
			stretch,
			boxes.NewText(F1, 10, label),
			boxes.NewText(F2, 10, "x"),
		)
		colBoxes = append(colBoxes, h, boxes.Kern(20))
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
			rowBoxes = append(rowBoxes,
				stretch,
				boxes.NewHBoxTo(pages.A4.URx,
					stretch,
					boxes.NewText(F1, 10, "- "+strconv.Itoa(page)+" -"),
					stretch),
				boxes.Kern(36))
			box := p.NewVBoxTo(pages.A4.URy, rowBoxes...)
			boxes.Ship(pageTree, box)
			rowBoxes = nil
			page++
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
