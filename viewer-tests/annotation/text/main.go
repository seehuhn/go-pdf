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

const (
	// horizontal spacing
	leftColStart  = 60.0
	leftColEnd    = 160.0
	rightColStart = 220.0
	rightColEnd   = 320.0
	commentStart  = 380.0

	// vertical spacing
	startY   = 780.0
	iconSize = 24.0
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

	background, err := pageBackground(paper)
	if err != nil {
		return err
	}
	page.DrawShading(background)

	B := standard.TimesBold.New()
	H := standard.Helvetica.New()

	w := &writer{
		annots: pdf.Array{},
		page:   page,
		style:  fallback.NewStyle(),
		yPos:   startY,
		font:   H,
	}

	// add headers
	page.TextBegin()
	page.TextSetMatrix(matrix.Translate(leftColStart-5, w.yPos))
	page.TextSetFont(B, 12)
	page.TextShow("Your PDF viewer")
	page.TextSetMatrix(matrix.Translate(rightColStart-5, w.yPos))
	page.TextShow("Quire appearance stream")
	page.TextEnd()
	w.yPos -= 24.0

	// test all 7 icon types with default colors
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
		text := &annotation.Text{
			Common: annotation.Common{
				Contents: fmt.Sprintf("Icon: %s", icon),
				Border:   annotation.PDFDefaultBorder,
				Flags:    annotation.FlagPrint,
			},
			Markup: annotation.Markup{
				User: "Test User",
			},
			Icon: icon,
		}
		err = w.addAnnotationPair(text, string(icon))
		if err != nil {
			return err
		}
	}

	// test with pink color
	pink := color.DeviceRGB(0.96, 0.87, 0.90)
	text := &annotation.Text{
		Common: annotation.Common{
			Contents: "Pink background",
			Color:    pink,
			Border:   annotation.PDFDefaultBorder,
			Flags:    annotation.FlagPrint,
		},
		Markup: annotation.Markup{
			User: "Test User",
		},
		Icon: annotation.TextIconNote,
	}
	err = w.addAnnotationPair(text, "Common.Color = pink")
	if err != nil {
		return err
	}

	// test with transparent color (no appearance stream)
	text = &annotation.Text{
		Common: annotation.Common{
			Contents: "Transparent background",
			Color:    annotation.Transparent,
			Border:   annotation.PDFDefaultBorder,
			Flags:    annotation.FlagPrint,
		},
		Markup: annotation.Markup{
			User: "Test User",
		},
		Icon: annotation.TextIconNote,
	}
	err = w.addAnnotationPair(text, "Common.Color = transparent")
	if err != nil {
		return err
	}

	// test with border width
	text = &annotation.Text{
		Common: annotation.Common{
			Contents: "Border width 2",
			Border:   &annotation.Border{Width: 2, SingleUse: true},
			Flags:    annotation.FlagPrint,
		},
		Markup: annotation.Markup{
			User: "Test User",
		},
		Icon: annotation.TextIconNote,
	}
	err = w.addAnnotationPair(text, "Common.Border.Width = 2")
	if err != nil {
		return err
	}

	page.PageDict["Annots"] = w.annots

	return page.Close()
}

type writer struct {
	annots pdf.Array
	page   *document.Page
	style  *fallback.Style
	yPos   float64
	font   font.Layouter
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

func (w *writer) addAnnotationPair(left *annotation.Text, label string) error {
	leftRef := w.page.RM.Out.Alloc()
	rightRef := w.page.RM.Out.Alloc()

	// center icons horizontally in columns
	leftCenter := (leftColStart + leftColEnd) / 2
	rightCenter := (rightColStart + rightColEnd) / 2

	// add label on the right side
	w.page.TextBegin()
	w.page.TextSetFont(w.font, 10)
	w.page.TextSetMatrix(matrix.Translate(commentStart, w.yPos-iconSize/2-3))
	w.page.TextShow(label)
	w.page.TextSetFont(w.font, 6)
	w.page.TextSetHorizontalScaling(0.9)
	w.page.TextSetMatrix(matrix.Translate(leftCenter+iconSize/2+3, w.yPos-iconSize))
	w.page.TextShow(fmt.Sprintf("%d %d R", leftRef.Number(), leftRef.Generation()))
	w.page.TextSetMatrix(matrix.Translate(rightCenter+iconSize/2+3, w.yPos-iconSize))
	w.page.TextShow(fmt.Sprintf("%d %d R", rightRef.Number(), rightRef.Generation()))
	w.page.TextEnd()

	right := clone(left)

	left.Rect = pdf.Rectangle{
		LLx: leftCenter - iconSize/2,
		LLy: w.yPos - iconSize,
		URx: leftCenter + iconSize/2,
		URy: w.yPos,
	}
	left.Contents += " (viewer)"

	right.Rect = pdf.Rectangle{
		LLx: rightCenter - iconSize/2,
		LLy: w.yPos - iconSize,
		URx: rightCenter + iconSize/2,
		URy: w.yPos,
	}
	right.Contents += " (quire)"

	w.style.AddAppearance(right)

	err := w.embed(left, leftRef)
	if err != nil {
		return err
	}

	err = w.embed(right, rightRef)
	if err != nil {
		return err
	}

	w.yPos -= iconSize + 12.0

	return nil
}

func clone[T any](v *T) *T {
	if v == nil {
		return nil
	}
	clone := *v
	return &clone
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
		P0:         vec.Vec2{X: pdf.Round(t0*nx, 1), Y: pdf.Round(t0*ny, 1)},
		P1:         vec.Vec2{X: pdf.Round(t1*nx, 1), Y: pdf.Round(t1*ny, 1)},
		F:          F,
		TMin:       t0,
		TMax:       t1,
	}
	return background, nil
}
