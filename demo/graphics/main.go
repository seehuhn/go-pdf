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

package main

import (
	"log"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
)

func main() {
	w, err := pdf.Create("graphics.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	F, err := builtin.Embed(w, builtin.Helvetica, "F")
	if err != nil {
		log.Fatal(err)
	}

	pageTree := pages.NewTree(w, nil)

	bbox := pages.A4
	g, err := graphics.NewPage(w)
	if err != nil {
		log.Fatal(err)
	}

	x := 72.0
	y := bbox.URy - 72.0
	r := 50.0
	g.Circle(x, y, r)
	g.Stroke()

	x += 120
	g.MoveTo(x, y)
	g.LineToArc(x, y, r, 0, 1.5*math.Pi)
	g.CloseAndStroke()

	x = 72
	y -= 72
	g.BeginText()
	g.SetFont(F, 12)
	g.StartLine(x, y)
	g.ShowText("AWAY again")
	g.StartNextLine(0, -15)
	g.ShowText("line 2")
	g.NewLine()
	g.ShowText("line 3")
	g.EndText()

	dict, err := g.Close()
	if err != nil {
		log.Fatal(err)
	}
	dict["MediaBox"] = bbox
	_, err = pageTree.AppendPage(dict)
	if err != nil {
		log.Fatal(err)
	}

	ref, err := pageTree.Close()
	if err != nil {
		log.Fatal(err)
	}
	w.Catalog.Pages = ref
}
