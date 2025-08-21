// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"math"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	paper := document.A4
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	F := standard.TimesRoman.New()
	I := standard.TimesItalic.New()

	xMid := paper.LLx + 0.5*paper.Dx()
	x0 := math.Round(paper.LLx + 36.0)
	x1 := math.Round(xMid - 18.0)
	x2 := math.Round(xMid + 18.0)
	x3 := x2 + x1 - x0
	yTop := paper.URy - 72.0

	textCol := color.DeviceGray(0)
	linkCol := color.DeviceRGB(0, 0, 0.9)

	page.TextBegin()

	page.TextFirstLine(x0, yTop)
	page.TextSetWordSpacing(0.967)
	page.TextSetFont(F, 10)
	page.TextShow("In the Middle Ages, a quire (also called a “")
	page.SetFillColor(linkCol)
	page.TextShow("gathering") // https://en.wikipedia.org/wiki/Gathering_(bookbinding)
	page.SetFillColor(textCol)
	page.TextShow("”) was")

	page.TextSecondLine(0, -13.0)
	page.TextSetWordSpacing(0.75)
	page.TextShow("most often formed of four folded sheets of ")
	page.SetFillColor(linkCol)
	page.TextShow("vellum") // https://en.wikipedia.org/wiki/Vellum
	page.SetFillColor(textCol)
	page.TextShow(" or ")
	page.SetFillColor(linkCol)
	page.TextShow("parch-") // https://en.wikipedia.org/wiki/Parchment

	page.TextNextLine()
	page.TextSetWordSpacing(-0.328)
	page.TextShow("ment") // cont.
	page.SetFillColor(textCol)
	page.TextShow(", i.e. eight leaves or ")
	page.SetFillColor(linkCol)
	page.TextShow("folios") // https://en.wikipedia.org/wiki/Folio
	page.SetFillColor(textCol)
	page.TextShow(", 16 sides. The term ")
	page.TextSetFont(I, 10)
	page.TextShow("quaternion")

	page.TextEnd()

	_ = x1
	_ = x2
	_ = x3

	return page.Close()
}
