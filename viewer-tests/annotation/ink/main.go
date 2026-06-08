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
	leftColEnd    = 160.0
	rightColStart = 220.0
	rightColEnd   = 320.0
	commentStart  = 380.0

	startY = 790.0

	shapeHeight = 40.0
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

	B := font.Must(standard.TimesBold.New())
	H := font.Must(standard.Helvetica.New())

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

	// test 1: default border width
	a := &annotation.Ink{
		Common: annotation.Common{
			Contents: "default line width",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
	}
	err = w.addAnnotationPair(a, scribble)
	if err != nil {
		return err
	}

	// test 2: Border.Width=2
	a = &annotation.Ink{
		Common: annotation.Common{
			Contents: "Border.Width=2",
			Flags:    annotation.FlagPrint,
			Border:   &annotation.Border{Width: 2, SingleUse: true},
			Color:    color.Blue,
		},
	}
	err = w.addAnnotationPair(a, scribble)
	if err != nil {
		return err
	}

	// test 3: BorderStyle.Width=2
	a = &annotation.Ink{
		Common: annotation.Common{
			Contents: "BorderStyle.Width=2",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		BorderStyle: &annotation.BorderStyle{Width: 2, Style: "S", SingleUse: true},
	}
	err = w.addAnnotationPair(a, scribble)
	if err != nil {
		return err
	}

	// test 4: dashed line
	a = &annotation.Ink{
		Common: annotation.Common{
			Contents: "dashed line",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		BorderStyle: &annotation.BorderStyle{
			Width:     2,
			Style:     "D",
			DashArray: []float64{12, 6, 4, 6},
			SingleUse: true,
		},
	}
	err = w.addAnnotationPair(a, scribble)
	if err != nil {
		return err
	}

	// test 5: thick stroke
	a = &annotation.Ink{
		Common: annotation.Common{
			Contents: "thick stroke (width 5)",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		BorderStyle: &annotation.BorderStyle{Width: 5, Style: "S", SingleUse: true},
	}
	err = w.addAnnotationPair(a, scribble)
	if err != nil {
		return err
	}

	// test 6: multi-stroke (plus sign — two disjoint paths)
	a = &annotation.Ink{
		Common: annotation.Common{
			Contents: "multi-stroke (plus sign)",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		BorderStyle: &annotation.BorderStyle{Width: 2, Style: "S", SingleUse: true},
	}
	err = w.addAnnotationPair(a, plusSign)
	if err != nil {
		return err
	}

	// test 7: short stroke (two points only)
	a = &annotation.Ink{
		Common: annotation.Common{
			Contents: "short stroke (two points)",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		BorderStyle: &annotation.BorderStyle{Width: 2, Style: "S", SingleUse: true},
	}
	err = w.addAnnotationPair(a, shortStroke)
	if err != nil {
		return err
	}

	// test 8: single-point dot
	a = &annotation.Ink{
		Common: annotation.Common{
			Contents: "single-point dot",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		BorderStyle: &annotation.BorderStyle{Width: 6, Style: "S", SingleUse: true},
	}
	err = w.addAnnotationPair(a, dot)
	if err != nil {
		return err
	}

	// test 9: no color (invisible)
	a = &annotation.Ink{
		Common: annotation.Common{
			Contents: "no color (invisible)",
			Flags:    annotation.FlagPrint,
		},
	}
	err = w.addAnnotationPair(a, scribble)
	if err != nil {
		return err
	}

	// test 10: stroking transparency
	a = &annotation.Ink{
		Common: annotation.Common{
			Contents:                "50% transparency",
			Flags:                   annotation.FlagPrint,
			Color:                   color.Blue,
			StrokingTransparency:    0.5,
			NonStrokingTransparency: 0.5,
		},
		BorderStyle: &annotation.BorderStyle{Width: 5, Style: "S", SingleUse: true},
	}
	err = w.addAnnotationPair(a, scribble)
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

// inkShape builds an InkList that fits inside the given bounding rectangle.
type inkShape func(x0, x1, y0, y1 float64) [][]vec.Vec2

func (w *writer) addAnnotationPair(left *annotation.Ink, shape inkShape) error {
	w.page.TextBegin()
	w.page.TextSetFont(w.font, 10)
	w.page.TextSetMatrix(matrix.Translate(commentStart, w.yPos-shapeHeight/2-3))
	w.page.TextShow(left.Contents)
	w.page.TextEnd()

	right := clone(left)

	left.Rect = pdf.Rectangle{
		LLx: leftColStart,
		LLy: w.yPos - shapeHeight,
		URx: leftColEnd,
		URy: w.yPos,
	}
	left.InkList = shape(leftColStart, leftColEnd, w.yPos-shapeHeight, w.yPos)
	left.Contents += " (viewer)"

	right.Rect = pdf.Rectangle{
		LLx: rightColStart,
		LLy: w.yPos - shapeHeight,
		URx: rightColEnd,
		URy: w.yPos,
	}
	right.InkList = shape(rightColStart, rightColEnd, w.yPos-shapeHeight, w.yPos)
	right.Contents += " (quire)"

	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}

	w.addAnnotation(left)
	w.addAnnotation(right)

	w.yPos -= shapeHeight + 10.0
	return nil
}

// scribble returns a single wavy path across the box.
func scribble(x0, x1, y0, y1 float64) [][]vec.Vec2 {
	const n = 17
	amp := (y1 - y0) * 0.35
	midY := (y0 + y1) / 2
	pts := make([]vec.Vec2, n)
	for i := range n {
		t := float64(i) / float64(n-1)
		pts[i] = vec.Vec2{
			X: x0 + t*(x1-x0),
			Y: midY + amp*math.Sin(t*3*math.Pi),
		}
	}
	return [][]vec.Vec2{pts}
}

// plusSign returns two disjoint strokes forming a plus sign.
func plusSign(x0, x1, y0, y1 float64) [][]vec.Vec2 {
	cx := (x0 + x1) / 2
	cy := (y0 + y1) / 2
	r := math.Min(x1-x0, y1-y0) * 0.4
	return [][]vec.Vec2{
		{{X: cx - r, Y: cy}, {X: cx + r, Y: cy}},
		{{X: cx, Y: cy - r}, {X: cx, Y: cy + r}},
	}
}

// shortStroke returns a single two-point stroke across the box diagonal.
func shortStroke(x0, x1, y0, y1 float64) [][]vec.Vec2 {
	return [][]vec.Vec2{
		{{X: x0 + 10, Y: y0 + 10}, {X: x1 - 10, Y: y1 - 10}},
	}
}

// dot returns a single-point sub-path at the box centre.
func dot(x0, x1, y0, y1 float64) [][]vec.Vec2 {
	return [][]vec.Vec2{
		{{X: (x0 + x1) / 2, Y: (y0 + y1) / 2}},
	}
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
