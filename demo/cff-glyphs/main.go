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

// Read a CFF font and display a magnified version of each glyph
// in a PDF file.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/boxes"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/pages"
)

var q float64 = 0.4

func main() {
	flag.Parse()

	out, err := pdf.Create("out.pdf")
	if err != nil {
		log.Fatal(err)
	}

	F, err := builtin.Embed(out, "Courier", "F")
	if err != nil {
		log.Fatal(err)
	}

	tree := pages.NewPageTree(out, &pages.DefaultAttributes{
		Resources: &pages.Resources{
			Font: pdf.Dict{
				F.InstName: F.Ref,
			},
		},
	})

	names := flag.Args()
	for _, fname := range names {
		cffData, err := loadCFFData(fname)
		if err != nil {
			log.Printf("%s: %v", fname, err)
			continue
		}

		cffFont, err := cff.Read(bytes.NewReader(cffData))
		if err != nil {
			log.Printf("%s: %v", fname, err)
			continue
		}

		X, err := cff.EmbedFontCID(out, cffFont, "X")
		if err != nil {
			log.Printf("%s: %v", fname, err)
			continue
		}

		for i := range cffFont.Glyphs {
			bbox := cffFont.Glyphs[i].Extent()
			left := int16(0)
			if bbox.LLx < left {
				left = bbox.LLx
			}
			right := int16(cffFont.Glyphs[i].Width)
			if right < 300 {
				right = 300
			}
			if bbox.URx > right {
				right = bbox.URx
			}
			top := int16(100)
			if bbox.URy > top {
				top = bbox.URy
			}
			bottom := int16(0)
			if bbox.LLy < bottom {
				bottom = bbox.LLy
			}

			page, err := tree.NewPage(&pages.Attributes{
				MediaBox: &pdf.Rectangle{
					LLx: q*float64(left) - 20,
					LLy: q*float64(bottom) - 20,
					URx: q*float64(right) + 20,
					URy: q*float64(top) + 12 + 20,
				},
				Resources: &pages.Resources{
					Font: pdf.Dict{
						X.InstName: X.Ref,
					},
				},
			})
			if err != nil {
				log.Fatal(err)
			}

			err = illustrateGlyph(page, F, X, cffFont, i)
			if err != nil {
				log.Fatal(err)
			}

			err = page.Close()
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func loadCFFData(fname string) ([]byte, error) {
	if strings.HasSuffix(fname, ".otf") {
		f, err := sfnt.Open(fname, nil)
		if err != nil {
			return nil, err
		}
		return f.Header.ReadTableBytes(f.Fd, "CFF ")
	}
	return os.ReadFile(fname)
}

func illustrateGlyph(page *pages.Page, F, X *font.Font, fnt *cff.Font, i int) error {
	hss := boxes.Glue(0, 1, 1, 1, 1)

	label := fmt.Sprintf("glyph %d: %s", i, fnt.Glyphs[i].Name)
	nameBox := boxes.Text(F, 12, label)
	titleBox := boxes.HBoxTo(page.BBox.URx-page.BBox.LLx, hss, nameBox, hss)
	titleBox.Draw(page, page.BBox.LLx, page.BBox.URy-20)

	page.Printf("%.3f 0 0 %.3f 0 0 cm\n", q, q)

	w := fnt.Glyphs[i].Width
	page.Println("q")
	page.Println("0.1 0.9 0.1 RG 3 w")
	page.Println("0 -10 m 0 10 l")
	page.Printf("0 0 m %d 0 l\n", w)
	page.Printf("%d -10 m %d 0 l %d 10 l\n", w-10, w, w-10)
	page.Println("S Q")

	glyph := fnt.Glyphs[i]

	page.Println("q")
	page.Println("0.5 0.9 0.9 rg")
	glyphImage := &font.Layout{
		Font:     X,
		FontSize: 1000,
		Glyphs: []font.Glyph{
			{
				Gid:     font.GlyphID(i),
				Advance: int32(fnt.Glyphs[i].Width),
			},
		},
	}
	glyphImage.Draw(page, 0, 0)
	page.Println("Q")

	var xx []float64
	var yy []float64
	var ink bool
	for _, cmd := range glyph.Cmds {
		switch cmd.Op {
		case cff.OpMoveTo:
			if ink {
				page.Println("h")
			}
			page.Printf("%.3f %.3f m\n", cmd.Args[0], cmd.Args[1])
			xx = append(xx, cmd.Args[0])
			yy = append(yy, cmd.Args[1])
		case cff.OpLineTo:
			page.Printf("%.3f %.3f l\n", cmd.Args[0], cmd.Args[1])
			xx = append(xx, cmd.Args[0])
			yy = append(yy, cmd.Args[1])
			ink = true
		case cff.OpCurveTo:
			page.Printf("%.3f %.3f %.3f %.3f %.3f %.3f c\n",
				cmd.Args[0], cmd.Args[1], cmd.Args[2], cmd.Args[3], cmd.Args[4], cmd.Args[5])
			xx = append(xx, cmd.Args[4])
			yy = append(yy, cmd.Args[5])
			ink = true
		}
	}
	if ink {
		page.Println("h")
	}
	page.Println("S")

	page.Println("q 0 0 0.8 rg")
	for i := range xx {
		x := xx[i]
		y := yy[i]
		label := boxes.Text(F, 16, fmt.Sprintf("%d", i))
		label.Draw(page, x, y)
	}
	page.Println("Q")

	return nil
}
