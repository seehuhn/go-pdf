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
	"os"
	"strings"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
)

const (
	margin = 40.0 // margin in points
)

var paper = document.A4

func main() {
	fmt.Println("writing test.pdf ...")
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(fname string) error {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	doc, err := document.CreateMultiPage(fname, paper, pdf.V2_0, opt)
	if err != nil {
		return err
	}

	w := newWriter(doc)

	// Add introduction page
	w.writeIntroduction()
	w.newPage()

	annotCol := color.DeviceRGB(0.4, 0.4, 1.0)

	w.printf("Text Annotation")

	textRef := doc.RM.Out.Alloc()
	popupRef := doc.RM.Out.Alloc()

	rect := w.makeRect(32, 32)
	popup := &annotation.Popup{
		Common: annotation.Common{
			Rect:      rect,
			Color:     annotCol,
			SingleUse: true, // Embed() creates a dict, we embed this manually
		},
		Parent: textRef,
	}
	now := string(pdf.Now().AsPDF(doc.RM.Out.GetOptions()).(pdf.String))
	text := &annotation.Text{
		Common: annotation.Common{
			Rect:         rect,
			Color:        annotCol,
			Contents:     "This is an example text annotation.  It contains some text.",
			LastModified: now,
			SingleUse:    true, // Embed() creates a dict, we embed this manually
		},
		Markup: annotation.Markup{
			User:  "Jochen Voss",
			Popup: popupRef,
		},
		Icon: annotation.TextIconNote,
	}
	textNative, _, err := text.Embed(doc.RM)
	if err != nil {
		return err
	}
	err = doc.RM.Out.Put(textRef, textNative)
	if err != nil {
		return err
	}

	popupNative, _, err := popup.Embed(doc.RM)
	if err != nil {
		return err
	}
	err = doc.RM.Out.Put(popupRef, popupNative)
	if err != nil {
		return err
	}

	p := w.page
	annots, _ := p.PageDict["Annots"].(pdf.Array)
	annots = append(annots, textRef, popupRef)
	p.PageDict["Annots"] = annots

	w.printf("Link Annotation")

	w.vSpace(10)
	w.ensureSpace(20)
	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.body, 18)
	w.yPos -= 15
	w.page.TextSetMatrix(matrix.Rotate(0.1).Translate(margin+10, w.yPos))
	w.page.SetFillColor(color.DeviceGray(0.3))
	w.page.TextShow("This is a ")
	w.page.SetFillColor(annotCol)
	gg := w.page.TextLayout(nil, "link to the first page")
	quadPoints := w.page.TextGetQuadPoints(gg)
	w.page.TextShowGlyphs(gg)
	w.page.SetFillColor(color.DeviceGray(0.3))
	w.page.TextShow(".")
	w.yPos -= 5
	w.page.TextEnd()
	w.page.PopGraphicsState()

	rect = pdf.Rectangle{
		LLx: min(quadPoints[0], quadPoints[2], quadPoints[4], quadPoints[6]),
		LLy: min(quadPoints[1], quadPoints[3], quadPoints[5], quadPoints[7]),
		URx: max(quadPoints[0], quadPoints[2], quadPoints[4], quadPoints[6]),
		URy: max(quadPoints[1], quadPoints[3], quadPoints[5], quadPoints[7]),
	}
	link := &annotation.Link{
		Common: annotation.Common{
			Rect:     rect,
			Contents: "Link to the first page",
			Color:    annotCol,
		},
		Action: pdf.Dict{
			"Type": pdf.Name("Action"),
			"S":    pdf.Name("Named"),
			"N":    pdf.Name("FirstPage"),
		},
		Border: &annotation.BorderStyle{
			Width:     0.5,
			Style:     "S",
			SingleUse: true,
		},
		QuadPoints: quadPoints,
	}
	linkRef, _, err := link.Embed(doc.RM)
	if err != nil {
		return err
	}

	annots, _ = p.PageDict["Annots"].(pdf.Array)
	annots = append(annots, linkRef)
	p.PageDict["Annots"] = annots

	err = w.Close()
	if err != nil {
		return err
	}

	return doc.Close()
}

