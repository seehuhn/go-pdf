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

		cff, err := cff.Read(bytes.NewReader(cffData))
		if err != nil {
			log.Printf("%s: %v", fname, err)
			continue
		}

		for i := range cff.GlyphName {
			bbox := cff.GlyphExtent[i]
			left := 0
			if bbox.LLx < left {
				left = bbox.LLx
			}
			right := cff.Width[i]
			if right < 300 {
				right = 300
			}
			if bbox.URx > right {
				right = bbox.URx
			}
			top := 100
			if bbox.URy > top {
				top = bbox.URy
			}
			bottom := 0
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
			})
			if err != nil {
				log.Fatal(err)
			}

			err = illustrateGlyph(page, F, cff, i)
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

type context struct {
	page       *pages.Page
	xx         []float64
	yy         []float64
	posX, posY float64
	w          int32
	ink        bool
}

func (ctx *context) SetWidth(w int32) {
	ctx.w = w
}

func (ctx *context) MoveTo(x, y float64) {
	if ctx.ink {
		ctx.page.Println("h")
	}
	ctx.posX = x
	ctx.posY = y
	ctx.page.Printf("%.2f %.2f m\n", ctx.posX, ctx.posY)
	ctx.xx = append(ctx.xx, ctx.posX)
	ctx.yy = append(ctx.yy, ctx.posY)
}

func (ctx *context) LineTo(x, y float64) {
	ctx.ink = true
	ctx.posX = x
	ctx.posY = y
	ctx.page.Printf("%.2f %.2f l\n", ctx.posX, ctx.posY)
	ctx.xx = append(ctx.xx, ctx.posX)
	ctx.yy = append(ctx.yy, ctx.posY)
}

func (ctx *context) CurveTo(xa, ya, xb, yb, xc, yc float64) {
	ctx.ink = true
	ctx.page.Printf("%.2f %.2f %.2f %.2f %.2f %.2f c\n",
		xa, ya, xb, yb, xc, yc)
	ctx.posX = xc
	ctx.posY = yc
	ctx.xx = append(ctx.xx, ctx.posX)
	ctx.yy = append(ctx.yy, ctx.posY)
}

func illustrateGlyph(page *pages.Page, F *font.Font, cff *cff.Font, i int) error {
	hss := boxes.Glue(0, 1, 1, 1, 1)

	label := fmt.Sprintf("glyph %d: %s", i, cff.GlyphName[i])
	nameBox := boxes.Text(F, 12, label)
	titleBox := boxes.HBoxTo(page.BBox.URx-page.BBox.LLx, hss, nameBox, hss)
	titleBox.Draw(page, page.BBox.LLx, page.BBox.URy-20)

	page.Printf("%.3f 0 0 %.3f 0 0 cm\n", q, q)

	page.Println("q")
	page.Println("0.1 0.9 0.1 RG 3 w")
	page.Println("0 -10 m 0 10 l")
	page.Printf("0 0 m %d 0 l\n", cff.Width[i])
	page.Printf("%d -10 m %d 0 l %d 10 l\n",
		cff.Width[i]-10, cff.Width[i], cff.Width[i]-10)
	page.Println("S Q")

	ctx := &context{
		page: page,
	}
	err := cff.DecodeCharString(ctx, i)
	if ctx.ink {
		page.Println("h")
	}
	page.Println("S")
	if err != nil {
		return err
	}

	page.Println("q 0 0 0.8 rg")
	for i := range ctx.xx {
		x := ctx.xx[i]
		y := ctx.yy[i]
		label := boxes.Text(F, 16, fmt.Sprintf("%d", i))
		label.Draw(page, x, y)
	}
	page.Println("Q")

	return nil
}
