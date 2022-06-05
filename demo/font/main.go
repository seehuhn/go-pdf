// seehuhn.de/go/pdf - a library for reading and writing PDF files
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
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/simple"
	"seehuhn.de/go/pdf/locale"
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

	fontFile := "../../font/otf/SourceSerif4-Regular.otf"
	var F1 *font.Font
	if strings.HasSuffix(fontFile, ".ttf") || strings.HasSuffix(fontFile, ".otf") {
		fd, err := os.Open(fontFile)
		if err != nil {
			return err
		}
		info, err := sfnt.Read(fd)
		if err != nil {
			fd.Close()
			return err
		}
		err = fd.Close()
		if err != nil {
			return err
		}
		F1, err = simple.Embed(out, info, "F1", locale.EnUS)
		if err != nil {
			return err
		}
	} else {
		var err error
		F1, err = builtin.Embed(out, fontFile, "F1")
		if err != nil {
			return err
		}
	}

	pageTree := pages.NewPageTree(out, nil)
	page, err := pageTree.NewPage(&pages.Attributes{
		Resources: &pages.Resources{
			Font: pdf.Dict{F1.InstName: F1.Ref},
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
	layout := F1.Typeset(text, fontSize)
	glyphs := layout.Glyphs

	for _, glyph := range layout.Glyphs {
		fmt.Printf("%q %v\n", string(glyph.Text), glyph)
	}

	page.Println("q")
	page.Println("1 .5 .5 RG")
	yPos := height - margin - float64(F1.Ascent)/1000
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
		bbox := F1.GlyphExtents[c]
		if !bbox.IsZero() {
			x := xPos + float64(gl.XOffset+bbox.LLx)
			y := yPos + float64(gl.YOffset+bbox.LLy)
			w := float64(bbox.URx - bbox.LLx)
			h := float64(bbox.URy - bbox.LLy)
			page.Printf("%.2f %.2f %.2f %.2f re\n", x, y, w, h)
			page.Printf("%.2f %.2f %.2f %.2f re\n", x, y-baseLineSkip, w, h)
		}
		xPos += float64(gl.Advance)
	}
	page.Println("s")
	page.Println("Q")

	box := boxes.Text(F1, fontSize, text)
	box.Draw(page, margin, yPos-baseLineSkip)

	xPos = margin
	layout.Draw(page, xPos, yPos)

	return page.Close()
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
