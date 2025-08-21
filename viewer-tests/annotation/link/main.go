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

	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
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

	w := &writer{
		Page:    page.Writer,
		Roman:   standard.TimesRoman.New(),
		Italic:  standard.TimesItalic.New(),
		TextCol: color.DeviceGray(0.1),
		LinkCol: color.DeviceRGB(0, 0, 0.9),

		RM: page.RM,
		BS: &annotation.BorderStyle{
			Width: 1,
			Style: "U",
		},
	}

	xMid := paper.LLx + 0.5*paper.Dx()
	x0 := math.Round(xMid - 244 - 12.0)
	x2 := math.Round(xMid + 12.0)
	yTop := paper.URy - 72.0

	err = w.AddParagraph(x0, yTop)
	if err != nil {
		return err
	}
	err = w.AddParagraph(x2, yTop)
	if err != nil {
		return err
	}

	page.PageDict["Annots"] = w.Annots

	return page.Close()
}

type writer struct {
	Page    *graphics.Writer
	Roman   font.Layouter
	Italic  font.Layouter
	TextCol color.Color
	LinkCol color.Color

	RM     *pdf.ResourceManager
	BS     *annotation.BorderStyle
	Annots pdf.Array
}

// AddParagraph adds a paragraph to the PDF document at the specified position.
// The text width is 244 units.
func (w *writer) AddParagraph(x, y float64) error {
	geom := w.Roman.GetGeometry()

	page := w.Page
	page.TextBegin()

	page.TextFirstLine(x, y)
	page.TextSetWordSpacing(0.967)
	page.TextSetFont(w.Roman, 10)
	page.SetFillColor(w.TextCol)
	page.TextShow("In the Middle Ages, a quire (also called a “")
	page.SetFillColor(w.LinkCol)
	qq := w.MakeLink("gathering")
	err := w.MakeAnnotation("https://en.wikipedia.org/wiki/Gathering_(bookbinding)",
		"Gathering (bookbinding)", qq)
	if err != nil {
		return err
	}
	page.SetFillColor(w.TextCol)
	page.TextShow("”) was")

	page.TextSecondLine(0, -geom.Leading*10)
	page.TextSetWordSpacing(0.750)
	page.TextShow("most often formed of four folded sheets of ")
	page.SetFillColor(w.LinkCol)
	qq = w.MakeLink("vellum")
	err = w.MakeAnnotation("https://en.wikipedia.org/wiki/Vellum",
		"Vellum", qq)
	if err != nil {
		return err
	}
	page.SetFillColor(w.TextCol)
	page.TextShow(" or ")
	page.SetFillColor(w.LinkCol)
	qq = w.MakeLink("parch-")
	err = w.MakeAnnotation("https://en.wikipedia.org/wiki/Parchment",
		"Parchment", qq)
	if err != nil {
		return err
	}

	page.TextNextLine()
	page.TextSetWordSpacing(-0.328)
	qq = w.MakeLink("ment")
	err = w.MakeAnnotation("https://en.wikipedia.org/wiki/Parchment",
		"Parchment", qq)
	if err != nil {
		return err
	}
	page.SetFillColor(w.TextCol)
	page.TextShow(", i.e. eight leaves or ")
	page.SetFillColor(w.LinkCol)
	qq = w.MakeLink("folios")
	err = w.MakeAnnotation("https://en.wikipedia.org/wiki/Folio",
		"Folio", qq)
	if err != nil {
		return err
	}
	page.SetFillColor(w.TextCol)
	page.TextShow(", 16 sides. The term ")
	page.TextSetFont(w.Italic, 10)
	page.TextShow("quaternion")

	page.TextEnd()

	return nil
}

func (w *writer) MakeLink(text string) []float64 {
	page := w.Page

	gg := page.TextLayout(nil, text)
	outline := page.TextGetQuadPoints(gg)
	page.TextShowGlyphs(gg)

	return outline
}

func (w *writer) MakeAnnotation(url string, title string, quadPoints ...[]float64) error {
	var qq []vec.Vec2
	for _, q := range quadPoints {
		// convert float array to Vec2 slice
		for i := 0; i < len(q); i += 2 {
			if i+1 < len(q) {
				qq = append(qq, vec.Vec2{X: q[i], Y: q[i+1]})
			}
		}
	}

	a := pdf.Dict{
		"S":   pdf.Name("URI"),
		"URI": pdf.String(url),
	}

	link := &annotation.Link{
		Common: annotation.Common{
			Contents: title,
			Flags:    annotation.FlagPrint,
			Color:    w.LinkCol,
		},
		Action:      a,
		Highlight:   annotation.LinkHighlightInvert,
		QuadPoints:  qq,
		BorderStyle: w.BS,
	}

	// compute the bounding box from the quad points
	for _, q := range quadPoints {
		for i := 0; i < len(q); i += 2 {
			link.Common.Rect.ExtendVec(vec.Vec2{X: q[i], Y: q[i+1]})
		}
	}

	dict, err := link.Encode(w.RM)
	if err != nil {
		return err
	}
	ref := w.RM.Out.Alloc()
	err = w.RM.Out.Put(ref, dict)
	if err != nil {
		return err
	}
	w.Annots = append(w.Annots, ref)

	return nil
}
