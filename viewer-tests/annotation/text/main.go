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
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/shading"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

const (
	rowSpacing = 32.0
	iconSize   = 24.0

	topRowY = 500.0

	labelX         = 70.0
	defaultColX    = 160.0
	styledColX     = 200.0
	pinkColX       = 270.0
	styledPinkColX = 310.0
)

func createDocument(filename string) error {
	paper := document.A5
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	page, err := document.CreateSinglePage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	background, err := pageBackground(paper)
	if err != nil {
		return err
	}
	page.DrawShading(background)

	w := &writer{
		annots:    pdf.Array{},
		page:      page,
		style:     fallback.NewStyle(),
		yPos:      topRowY,
		labelFont: standard.Helvetica.New(),
	}

	pink := color.DeviceRGB(0.96, 0.87, 0.90)

	allIcons := []annotation.TextIcon{
		annotation.TextIconComment,
		annotation.TextIconKey,
		annotation.TextIconNote,
		annotation.TextIconHelp,
		annotation.TextIconNewParagraph,
		annotation.TextIconParagraph,
		annotation.TextIconInsert,
	}
	for _, icon := range allIcons {
		w.label(icon)

		// viewer default style
		err := w.annotationPair(icon, defaultColX, nil, false)
		if err != nil {
			return err
		}

		// with appearance dictionary
		err = w.annotationPair(icon, styledColX, nil, true)
		if err != nil {
			return err
		}

		// pink background
		err = w.annotationPair(icon, pinkColX, pink, false)
		if err != nil {
			return err
		}

		// styled with pink background
		err = w.annotationPair(icon, styledPinkColX, pink, true)
		if err != nil {
			return err
		}

		w.yPos -= rowSpacing
	}

	page.PageDict["Annots"] = w.annots

	return page.Close()
}

type writer struct {
	annots    pdf.Array
	page      *document.Page
	style     *fallback.Style
	yPos      float64
	labelFont font.Layouter
}

func (w *writer) embed(a annotation.Annotation, ref pdf.Reference) error {
	obj, err := a.Encode(w.page.RM)
	if err != nil {
		return err
	}
	err = w.page.RM.Out.Put(ref, obj)
	if err != nil {
		return err
	}
	w.annots = append(w.annots, ref)
	return nil
}

// createIconLabel creates the title text for an icon
func (w *writer) label(iconName annotation.TextIcon) {
	w.page.TextBegin()
	w.page.TextSetFont(w.labelFont, 8)
	w.page.TextFirstLine(labelX, w.yPos+10)
	w.page.TextShow(string(iconName))
	w.page.TextEnd()
}

// annotationPair creates a text annotation and its popup
func (w *writer) annotationPair(icon annotation.TextIcon, xPos float64, backgroundColor color.Color, useStyle bool) error {
	textRef := w.page.RM.Out.Alloc()
	popupRef := w.page.RM.Out.Alloc()

	y := w.yPos + 24
	rect := pdf.Rectangle{LLx: xPos, LLy: y - iconSize, URx: xPos + iconSize, URy: y}

	popup := &annotation.Popup{
		Common: annotation.Common{
			Rect:  rect,
			Color: backgroundColor,
		},
		Parent: textRef,
	}

	text := &annotation.Text{
		Common: annotation.Common{
			Rect:     rect,
			Contents: fmt.Sprintf("Icon name %q", icon),
			Color:    backgroundColor,
		},
		Markup: annotation.Markup{
			User:  "Jochen Voss",
			Popup: popupRef,
		},
		Icon: icon,
	}

	if useStyle {
		w.style.AddAppearance(text)
	}

	err := w.embed(text, textRef)
	if err != nil {
		return err
	}

	return w.embed(popup, popupRef)
}

func pageBackground(paper *pdf.Rectangle) (graphics.Shading, error) {
	alpha := 30.0 / 360 * 2 * math.Pi

	nx := math.Cos(alpha)
	ny := math.Sin(alpha)

	t0 := pdf.Round(paper.LLx*nx+paper.LLy*ny, 1)
	t1 := pdf.Round(paper.URx*nx+paper.URy*ny, 1)

	F := &function.Type4{
		Domain:  []float64{t0, t1},
		Range:   []float64{0, 1, 0, 1, 0, 1},
		Program: "dup 16 div floor 16 mul sub 8 ge {0.99 0.98 0.95}{0.96 0.94 0.89}ifelse",
	}

	background := &shading.Type2{
		ColorSpace: color.DeviceRGBSpace,
		X0:         pdf.Round(t0*nx, 1),
		Y0:         pdf.Round(t0*ny, 1),
		X1:         pdf.Round(t1*nx, 1),
		Y1:         pdf.Round(t1*ny, 1),
		F:          F,
		TMin:       t0,
		TMax:       t1,
	}
	return background, nil
}
