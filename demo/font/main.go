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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/pages"
)

const (
	fontSize = 48.0
)

func writePage(out *pdf.Writer, text string, width, height float64) error {
	subset := make(map[rune]bool)
	for _, r := range text {
		subset[r] = true
	}

	// F1, err := builtin.Embed(out, "F1", "Times-Roman", nil)
	// F1, err := truetype.Embed(out, "F1", "../../font/truetype/ttf/FreeSerif.ttf", subset)
	// F1, err := truetype.Embed(out, "F1", "../../font/truetype/ttf/Roboto-Regular.ttf", subset)
	F1, err := truetype.Embed(out, "F1", "../../font/truetype/ttf/SourceSerif4-Regular.ttf", subset)
	if err != nil {
		return err
	}

	page, err := pages.SinglePage(out, &pages.Attributes{
		Resources: pdf.Dict{
			"Font": pdf.Dict{F1.Name: F1.Ref},
		},
		MediaBox: &pdf.Rectangle{
			URx: width,
			URy: height,
		},
	})
	if err != nil {
		return err
	}

	margin := 50.0
	baseLineSkip := 1.2 * fontSize
	q := fontSize / float64(F1.GlyphUnits)
	layout := F1.Typeset(text, fontSize)
	glyphs := layout.Glyphs

	for _, glyph := range layout.Glyphs {
		fmt.Println(glyph)
	}

	page.Println("q")
	page.Println("1 .5 .5 RG")
	yPos := height - margin - F1.Ascent*q
	for y := yPos; y > margin; y -= baseLineSkip {
		page.Printf("%.1f %.1f m %.1f %.1f l\n", margin, y, width-margin, y)
	}
	page.Println("s")
	page.Println("Q")

	page.Println("q")
	page.Println(".2 1 .2 RG")
	xPos := margin
	for _, gl := range glyphs {
		c := gl.Gid
		bbox := F1.GlyphExtent[c]
		if !bbox.IsZero() {
			x := xPos + float64(gl.XOffset+bbox.LLx)*q
			y := yPos + float64(gl.YOffset+bbox.LLy)*q
			w := float64(bbox.URx-bbox.LLx) * q
			h := float64(bbox.URy-bbox.LLy) * q
			page.Printf("%.2f %.2f %.2f %.2f re\n", x, y, w, h)
			page.Printf("%.2f %.2f %.2f %.2f re\n", x, y-baseLineSkip, w, h)
		}
		xPos += float64(gl.Advance) * q
	}
	page.Println("s")
	page.Println("Q")

	box := boxes.NewText(F1, fontSize, text)
	box.Draw(page, margin, yPos-baseLineSkip)

	xPos = margin
	for _, gl := range glyphs {
		c := gl.Gid
		bbox := F1.GlyphExtent[c]
		if !bbox.IsZero() {
			x := xPos + float64(gl.XOffset)*q
			y := yPos + float64(gl.YOffset)*q
			page.Printf("BT /F1 %f Tf\n", fontSize)
			page.Printf("%f %f Td\n", x, y)
			enc := F1.Enc(c)
			enc.PDF(page)
			page.Println(" Tj")
			page.Println("ET")
		}
		xPos += float64(gl.Advance) * q
	}

	err = page.Close()
	if err != nil {
		return err
	}
	return nil
}

func main() {
	out, err := pdf.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}

	const width = 8 * 72
	const height = 6 * 72

	text := "VATa\u0308rfisch"
	err = writePage(out, text, width, height)
	if err != nil {
		log.Fatal(err)
	}

	out.SetInfo(&pdf.Info{
		Title:  "PDF Test Document",
		Author: "Jochen Voß",
	})

	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}
