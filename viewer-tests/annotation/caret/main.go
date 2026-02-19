// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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
	leftColStart  = 60.0
	leftColEnd    = 90.0
	rightColStart = 220.0
	rightColEnd   = 250.0
	commentStart  = 380.0

	startY    = 780.0
	caretSize = 24.0
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

	page.DrawShading(pageBackground(paper))

	B := standard.TimesBold.New()
	H := standard.Helvetica.New()

	w := &writer{
		page:  page,
		style: fallback.NewStyle(),
		yPos:  startY,
		font:  H,
	}

	page.TextBegin()
	page.TextSetMatrix(matrix.Translate(leftColStart-5, w.yPos))
	page.TextSetFont(B, 12)
	page.TextShow("Your PDF viewer")
	page.TextSetMatrix(matrix.Translate(rightColStart-5, w.yPos))
	page.TextShow("Quire appearance stream")
	page.TextEnd()
	w.yPos -= 24.0

	// blue caret
	a := &annotation.Caret{
		Common: annotation.Common{
			Contents: "blue caret",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	// paragraph symbol, large rect with RD
	err = w.addWidePair(&annotation.Caret{
		Common: annotation.Common{
			Contents: "Sy=P, large rect",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		Symbol: "P",
		Margin: []float64{30, 10, 30, 10},
	})
	if err != nil {
		return err
	}

	// nil color
	a = &annotation.Caret{
		Common: annotation.Common{
			Contents: "nil color",
			Flags:    annotation.FlagPrint,
		},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	// border width
	a = &annotation.Caret{
		Common: annotation.Common{
			Contents: "Border.Width=2",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
			Border:   &annotation.Border{Width: 2, SingleUse: true},
		},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	// tall rect
	err = w.addTallCaret()
	if err != nil {
		return err
	}

	return page.Close()
}

type writer struct {
	page  *document.Page
	style *fallback.Style
	yPos  float64
	font  font.Layouter
}

func (w *writer) addAnnotation(a annotation.Annotation) {
	w.page.Page.Annots = append(w.page.Page.Annots, a)
}

func (w *writer) addAnnotationPair(left *annotation.Caret) error {
	w.page.TextBegin()
	w.page.TextSetFont(w.font, 10)
	w.page.TextSetMatrix(matrix.Translate(commentStart, w.yPos-caretSize/2-3))
	w.page.TextShow(left.Contents)
	w.page.TextEnd()

	right := clone(left)

	left.Rect = pdf.Rectangle{
		LLx: leftColStart,
		LLy: w.yPos - caretSize,
		URx: leftColEnd,
		URy: w.yPos,
	}
	left.Contents += " (viewer)"

	right.Rect = pdf.Rectangle{
		LLx: rightColStart,
		LLy: w.yPos - caretSize,
		URx: rightColEnd,
		URy: w.yPos,
	}
	right.Contents += " (quire)"

	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}

	w.addAnnotation(left)
	w.addAnnotation(right)

	w.yPos -= caretSize + 12.0
	return nil
}

// addWidePair places the caret region (inside RD margins) at the standard
// column positions, expanding the outer rect outward by the margins.
func (w *writer) addWidePair(left *annotation.Caret) error {
	m := left.Margin
	if len(m) != 4 {
		m = []float64{0, 0, 0, 0}
	}

	// outer rect height determines vertical spacing
	outerH := caretSize + m[1] + m[3]

	w.page.TextBegin()
	w.page.TextSetFont(w.font, 10)
	w.page.TextSetMatrix(matrix.Translate(commentStart, w.yPos-caretSize/2-3))
	w.page.TextShow(left.Contents)
	w.page.TextEnd()

	right := clone(left)

	// inner caret region aligns with standard column positions;
	// outer rect is expanded by the margins
	left.Rect = pdf.Rectangle{
		LLx: leftColStart - m[0],
		LLy: w.yPos - caretSize - m[1],
		URx: leftColEnd + m[2],
		URy: w.yPos + m[3],
	}
	left.Contents += " (viewer)"

	right.Rect = pdf.Rectangle{
		LLx: rightColStart - m[0],
		LLy: w.yPos - caretSize - m[1],
		URx: rightColEnd + m[2],
		URy: w.yPos + m[3],
	}
	right.Contents += " (quire)"

	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}

	w.addAnnotation(left)
	w.addAnnotation(right)

	w.yPos -= outerH + 12.0
	return nil
}

func (w *writer) addTallCaret() error {
	tallH := 48.0

	w.page.TextBegin()
	w.page.TextSetFont(w.font, 10)
	w.page.TextSetMatrix(matrix.Translate(commentStart, w.yPos-tallH/2-3))
	w.page.TextShow("tall rect")
	w.page.TextEnd()

	left := &annotation.Caret{
		Common: annotation.Common{
			Contents: "tall rect (viewer)",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
	}
	right := &annotation.Caret{
		Common: annotation.Common{
			Contents: "tall rect (quire)",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
	}

	left.Rect = pdf.Rectangle{
		LLx: leftColStart,
		LLy: w.yPos - tallH,
		URx: leftColEnd,
		URy: w.yPos,
	}
	right.Rect = pdf.Rectangle{
		LLx: rightColStart,
		LLy: w.yPos - tallH,
		URx: rightColEnd,
		URy: w.yPos,
	}

	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}

	w.addAnnotation(left)
	w.addAnnotation(right)

	w.yPos -= tallH + 12.0
	return nil
}

func clone[T any](v *T) *T {
	if v == nil {
		return nil
	}
	clone := *v
	return &clone
}

func pageBackground(paper *pdf.Rectangle) graphics.Shading {
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
		ColorSpace: color.SpaceDeviceRGB,
		P0:         vec.Vec2{X: pdf.Round(t0*nx, 1), Y: pdf.Round(t0*ny, 1)},
		P1:         vec.Vec2{X: pdf.Round(t1*nx, 1), Y: pdf.Round(t1*ny, 1)},
		F:          F,
		TMin:       t0,
		TMax:       t1,
	}
	return background
}