type writer struct {
	doc  *document.MultiPage
	page *document.Page
	yPos float64

	heading font.Layouter
	body    font.Layouter

	grey color.Color
}

func newWriter(doc *document.MultiPage) *writer {
	w := &writer{
		doc:     doc,
		yPos:    paper.URy - margin,
		heading: standard.Helvetica.New(),
		body:    standard.TimesRoman.New(),
		grey:    color.DeviceGray(0.9),
	}

	return w
}

func (w *writer) Close() error {
	if w.page != nil {
		return w.page.Close()
	}
	return nil
}

func (w *writer) printf(format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	lines := strings.Split(text, "\n")

	w.ensureSpace(15)
	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.heading, 12)
	for i, line := range lines {
		w.yPos -= 10
		switch i {
		case 0:
			w.page.TextFirstLine(margin, w.yPos)
		case 1:
			w.page.TextSecondLine(0, -15)
		default:
			w.page.TextNextLine()
		}
		w.page.TextShow(line)
		w.yPos -= 5
	}
	w.page.TextEnd()
	w.page.PopGraphicsState()
}

func (w *writer) ensureSpace(v float64) error {
	if w.page == nil || w.yPos-v < margin {
		if w.page != nil {
			err := w.page.Close()
			if err != nil {
				return err
			}
		}
		w.page = w.doc.AddPage()
		w.yPos = paper.URy - margin
	}
	return nil
}

func (w *writer) newPage() error {
	if w.page != nil {
		err := w.page.Close()
		if err != nil {
			return err
		}
	}
	w.page = w.doc.AddPage()
	w.yPos = paper.URy - margin
	return nil
}

func (w *writer) writeIntroduction() {
	w.ensureSpace(200) // Make sure we have enough space

	w.yPos -= 60

	// Title
	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.heading, 16)
	w.page.TextFirstLine(margin, w.yPos)
	w.page.TextShow("PDF Annotation Types Visual Test")
	w.page.TextEnd()
	w.page.PopGraphicsState()
	w.yPos -= 30

	// Introduction paragraphs
	textWidth := paper.URx - 2*margin

	w.printParagraph(textWidth,
		"This document serves as a visual test for the PDF annotation types "+
			"implemented in the go-pdf library.")
}

func (w *writer) printParagraph(width float64, content string) {
	// Simple paragraph rendering with basic word wrapping
	words := strings.Fields(content)

	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.body, 11)
	w.page.TextFirstLine(margin, w.yPos)

	var currentLine []string
	estimatedWidth := 0.0
	avgCharWidth := 6.0 // Rough estimation for Times Roman at 11pt

	for _, word := range words {
		testWidth := estimatedWidth
		if len(currentLine) > 0 {
			testWidth += avgCharWidth // space
		}
		testWidth += float64(len(word)) * avgCharWidth

		if testWidth > width && len(currentLine) > 0 {
			// Output current line and start new one
			w.page.TextShow(strings.Join(currentLine, " "))
			w.page.TextSecondLine(0, -13)
			w.yPos -= 13
			currentLine = []string{word}
			estimatedWidth = float64(len(word)) * avgCharWidth
		} else {
			currentLine = append(currentLine, word)
			estimatedWidth = testWidth
		}
	}

	// Output remaining text
	if len(currentLine) > 0 {
		w.page.TextShow(strings.Join(currentLine, " "))
		w.yPos -= 13
	}

	w.page.TextEnd()
	w.page.PopGraphicsState()

	w.vSpace(6)
}

func (w *writer) vSpace(v float64) {
	if w.page == nil {
		return
	}
	w.yPos = max(w.yPos-v, margin)
}

func (w *writer) makeRect(width, height float64) pdf.Rectangle {
	w.ensureSpace(height + 20)

	w.yPos -= 10

	w.yPos -= height
	res := pdf.Rectangle{
		LLx: margin,
		LLy: w.yPos,
		URx: margin + width,
		URy: w.yPos + height,
	}

	w.yPos -= 10

	w.page.PushGraphicsState()
	w.page.SetFillColor(w.grey)
	w.page.Rectangle(res.LLx, res.LLy, width, height)
	w.page.Fill()
	w.page.PopGraphicsState()

	return res
}
