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
	"strings"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/simple"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
)

const (
	fontSize = 48.0
)

func main() {
	out, err := pdf.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}

	const width = 8 * 72
	const height = 6 * 72

	text := "Ba\u0308rfisch"
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

func writePage(out *pdf.Writer, text string, width, height float64) error {
	fontFile := "../../sfnt/otf/SourceSerif4-Regular.otf"
	var F1 *font.Font
	var err error
	if strings.HasSuffix(fontFile, ".ttf") || strings.HasSuffix(fontFile, ".otf") {
		F1, err = simple.EmbedFile(out, fontFile, "F1", language.AmericanEnglish)
	} else {
		F1, err = builtin.Embed(out, fontFile, "F1")
	}
	if err != nil {
		return err
	}

	pageTree := pages.NewTree(out, nil)

	g, err := graphics.NewPage(out)
	if err != nil {
		return err
	}

	glyphs := F1.Typeset(text, fontSize)
	for _, glyph := range glyphs {
		fmt.Printf("%q %v\n", string(glyph.Text), glyph)
	}

	q := fontSize / float64(F1.UnitsPerEm)

	margin := 50.0
	baseLineSkip := 1.2 * fontSize
	xPos := margin
	yPos := height - margin - F1.Ascent.AsFloat(q)

	// draw red horizontal rules
	g.PushGraphicsState()
	g.SetStrokeRGB(1, .5, .5)
	for y := yPos; y > margin; y -= baseLineSkip {
		g.MoveTo(margin, y)
		g.LineTo(width-margin, y)
	}
	g.Stroke()
	g.PopGraphicsState()

	g.PushGraphicsState()
	g.SetStrokeRGB(.2, 1, .2)
	for _, gl := range glyphs {
		c := gl.Gid
		rect := F1.GlyphExtents[c]
		bbox := &pdf.Rectangle{
			LLx: rect.LLx.AsFloat(q),
			LLy: rect.LLy.AsFloat(q),
			URx: rect.URx.AsFloat(q),
			URy: rect.URy.AsFloat(q),
		}
		if !bbox.IsZero() {
			x := xPos + gl.XOffset.AsFloat(q) + bbox.LLx
			y := yPos + gl.YOffset.AsFloat(q) + bbox.LLy
			w := bbox.URx - bbox.LLx
			h := bbox.URy - bbox.LLy
			g.Rectangle(x, y, w, h)
		}
		xPos += gl.Advance.AsFloat(q)
	}
	g.Stroke()
	g.PopGraphicsState()

	xPos = margin
	g.BeginText()
	g.SetFont(F1, fontSize)
	g.StartLine(xPos, yPos)
	g.ShowGlyphsAligned(glyphs, 0, 0)

	dict, err := g.Close()
	if err != nil {
		return err
	}

	dict["MediaBox"] = &pdf.Rectangle{
		URx: width,
		URy: height,
	}
	_, err = pageTree.AppendPage(dict)
	if err != nil {
		return err
	}

	ref, err := pageTree.Close()
	if err != nil {
		return err
	}
	out.Catalog.Pages = ref
	return nil
}
