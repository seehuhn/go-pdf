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

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

const (
	// horizontal spacing
	leftColStart  = 36.0
	rightColStart = 304.0
	colWidth      = 244.0

	// vertical spacing
	startY = 780.0
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
		Page:    page.Builder,
		Roman:   standard.TimesRoman.New(),
		Italic:  standard.TimesItalic.New(),
		TextCol: color.DeviceGray(0.1),
		LinkCol: color.DeviceRGB{0, 0, 0.9},

		style: fallback.NewStyle(),
		page:  page,
		yPos:  startY,
	}

	// title

	B := standard.TimesBold.New()
	page.TextBegin()
	page.TextSetMatrix(matrix.Translate(leftColStart, w.yPos))
	page.TextSetFont(B, 12)
	glyphs := page.TextLayout(nil, "Your PDF viewer")
	glyphs.Align(colWidth, 0.5)
	page.TextShowGlyphs(glyphs)
	page.TextSetMatrix(matrix.Translate(rightColStart, w.yPos))
	glyphs = page.TextLayout(nil, "Quire appearance stream")
	glyphs.Align(colWidth, 0.5)
	page.TextShowGlyphs(glyphs)
	page.TextEnd()

	w.yPos -= 36.0

	// paragraphs of text with links

	err = w.addParagraph(leftColStart, w.yPos, false)
	if err != nil {
		return err
	}
	err = w.addParagraph(rightColStart, w.yPos, true)
	if err != nil {
		return err
	}
	w.yPos -= 64.0

	// different ways of setting the border width

	a := &annotation.Link{
		Common: annotation.Common{
			Contents: "default border width",
			Flags:    annotation.FlagPrint,
			Color:    w.LinkCol,
			Border:   &annotation.Border{Width: 1, SingleUse: true},
		},
		Action:    &action.URI{URI: "https://www.example.com/"},
		Highlight: annotation.LinkHighlightInvert,
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	a = &annotation.Link{
		Common: annotation.Common{
			Contents: "Common.Border.Width=2",
			Flags:    annotation.FlagPrint,
			Color:    w.LinkCol,
			Border:   &annotation.Border{Width: 2, SingleUse: true},
		},
		Action: &action.URI{URI: "https://www.example.com/"},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	a = &annotation.Link{
		Common: annotation.Common{
			Contents: "BorderStyle.Width=2",
			Flags:    annotation.FlagPrint,
			Border:   &annotation.Border{Width: 2, SingleUse: true},
			Color:    w.LinkCol,
		},
		Action: &action.URI{URI: "https://www.example.com/"},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	a = &annotation.Link{
		Common: annotation.Common{
			Contents: "default border color",
			Flags:    annotation.FlagPrint,
		},
		Action:      &action.URI{URI: "https://www.example.com/"},
		BorderStyle: &annotation.BorderStyle{Width: 2, Style: "S", SingleUse: true},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	a = &annotation.Link{
		Common: annotation.Common{
			Contents: "transparent border",
			Flags:    annotation.FlagPrint,
			Color:    annotation.Transparent,
		},
		Action:      &action.URI{URI: "https://www.example.com/"},
		BorderStyle: &annotation.BorderStyle{Width: 2, Style: "S", SingleUse: true},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	// framed links with different border styles

	for _, style := range []pdf.Name{"S", "D", "B", "I", "U"} {
		bs := &annotation.BorderStyle{
			Width:     2,
			Style:     style,
			SingleUse: true,
		}
		if style == "D" {
			bs.DashArray = []float64{5, 2}
		}

		a := &annotation.Link{
			Common: annotation.Common{
				Contents: "frame style " + string(style),
				Flags:    annotation.FlagPrint,
				Color:    w.LinkCol,
			},
			Action:      &action.URI{URI: "https://www.example.com/"},
			Highlight:   annotation.LinkHighlightInvert,
			BorderStyle: bs,
		}
		err = w.addAnnotationPair(a)
		if err != nil {
			return err
		}
	}

	// multiple quad points with border

	err = w.DrawQuads(leftColStart+122, w.yPos, false)
	if err != nil {
		return err
	}
	err = w.DrawQuads(rightColStart+122, w.yPos, true)
	if err != nil {
		return err
	}
	w.yPos -= 64.0

	// hexagon shaped link area

	xMid := leftColStart + 0.5*(rightColStart-leftColStart+colWidth)
	hex := make([]vec.Vec2, 6)
	for i := range hex {
		angle := float64(i)*math.Pi/3 + 0.1
		hex[i] = vec.Vec2{
			X: pdf.Round(xMid+100*math.Cos(angle), 2),
			Y: pdf.Round(w.yPos+100*math.Sin(angle)-120, 2),
		}
	}
	page.SetFillColor(color.DeviceCMYK{0, 0.9, 0.9, 0})
	page.MoveTo(hex[0].X, hex[0].Y)
	for i := 1; i < len(hex); i++ {
		page.LineTo(hex[i].X, hex[i].Y)
	}
	page.Fill()
	page.SetStrokeColor(color.Green)
	page.SetLineWidth(1)
	page.MoveTo(hex[0].X, hex[0].Y)
	page.LineTo(hex[1].X, hex[1].Y)
	page.LineTo(hex[2].X, hex[2].Y)
	page.LineTo(hex[3].X, hex[3].Y)
	page.ClosePath()
	page.MoveTo(hex[0].X, hex[0].Y)
	page.LineTo(hex[3].X, hex[3].Y)
	page.LineTo(hex[4].X, hex[4].Y)
	page.LineTo(hex[5].X, hex[5].Y)
	page.ClosePath()
	page.Stroke()
	w.MakeAnnotation("https://en.wikipedia.org/wiki/Hexagon", "Hexagon", nil, false,
		[]vec.Vec2{hex[0], hex[1], hex[2], hex[3]},
		[]vec.Vec2{hex[3], hex[4], hex[5], hex[0]})
	w.yPos -= 280

	return page.Close()
}

type writer struct {
	Page    *builder.Builder
	Roman   font.Layouter
	Italic  font.Layouter
	TextCol color.Color
	LinkCol color.Color

	style *fallback.Style
	page  *document.Page
	yPos  float64
}

func (w *writer) addAnnotation(a annotation.Annotation) {
	w.page.Page.Annots = append(w.page.Page.Annots, a)
}

// addParagraph adds a paragraph to the PDF document at the specified position.
// The text width is 244 units.
func (w *writer) addParagraph(x, y float64, withAppearance bool) error {
	geom := w.Roman.GetGeometry()

	page := w.Page
	page.TextBegin()

	page.TextFirstLine(x, y)
	page.TextSetWordSpacing(0.967)
	page.TextSetFont(w.Roman, 10)
	page.SetFillColor(w.TextCol)
	page.TextShow("In the Middle Ages, a quire (also called a “")
	page.SetFillColor(w.LinkCol)
	qq := w.makeLink("gathering")
	err := w.MakeAnnotation("https://en.wikipedia.org/wiki/Gathering_(bookbinding)",
		"Wikipedia: Gathering (bookbinding)", nil, withAppearance, qq)
	if err != nil {
		return err
	}
	page.SetFillColor(w.TextCol)
	page.TextShow("”) was")

	page.TextSecondLine(0, -geom.Leading*10)
	page.TextSetWordSpacing(0.750)
	page.TextShow("most often formed of four folded sheets of ")
	page.SetFillColor(w.LinkCol)
	qq = w.makeLink("vellum")
	err = w.MakeAnnotation("https://en.wikipedia.org/wiki/Vellum",
		"Wikipedia: Vellum", nil, withAppearance, qq)
	if err != nil {
		return err
	}
	page.SetFillColor(w.TextCol)
	page.TextShow(" or ")
	page.SetFillColor(w.LinkCol)
	qq = w.makeLink("parch-")
	err = w.MakeAnnotation("https://en.wikipedia.org/wiki/Parchment",
		"Wikipedia: Parchment", nil, withAppearance, qq)
	if err != nil {
		return err
	}

	page.TextNextLine()
	page.TextSetWordSpacing(-0.333)
	qq = w.makeLink("ment")
	err = w.MakeAnnotation("https://en.wikipedia.org/wiki/Parchment",
		"Wikipedia: Parchment", nil, withAppearance, qq)
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
	qq = w.makeLink("folios")
	err = w.MakeAnnotation("https://en.wikipedia.org/wiki/Folio",
		"Wikipedia: Folio", nil, withAppearance, qq)
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

func (w *writer) makeLink(text string) []vec.Vec2 {
	page := w.Page

	glyphs := page.TextLayout(nil, text)
	corners := page.TextGetQuadPoints(glyphs, 0)
	page.TextShowGlyphs(glyphs)

	return corners
}

func (w *writer) addAnnotationPair(left *annotation.Link) error {
	page := w.Page

	text := left.Common.Contents

	const (
		margin = 30.0
		boxW   = 244.0 - 2*margin
	)

	// draw the text for both columns
	page.TextBegin()
	page.TextSetFont(w.Roman, 12)
	glyphs := page.TextLayout(nil, text)
	textW := glyphs.TotalWidth()
	page.SetFillColor(w.LinkCol)
	leftX := leftColStart + margin
	page.TextSetMatrix(matrix.Translate(pdf.Round(leftX+(boxW-textW)/2, 2), w.yPos))
	page.TextShowGlyphs(glyphs)
	rightX := rightColStart + margin
	page.TextSetMatrix(matrix.Translate(pdf.Round(rightX+(boxW-textW)/2, 2), w.yPos))
	page.TextShowGlyphs(glyphs)
	page.TextEnd()

	// create a pair of annotations
	right := clone(left)

	left.Common.Rect = pdf.Rectangle{
		LLx: pdf.Round(leftX, 2),
		LLy: pdf.Round(w.yPos-6, 2),
		URx: pdf.Round(leftX+boxW, 2),
		URy: pdf.Round(w.yPos+14, 2),
	}
	left.Common.Contents = "www.example.com (viewer)"
	w.addAnnotation(left)

	right.Common.Rect = pdf.Rectangle{
		LLx: pdf.Round(rightX, 2),
		LLy: pdf.Round(w.yPos-6, 2),
		URx: pdf.Round(rightX+boxW, 2),
		URy: pdf.Round(w.yPos+14, 2),
	}
	right.Common.Contents = "www.example.com (quire)"
	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}
	w.addAnnotation(right)

	w.yPos -= 36.0

	return nil
}

func (w *writer) DrawQuads(x, y float64, withAppearance bool) error {
	page := w.Page

	const alpha = 0.1
	dx := math.Cos(alpha)
	dy := math.Sin(alpha)
	q1 := []vec.Vec2{
		{X: pdf.Round(x-80*dx, 2), Y: pdf.Round(y-80*dy-20, 2)},
		{X: pdf.Round(x+90*dx, 2), Y: pdf.Round(y+90*dy-20, 2)},
		{X: pdf.Round(x+100*dx-10*dy, 2), Y: pdf.Round(y+100*dy+10*dx-20, 2)},
		{X: pdf.Round(x-80*dx-10*dy, 2), Y: pdf.Round(y-80*dy+10*dx-20, 2)},
	}
	q2 := []vec.Vec2{
		{X: pdf.Round(x-100*dx+10*dy, 2), Y: pdf.Round(y-100*dy-10*dx-20, 2)},
		{X: pdf.Round(x+80*dx+10*dy, 2), Y: pdf.Round(y+80*dy-10*dx-20, 2)},
		{X: pdf.Round(x+80*dx, 2), Y: pdf.Round(y+80*dy-20, 2)},
		{X: pdf.Round(x-100*dx, 2), Y: pdf.Round(y-100*dy-20, 2)},
	}
	page.SetFillColor(color.DeviceRGB{0.9, 0.9, 0})
	page.MoveTo(q1[0].X, q1[0].Y)
	for i := 1; i < len(q1); i++ {
		page.LineTo(q1[i].X, q1[i].Y)
	}
	page.ClosePath()
	page.Fill()
	page.SetFillColor(color.DeviceRGB{0, 0.9, 0.9})
	page.MoveTo(q2[0].X, q2[0].Y)
	for i := 1; i < len(q2); i++ {
		page.LineTo(q2[i].X, q2[i].Y)
	}
	page.ClosePath()
	page.Fill()

	bs := &annotation.BorderStyle{
		Width:     2,
		Style:     "U",
		SingleUse: true,
	}
	err := w.MakeAnnotation("https://www.example.com/",
		"www.example.com", bs, withAppearance, q1, q2)
	if err != nil {
		return err
	}

	return nil
}

func (w *writer) MakeAnnotation(url string, title string, bs *annotation.BorderStyle, app bool, quadPoints ...[]vec.Vec2) error {
	var qq []vec.Vec2
	for _, q := range quadPoints {
		qq = append(qq, q...)
	}
	for i := range qq {
		qq[i].X = pdf.Round(qq[i].X, 2)
		qq[i].Y = pdf.Round(qq[i].Y, 2)
	}

	link := &annotation.Link{
		Common: annotation.Common{
			Contents: title,
			Flags:    annotation.FlagPrint,
		},
		Action:      &action.URI{URI: url},
		Highlight:   annotation.LinkHighlightInvert,
		BorderStyle: bs,
	}

	// compute the bounding box from the quad points
	for _, point := range qq {
		link.Common.Rect.ExtendVec(point)
	}

	if len(quadPoints) > 1 {
		link.QuadPoints = qq

		// Avoid rounding issues when viewers check whether the quad points
		// are inside the rectangle.
		link.Common.Rect.LLx -= 0.01
		link.Common.Rect.LLy -= 0.01
		link.Common.Rect.URx += 0.01
		link.Common.Rect.URy += 0.01
	}
	link.Common.Rect.IRound(2)

	if app {
		err := w.style.AddAppearance(link)
		if err != nil {
			return err
		}
	}

	w.addAnnotation(link)

	return nil
}

func clone[T any](v *T) *T {
	if v == nil {
		return nil
	}
	clone := *v
	return &clone
}
