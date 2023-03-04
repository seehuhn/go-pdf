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
	"seehuhn.de/go/pdf/pages"
)

func main() {
	out, err := pdf.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}

	font, err := builtin.Embed(out, builtin.Helvetica, "F1")
	if err != nil {
		log.Fatal(err)
	}

	pageTree := pages.InstallTree(out, &pages.InheritableAttributes{
		MediaBox: &pdf.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 200},
	})
	frontMatter, err := pageTree.NewSubTree(nil)
	if err != nil {
		log.Fatal(err)
	}
	var extra *pages.Tree
	for i := 1; i <= 99; i++ {
		if i == 3 {
			extra, err = pageTree.NewSubTree(nil)
			if err != nil {
				log.Fatal(err)
			}
		}

		g, err := pages.NewPage(out)
		if err != nil {
			log.Fatal(err)
		}

		g.BeginText()
		g.SetFont(font, 12)
		g.StartLine(30, 30)
		if i < 3 {
			g.ShowText(fmt.Sprintf("page %d", i))
		} else {
			g.ShowText(fmt.Sprintf("page %d", i+1))
		}
		g.EndText()

		dict, err := g.Close()
		if err != nil {
			log.Fatal(err)
		}
		_, err = pageTree.AppendPage(dict)
		if err != nil {
			log.Fatal(err)
		}
	}

	{
		g, err := pages.NewPage(out)
		if err != nil {
			log.Fatal(err)
		}

		g.BeginText()
		g.SetFont(font, 12)
		g.StartLine(30, 30)
		g.ShowText("Title")
		g.EndText()

		dict, err := g.Close()
		if err != nil {
			log.Fatal(err)
		}
		_, err = frontMatter.AppendPage(dict)
		if err != nil {
			log.Fatal(err)
		}
	}

	{
		g, err := pages.NewPage(out)
		if err != nil {
			log.Fatal(err)
		}

		g.BeginText()
		g.SetFont(font, 12)
		g.StartLine(30, 30)
		g.ShowText("three")
		g.EndText()

		dict, err := g.Close()
		if err != nil {
			log.Fatal(err)
		}
		_, err = extra.AppendPage(dict)
		if err != nil {
			log.Fatal(err)
		}
	}

	out.Catalog.PageLabels = pdf.Dict{
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
