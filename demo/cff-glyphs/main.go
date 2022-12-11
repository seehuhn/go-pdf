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
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages2"
	"seehuhn.de/go/pdf/sfnt/cff"
	"seehuhn.de/go/pdf/sfnt/funit"
	"seehuhn.de/go/pdf/sfnt/header"
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

	tree := pages2.NewTree(out, &pages2.InheritableAttributes{
		Resources: &pdf.Resources{
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

		for i := range cffFont.Glyphs {
			glyphBBox := cffFont.Glyphs[i].Extent()
			left := funit.Int16(0)
			if glyphBBox.LLx < left {
				left = glyphBBox.LLx
			}
			right := cffFont.Glyphs[i].Width
			if right < 300 {
				right = 300
			}
			if glyphBBox.URx > right {
				right = glyphBBox.URx
			}
			top := funit.Int16(100)
			if glyphBBox.URy > top {
				top = glyphBBox.URy
			}
			bottom := funit.Int16(0)
			if glyphBBox.LLy < bottom {
				bottom = glyphBBox.LLy
			}
			pageBBox := &pdf.Rectangle{
				LLx: q*float64(left) - 20,
				LLy: q*float64(bottom) - 20,
				URx: q*float64(right) + 20,
				URy: q*float64(top) + 12 + 20,
			}

			page, err := graphics.NewPage(out)
			if err != nil {
				log.Fatal(err)
			}

			ctx := &context{
				page:      page,
				pageBBox:  pageBBox,
				labelFont: F,
				labelSize: 12,
			}
			err = illustrateGlyph(ctx, cffFont, i)
			if err != nil {
				log.Fatal(err)
			}

			dict, err := page.Close()
			if err != nil {
				log.Fatal(err)
			}
			dict["MediaBox"] = pageBBox

			_, err = tree.AppendPage(dict)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	rootRef, err := tree.Close()
	if err != nil {
		log.Fatal(err)
	}
	out.Catalog.Pages = rootRef

	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func loadCFFData(fname string) ([]byte, error) {
	if strings.HasSuffix(fname, ".cff") {
		return os.ReadFile(fname)
	}

	r, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	header, err := header.Read(r)
	if err != nil {
		return nil, err
	}

	cffData, err := header.ReadTableBytes(r, "CFF ")
	if err != nil {
		return nil, err
	}
	return cffData, nil
}

type context struct {
	page      *graphics.Page
	pageBBox  *pdf.Rectangle
	labelFont *font.Font
	labelSize float64
}

func illustrateGlyph(ctx *context, fnt *cff.Font, i int) error {
	page := ctx.page

	hss := boxes.Glue(0, 1, 1, 1, 1)

	var label string
	if fnt.Glyphs[i].Name != "" {
		label = fmt.Sprintf("glyph %d %q", i, fnt.Glyphs[i].Name)
	} else {
		label = fmt.Sprintf("glyph %d", i)
	}
	nameBox := boxes.Text(ctx.labelFont, ctx.labelSize, label)
	titleBox := boxes.HBoxTo(ctx.pageBBox.URx-ctx.pageBBox.LLx, hss, nameBox, hss)
	titleBox.Draw(page, ctx.pageBBox.LLx, ctx.pageBBox.URy-20)

	page.Scale(q, q)

	// illustrate the advance width by drawing an arrow
	w := fnt.Glyphs[i].Width
	page.PushGraphicsState()
	page.SetStrokeRGB(0.1, 0.9, 0.1)
	page.SetLineWidth(3)
	page.MoveTo(0, -10)
	page.LineTo(0, 10)
	page.MoveTo(0, 0)
	page.LineTo(float64(w), 0)
	page.MoveTo(float64(w)-10, -10)
	page.LineTo(float64(w), 0)
	page.LineTo(float64(w-10), 10)
	page.Stroke()
	page.PopGraphicsState()

	// draw the glyph outline
	var xx []cff.Fixed16
	var yy []cff.Fixed16
	glyph := fnt.Glyphs[i]
	if len(glyph.Cmds) > 0 {
		var ink bool
		for _, cmd := range glyph.Cmds {
			switch cmd.Op {
			case cff.OpMoveTo:
				if ink {
					page.ClosePath()
				}
				page.MoveTo(cmd.Args[0].Float64(), cmd.Args[1].Float64())
				xx = append(xx, cmd.Args[0])
				yy = append(yy, cmd.Args[1])
			case cff.OpLineTo:
				page.LineTo(cmd.Args[0].Float64(), cmd.Args[1].Float64())
				xx = append(xx, cmd.Args[0])
				yy = append(yy, cmd.Args[1])
				ink = true
			case cff.OpCurveTo:
				page.CurveTo(cmd.Args[0].Float64(), cmd.Args[1].Float64(),
					cmd.Args[2].Float64(), cmd.Args[3].Float64(),
					cmd.Args[4].Float64(), cmd.Args[5].Float64())
				xx = append(xx, cmd.Args[4])
				yy = append(yy, cmd.Args[5])
				ink = true
			}
		}
		if ink {
			page.ClosePath()
		}
		page.Stroke()
	}

	page.PushGraphicsState()
	page.SetFillRGB(0, 0, 0.8)
	for i := range xx {
		x := xx[i]
		y := yy[i]
		label := boxes.Text(ctx.labelFont, 16, fmt.Sprintf("%d", i))
		label.Draw(page, x.Float64(), y.Float64())
	}
	page.PopGraphicsState()

	return nil
}
