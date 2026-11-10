// Copyright 2021 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
)

func main() {
	out, err := pdf.Create("test.pdf", pdf.V1_7, nil)
	if err != nil {
		log.Fatal(err)
	}

	rm := pdf.NewResourceManager(out)

	font := font.Must(standard.Helvetica.New())

	mediaBox := &pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 200}
	pageTree := pagetree.NewWriter(out, rm)

	frontMatter, err := pageTree.NewRange()
	if err != nil {
		log.Fatal(err)
	}
	var extra *pagetree.Writer
	for i := 1; i <= 99; i++ {
		if i == 3 {
			extra, err = pageTree.NewRange()
			if err != nil {
				log.Fatal(err)
			}
		}

		g := builder.New(content.Page, nil, pdf.V2_0)

		g.TextBegin()
		g.TextSetFont(font, 12)
		g.TextFirstLine(30, 30)
		if i < 3 {
			g.TextShow(fmt.Sprintf("page %d", i))
		} else {
			g.TextShow(fmt.Sprintf("page %d", i+1))
		}
		g.TextEnd()

		p := &page.Page{
			MediaBox:  mediaBox,
			Resources: g.Resources,
			Contents:  []page.Segment{&content.Operators{Ops: g.Stream}},
		}
		err = pageTree.AppendPage(p)
		if err != nil {
			log.Fatal(err)
		}
	}

	{
		g := builder.New(content.Page, nil, pdf.V2_0)

		g.TextBegin()
		g.TextSetFont(font, 12)
		g.TextFirstLine(30, 30)
		g.TextShow("Title")
		g.TextEnd()

		p := &page.Page{
			MediaBox:  mediaBox,
			Resources: g.Resources,
			Contents:  []page.Segment{&content.Operators{Ops: g.Stream}},
		}
		err = frontMatter.AppendPage(p)
		if err != nil {
			log.Fatal(err)
		}
	}

	{
		g := builder.New(content.Page, nil, pdf.V2_0)

		g.TextBegin()
		g.TextSetFont(font, 12)
		g.TextFirstLine(30, 30)
		g.TextShow("three")
		g.TextEnd()

		p := &page.Page{
			MediaBox:  mediaBox,
			Resources: g.Resources,
			Contents:  []page.Segment{&content.Operators{Ops: g.Stream}},
		}
		err = extra.AppendPage(p)
		if err != nil {
			log.Fatal(err)
		}
	}

	ref, err := pageTree.Close()
	if err != nil {
		log.Fatal(err)
	}
	out.GetMeta().Catalog.Pages = ref

	out.GetMeta().Catalog.PageLabels = pdf.Dict{
		"Nums": pdf.Array{
			pdf.Integer(0),
			pdf.Dict{
				"P": pdf.Text("Title"),
			},
			pdf.Integer(1),
			pdf.Dict{
				"S": pdf.Name("D"),
			},
		},
	}

	err = rm.Close()
	if err != nil {
		log.Fatal(err)
	}

	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}
