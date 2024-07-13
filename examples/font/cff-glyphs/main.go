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
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/runenames"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/header"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	pdft1 "seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/matrix"
	"seehuhn.de/go/pdf/pagetree"
)

const (
	q            = 0.3
	topMargin    = 24 + 3*12
	rightMargin  = 24
	bottomMargin = 24
	leftMargin   = 24
)

func main() {
	fileNames := os.Args[1:]
	if len(fileNames) == 0 {
		fmt.Fprintf(os.Stderr, "usage: %s font.ttf font.otf ...\n",
			filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	out, err := pdf.Create("out.pdf", pdf.V1_7, nil)
	if err != nil {
		log.Fatal(err)
	}

	labelFont, err := pdft1.Courier.Embed(out, nil)
	if err != nil {
		log.Fatal(err)
	}

	tree := pagetree.NewWriter(out)
	rm := graphics.NewResourceManager(out)

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
			LLx: fontBBox.LLx.AsFloat(q) - leftMargin,
			LLy: fontBBox.LLy.AsFloat(q) - bottomMargin,
			URx: fontBBox.URx.AsFloat(q) + rightMargin,
			URy: fontBBox.URy.AsFloat(q) + topMargin,
		}

		subTree, err := tree.NewRange()
		if err != nil {
			log.Fatal(err)
		}

		ctx := &illustrator{
			labelFont: labelFont,
			pageTree:  subTree,
			pageSize:  pageSize,
			rm:        rm,
		}

		err = ctx.Show(cffFont, pageSize)
		if err != nil {
			log.Fatal(err)
		}
	}

	ref, err := tree.Close()
	if err != nil {
		log.Fatal(err)
	}
	out.GetMeta().Catalog.Pages = ref

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

func getFontBBox(fnt *cff.Font) *funit.Rect16 {
	bbox := &funit.Rect16{}
	for _, g := range fnt.Glyphs {
		bbox.Extend(g.Extent())
	}
	return bbox
}

type illustrator struct {
	labelFont font.Embedded
	pageTree  *pagetree.Writer
	pageSize  *pdf.Rectangle
	rm        *graphics.ResourceManager
}

func (ctx *illustrator) Show(fnt *cff.Font, pageSize *pdf.Rectangle) error {
	codes := make(map[glyph.ID][]int)
	if fnt.Encoding != nil {
		for code, gid := range fnt.Encoding {
			if gid == 0 {
				continue
			}
			codes[gid] = append(codes[gid], code)
		}
	}
	CIDs := make(map[glyph.ID]cid.CID)
	if fnt.GIDToCID != nil {
		for gid, cid := range fnt.GIDToCID {
			CIDs[glyph.ID(gid)] = cid
		}
	}

	for i, g := range fnt.Glyphs {
		contentRef := ctx.pageTree.Out.Alloc()
		stream, err := ctx.pageTree.Out.OpenStream(contentRef, nil, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		page := graphics.NewWriter(stream, ctx.rm)

		// show the glyph extent as a shaded rectangle
		bbox := g.Extent()
		x0 := bbox.LLx.AsFloat(q)
		y0 := bbox.LLy.AsFloat(q)
		x1 := bbox.URx.AsFloat(q)
		y1 := bbox.URy.AsFloat(q)
		page.PushGraphicsState()
		page.SetFillColor(color.DeviceGray.New(0.9))
		page.Rectangle(x0, y0, x1-x0, y1-y0)
		page.Fill()
		page.PopGraphicsState()

		// show the glyph ID and name
		var label []string
		label = append(label, fmt.Sprintf("glyph %d", i))
		if g.Name != "" {
			label = append(label, fmt.Sprintf("%q", g.Name))
		}
		if cc := codes[glyph.ID(i)]; len(cc) > 0 {
			var ccString []string
			for _, c := range cc {
				ccString = append(ccString, fmt.Sprintf("%d", c))
			}
			label = append(label, fmt.Sprintf("code=%s", strings.Join(ccString, ",")))
		}
		if cid, ok := CIDs[glyph.ID(i)]; ok {
			label = append(label, fmt.Sprintf("CID=%d", cid))
		}

		page.TextBegin()
		page.TextSetFont(ctx.labelFont, 12)
		page.TextFirstLine(ctx.pageSize.LLx+22, ctx.pageSize.URy-30)
		page.TextShow(strings.Join(label, ", "))
		if g.Name != "" {
			rr := names.ToUnicode(g.Name, false)
			if len(rr) == 1 {
				runeName := runenames.Name(rr[0])
				page.TextSecondLine(0, -15)
				page.TextShow(runeName)
			}
		}
		page.TextEnd()

		page.Transform(matrix.Scale(q, q))

		// illustrate the advance width by drawing an arrow
		page.PushGraphicsState()
		page.SetStrokeColor(color.DeviceRGB.New(0.1, 0.9, 0.1))
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
			var xx []float64
			var yy []float64

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
					page.MoveTo(cmd.Args[0], cmd.Args[1])
					xx = append(xx, cmd.Args[0])
					yy = append(yy, cmd.Args[1])
				case cff.OpLineTo:
					page.LineTo(cmd.Args[0], cmd.Args[1])
					xx = append(xx, cmd.Args[0])
					yy = append(yy, cmd.Args[1])
					ink = true
				case cff.OpCurveTo:
					page.CurveTo(cmd.Args[0], cmd.Args[1],
						cmd.Args[2], cmd.Args[3],
						cmd.Args[4], cmd.Args[5])
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
			page.SetFillColor(color.DeviceRGB.New(0, 0, 0.8))
			page.TextBegin()
			page.TextSetFont(ctx.labelFont, 8/q)
			xPrev := 0.0
			yPrev := 0.0
			for i := range xx {
				x := xx[i]
				y := yy[i] - 2
				dx := x - xPrev
				dy := y - yPrev
				page.TextFirstLine(dx, dy)
				page.TextShowAligned(fmt.Sprintf("%d", i), 0, 0.5)
				xPrev = x
				yPrev = y
			}
			page.TextEnd()
			page.PopGraphicsState()
		}

		err = stream.Close()
		if err != nil {
			return err
		}

		pageDict := pdf.Dict{
			"Type":     pdf.Name("Page"),
			"Contents": contentRef,
			"MediaBox": pageSize,
		}
		if page.Resources != nil {
			pageDict["Resources"] = pdf.AsDict(page.Resources)
		}
		err = ctx.pageTree.AppendPage(pageDict)
		if err != nil {
			return err
		}
	}
	return nil
}
