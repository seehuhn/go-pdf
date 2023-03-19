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
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/text/unicode/runenames"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/funit"
	"seehuhn.de/go/sfnt/header"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
	"seehuhn.de/go/sfnt/type1/names"
)

const (
	q      = 0.4
	margin = 24
)

func main() {
	fileNames := os.Args[1:]
	if len(fileNames) == 0 {
		fmt.Fprintf(os.Stderr, "usage: %s font.ttf font.otf ...\n", os.Args[0])
	}

	out, err := pdf.Create("out.pdf")
	if err != nil {
		log.Fatal(err)
	}

	labelFont, err := builtin.Embed(out, "Courier", "F")
	if err != nil {
		log.Fatal(err)
	}

	tree := pages.NewTree(out, nil)

	for _, fname := range fileNames {
		cffData, err := loadCFFData(fname)
		if err != nil {
			log.Printf("%s: %v", fname, err)
			continue
		}
		cffFont, err := cff.Read(bytes.NewReader(cffData))
		if err != nil {
			log.Fatal(err)
		}

		fontBBox := getFontBBox(cffFont)
		pageSize := &pdf.Rectangle{
			LLx: fontBBox.LLx.AsFloat(q) - margin,
			LLy: fontBBox.LLy.AsFloat(q) - margin,
			URx: fontBBox.URx.AsFloat(q) + margin,
			URy: fontBBox.URy.AsFloat(q) + margin,
		}

		subTree, err := tree.NewSubTree(&pages.InheritableAttributes{
			MediaBox: pageSize,
		})
		if err != nil {
			log.Fatal(err)
		}

		ctx := &illustrator{
			labelFont: labelFont,
			pageTree:  subTree,
			pageSize:  pageSize,
		}

		err = ctx.Show(cffFont)
		if err != nil {
			log.Fatal(err)
		}
	}

	ref, err := tree.Close()
	if err != nil {
		log.Fatal(err)
	}
	out.Catalog.Pages = ref

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

func getFontBBox(fnt *cff.Font) *funit.Rect {
	bbox := &funit.Rect{}
	for _, g := range fnt.Glyphs {
		bbox.Extend(g.Extent())
	}
	return bbox
}

type illustrator struct {
	labelFont font.Embedded
	pageTree  *pages.Tree
	pageSize  *pdf.Rectangle
}

func (ctx *illustrator) Show(fnt *cff.Font) error {
	compress := &pdf.FilterInfo{Name: pdf.Name("LZWDecode")}
	if ctx.pageTree.Out.Version >= pdf.V1_2 {
		compress = &pdf.FilterInfo{Name: pdf.Name("FlateDecode")}
	}

	for i, g := range fnt.Glyphs {
		stream, contentRef, err := ctx.pageTree.Out.OpenStream(nil, nil, compress)
		if err != nil {
			return err
		}
		page := graphics.NewPage(stream)

		// show the glyph extent as a shaded rectangle
		bbox := g.Extent()
		x0 := bbox.LLx.AsFloat(q)
		y0 := bbox.LLy.AsFloat(q)
		x1 := bbox.URx.AsFloat(q)
		y1 := bbox.URy.AsFloat(q)
		page.PushGraphicsState()
		page.SetFillColor(color.Gray(0.9))
		page.Rectangle(x0, y0, x1-x0, y1-y0)
		page.Fill()
		page.PopGraphicsState()

		// show the glyph ID and name
		var label string
		if g.Name != "" {
			label = fmt.Sprintf("glyph %d %q", i, g.Name)
		} else {
			label = fmt.Sprintf("glyph %d", i)
		}
		page.BeginText()
		page.SetFont(ctx.labelFont, 12)
		page.StartLine(ctx.pageSize.LLx+22, ctx.pageSize.URy-30)
		page.ShowText(label)
		if g.Name != "" {
			rr := names.ToUnicode(g.Name, false)
			if len(rr) == 1 {
				runeName := runenames.Name(rr[0])
				page.StartNextLine(0, -15)
				page.ShowText(runeName)
			}
		}
		page.EndText()

		page.Scale(q, q)

		// illustrate the advance width by drawing an arrow
		page.PushGraphicsState()
		page.SetStrokeColor(color.RGB(0.1, 0.9, 0.1))
		page.SetLineWidth(3)
		page.MoveTo(0, -10)
		page.LineTo(0, 10)
		page.MoveTo(0, 0)
		w := float64(g.Width)
		page.LineTo(w, 0)
		page.MoveTo(w-10, -10)
		page.LineTo(w, 0)
		page.LineTo(w-10, 10)
		page.Stroke()
		page.PopGraphicsState()

		if len(g.Cmds) > 0 {
			var xx []cff.Fixed16
			var yy []cff.Fixed16

			// draw the glyph outline
			var ink bool
			page.PushGraphicsState()
			page.SetLineWidth(1 / q)
			for _, cmd := range g.Cmds {
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
			page.PopGraphicsState()

			// label the points
			page.PushGraphicsState()
			page.SetFillColor(color.RGB(0, 0, 0.8))
			page.BeginText()
			page.SetFont(ctx.labelFont, 8/q)
			xPrev := 0.0
			yPrev := 0.0
			for i := range xx {
				x := xx[i].Float64()
				y := yy[i].Float64() - 2
				dx := x - xPrev
				dy := y - yPrev
				page.StartLine(dx, dy)
				page.ShowTextAligned(fmt.Sprintf("%d", i), 0, 0.5)
				xPrev = x
				yPrev = y
			}
			page.EndText()
			page.PopGraphicsState()
		}

		err = stream.Close()
		if err != nil {
			return err
		}

		pageDict := pdf.Dict{
			"Type":     pdf.Name("Page"),
			"Contents": contentRef,
		}
		if page.Resources != nil {
			pageDict["Resources"] = pdf.AsDict(page.Resources)
		}
		_, err = ctx.pageTree.AppendPage(pageDict, nil)
		if err != nil {
			return err
		}
	}
	return nil
}
