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
	// horizontal spacing
	leftColStart  = 60.0
	leftColEnd    = 160.0
	rightColStart = 220.0
	rightColEnd   = 320.0
	commentStart  = 380.0

	startY = 790.0
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
		size:  48,
	}

	page.TextBegin()
	page.TextSetMatrix(matrix.Translate(leftColStart-5, w.yPos))
	page.TextSetFont(B, 12)
	page.TextShow("Your PDF viewer")
	page.TextSetMatrix(matrix.Translate(rightColStart-5, w.yPos))
	page.TextShow("Quire appearance stream")
	page.TextEnd()
	w.yPos -= 24.0

	a := &annotation.Polygon{
		Common: annotation.Common{
			Contents: "default border width",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	a = &annotation.Polygon{
		Common: annotation.Common{
			Contents: "Border.Width=2",
			Flags:    annotation.FlagPrint,
			Border:   &annotation.Border{Width: 2, SingleUse: true},
			Color:    color.Blue,
		},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	a = &annotation.Polygon{
		Common: annotation.Common{
			Contents: "BorderStyle.Width=2",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		BorderStyle: &annotation.BorderStyle{Width: 2, Style: "S", SingleUse: true},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	a = &annotation.Polygon{
		Common: annotation.Common{
			Contents: "dashed border",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		BorderStyle: &annotation.BorderStyle{
			Width:     2,
			Style:     "D",
			DashArray: []float64{20, 2, 5, 2},
			SingleUse: true,
		},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	a = &annotation.Polygon{
		Common: annotation.Common{
			Contents: "border style B",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		BorderStyle: &annotation.BorderStyle{
			Width:     2,
			Style:     "B",
			SingleUse: true,
		},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	a = &annotation.Polygon{
		Common: annotation.Common{
			Contents: "border style U",
			Flags:    annotation.FlagPrint,
			Color:    color.Blue,
		},
		BorderStyle: &annotation.BorderStyle{
			Width:     2,
			Style:     "U",
			SingleUse: true,
		},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	a = &annotation.Polygon{
		Common: annotation.Common{
			Contents: "default border color",
			Flags:    annotation.FlagPrint,
			Border:   &annotation.Border{Width: 2, SingleUse: true},
		},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	a = &annotation.Polygon{
		Common: annotation.Common{
			Contents: "filled",
			Flags:    annotation.FlagPrint,
			Border:   &annotation.Border{Width: 1, SingleUse: true},
			Color:    color.Black,
		},
		FillColor: color.White,
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	// cloudy borders need slightly larger pentagons
	w.size = 70

	a = &annotation.Polygon{
		Common: annotation.Common{
			Contents: "cloudy border, intensity=0",
			Flags:    annotation.FlagPrint,
			Color:    color.Black,
		},
		FillColor: color.White,
		BorderStyle: &annotation.BorderStyle{
			Width:     1,
			Style:     "S",
			SingleUse: true,
		},
		BorderEffect: &annotation.BorderEffect{
			Style:     "C",
			Intensity: 0,
			SingleUse: true,
		},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	a = &annotation.Polygon{
		Common: annotation.Common{
			Contents: "cloudy border, intensity=1",
			Flags:    annotation.FlagPrint,
			Color:    color.Black,
		},
		FillColor: color.White,
		BorderStyle: &annotation.BorderStyle{
			Width:     1,
			Style:     "S",
			SingleUse: true,
		},
		BorderEffect: &annotation.BorderEffect{
			Style:     "C",
			Intensity: 1,
			SingleUse: true,
		},
	}
	err = w.addAnnotationPair(a)
	if err != nil {
		return err
	}

	w.yPos += 12

	a = &annotation.Polygon{
		Common: annotation.Common{
			Contents: "cloudy border, intensity=2",
			Flags:    annotation.FlagPrint,
			Color:    color.Black,
		},
		FillColor: color.White,
		BorderStyle: &annotation.BorderStyle{
			Width:     1,
			Style:     "S",
			SingleUse: true,
		},
		BorderEffect: &annotation.BorderEffect{
			Style:     "C",
			Intensity: 2,
			SingleUse: true,
		},
	}
	err = w.addAnnotationPair(a)
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
	size  float64
}

func (w *writer) addAnnotation(a annotation.Annotation) {
	w.page.Page.Annots = append(w.page.Page.Annots, a)
}

func (w *writer) addAnnotationPair(left *annotation.Polygon) error {
	if left.BorderEffect != nil {
		w.yPos -= 10 * left.BorderEffect.Intensity
	}

	w.page.TextBegin()
	w.page.TextSetFont(w.font, 10)
	w.page.TextSetMatrix(matrix.Translate(commentStart, w.yPos-w.size/2-3))
	w.page.TextShow(left.Contents)
	w.page.TextEnd()

	right := clone(left)

	left.Rect = pdf.Rectangle{
		LLx: leftColStart,
		LLy: w.yPos - w.size,
		URx: leftColEnd,
		URy: w.yPos,
	}
	left.Vertices = pentagonVertices(
		(leftColStart+leftColEnd)/2,
		w.yPos-w.size/2,
		w.size/2-1,
	)
	left.Contents += " (viewer)"

	right.Rect = pdf.Rectangle{
		LLx: rightColStart,
		LLy: w.yPos - w.size,
		URx: rightColEnd,
		URy: w.yPos,
	}
	right.Vertices = pentagonVertices(
		(rightColStart+rightColEnd)/2,
		w.yPos-w.size/2,
		w.size/2-1,
	)
	right.Contents += " (quire)"

	err := w.style.AddAppearance(right)
	if err != nil {
		return err
	}

	w.addAnnotation(left)
	w.addAnnotation(right)

	w.yPos -= w.size + 6.0
	if left.BorderEffect != nil {
		w.yPos -= 10 * left.BorderEffect.Intensity
	}
	return nil
}

// pentagonVertices returns flat x,y pairs for a regular pentagon
// centered at (cx, cy) with the given radius.
func pentagonVertices(cx, cy, r float64) []float64 {
	vv := make([]float64, 10)
	for i := range 5 {
		// start at top, go clockwise
		angle := math.Pi/2 + float64(i)*2*math.Pi/5
		vv[2*i] = cx + r*math.Cos(angle)
		vv[2*i+1] = cy + r*math.Sin(angle)
	}
	return vv
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
