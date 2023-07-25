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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pagetree"
)

func main() {
	out, err := pdf.Create("test.pdf", nil)
	if err != nil {
		log.Fatal(err)
	}

	font, err := builtin.Helvetica.Embed(out, "F")
	if err != nil {
		log.Fatal(err)
	}

	mediaBox := &pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 200}
	pageTree := pagetree.NewWriter(out)

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

		contentRef := out.Alloc()
		stream, err := out.OpenStream(contentRef, nil, pdf.FilterCompress{})
		if err != nil {
			log.Fatal(err)
		}
		g := graphics.NewPage(stream)

		g.TextStart()
		g.TextSetFont(font, 12)
		g.TextFirstLine(30, 30)
		if i < 3 {
			g.TextShow(fmt.Sprintf("page %d", i))
		} else {
			g.TextShow(fmt.Sprintf("page %d", i+1))
		}
		g.TextEnd()

		err = stream.Close()
		if err != nil {
			log.Fatal(err)
		}
		dict := pdf.Dict{
			"Type":     pdf.Name("Page"),
			"Contents": contentRef,
			"MediaBox": mediaBox,
		}
		if g.Resources != nil {
			dict["Resources"] = pdf.AsDict(g.Resources)
		}
		err = pageTree.AppendPage(dict)
		if err != nil {
			log.Fatal(err)
		}
	}

	{
		contentRef := out.Alloc()
		stream, err := out.OpenStream(contentRef, nil, pdf.FilterCompress{})
		if err != nil {
			log.Fatal(err)
		}
		g := graphics.NewPage(stream)

		g.TextStart()
		g.TextSetFont(font, 12)
		g.TextFirstLine(30, 30)
		g.TextShow("Title")
		g.TextEnd()

		err = stream.Close()
		if err != nil {
			log.Fatal(err)
		}
		dict := pdf.Dict{
			"Type":     pdf.Name("Page"),
			"Contents": contentRef,
			"MediaBox": mediaBox,
		}
		if g.Resources != nil {
			dict["Resources"] = pdf.AsDict(g.Resources)
		}
		err = frontMatter.AppendPage(dict)
		if err != nil {
			log.Fatal(err)
		}
	}

	{
		contentRef := out.Alloc()
		stream, err := out.OpenStream(contentRef, nil, pdf.FilterCompress{})
		if err != nil {
			log.Fatal(err)
		}
		g := graphics.NewPage(stream)
		if err != nil {
			log.Fatal(err)
		}

		g.TextStart()
		g.TextSetFont(font, 12)
		g.TextFirstLine(30, 30)
		g.TextShow("three")
		g.TextEnd()

		err = stream.Close()
		if err != nil {
			log.Fatal(err)
		}
		dict := pdf.Dict{
			"Type":     pdf.Name("Page"),
			"Contents": contentRef,
			"MediaBox": mediaBox,
		}
		if g.Resources != nil {
			dict["Resources"] = pdf.AsDict(g.Resources)
		}
		err = extra.AppendPage(dict)
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
				"P": pdf.TextString("Title"),
			},
			pdf.Integer(1),
			pdf.Dict{
				"S": pdf.Name("D"),
			},
		},
	}

	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}
