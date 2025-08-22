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
	"seehuhn.de/go/pdf/annotation/fallback"
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

		style: fallback.NewStyle(),
		RM:    page.RM,
		BS: &annotation.BorderStyle{
			Width: 1,
			Style: "U",
		},
	}

	xMid := paper.LLx + 0.5*paper.Dx()
	x0 := math.Round(xMid - 244 - 12.0)
	x2 := math.Round(xMid + 12.0)
	yTop := paper.URy - 72.0

	err = w.AddParagraph(x0, yTop, false)
	if err != nil {
		return err
	}
	err = w.AddParagraph(x2, yTop, true)
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

	style  *fallback.Style
	RM     *pdf.ResourceManager
	BS     *annotation.BorderStyle
	Annots pdf.Array
}

// AddParagraph adds a paragraph to the PDF document at the specified position.
// The text width is 244 units.
func (w *writer) AddParagraph(x, y float64, withAppearance bool) error {
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
		"Wikipedia: Gathering (bookbinding)", withAppearance, qq)
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
		"Wikipedia: Vellum", withAppearance, qq)
	if err != nil {
		return err
	}
	page.SetFillColor(w.TextCol)
	page.TextShow(" or ")
	page.SetFillColor(w.LinkCol)
	qq = w.MakeLink("parch-")
	err = w.MakeAnnotation("https://en.wikipedia.org/wiki/Parchment",
		"Wikipedia: Parchment", withAppearance, qq)
	if err != nil {
		return err
	}

	page.TextNextLine()
	page.TextSetWordSpacing(-0.333)
	qq = w.MakeLink("ment")
	err = w.MakeAnnotation("https://en.wikipedia.org/wiki/Parchment",
		"Wikipedia: Parchment", withAppearance, qq)
	if err != nil {
		return err
	}
	page.SetFillColor(w.TextCol)
	page.TextShow(", ")
	page.TextSetFont(w.Italic, 10)
	page.TextShow("i.e.")
	page.TextSetFont(w.Roman, 10)
	page.TextShow(" eight leaves or ")
	page.SetFillColor(w.LinkCol)
	qq = w.MakeLink("folios")
	err = w.MakeAnnotation("https://en.wikipedia.org/wiki/Folio",
		"Wikipedia: Folio", withAppearance, qq)
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

func (w *writer) MakeLink(text string) []vec.Vec2 {
	page := w.Page

	glyphs := page.TextLayout(nil, text)
	corners := page.TextGetQuadPoints(glyphs)
	page.TextShowGlyphs(glyphs)

	return corners
}

func (w *writer) MakeAnnotation(url string, title string, app bool, quadPoints ...[]vec.Vec2) error {
	var qq []vec.Vec2
	for _, q := range quadPoints {
		// quadPoints are already Vec2 slices
		qq = append(qq, q...)
	}
	for i := range qq {
		qq[i].X = pdf.Round(qq[i].X, 2)
		qq[i].Y = pdf.Round(qq[i].Y, 2)
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
		BorderStyle: w.BS,
	}

	if len(quadPoints) > 1 {
		link.QuadPoints = qq
	}

	// compute the bounding box from the quad points
	for _, point := range qq {
		link.Common.Rect.ExtendVec(point)
	}

	if app {
		err := w.style.AddAppearance(link)
		if err != nil {
			return err
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
