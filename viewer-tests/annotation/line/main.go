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
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/shading"
)

const (
	// column positions
	leftColStart  = 100.0
	leftColEnd    = 250.0
	rightColStart = 350.0

	// vertical spacing for different groups
	startY          = 750.0
	lineEndingStep  = 20.0
	borderTestStep  = 20.0
	captionTestStep = 30.0

	// line characteristics
	defaultLineWidth = 1.2
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
	doc, err := document.CreateMultiPage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	page := doc.AddPage()
	page.DrawShading(pageBackground(paper))

	style, err := fallback.NewStyle().New(pdf.V1_7)
	if err != nil {
		return err
	}

	w := &writer{
		page:  page,
		style: style,
		yPos:  startY,
	}

	lineStyle := &annotation.BorderStyle{
		Width: defaultLineWidth,
	}

	// Group 1: Line ending styles
	lineEndingStyles := []annotation.LineEndingStyle{
		annotation.LineEndingStyleSquare,
		annotation.LineEndingStyleCircle,
		annotation.LineEndingStyleDiamond,
		annotation.LineEndingStyleOpenArrow,
		annotation.LineEndingStyleClosedArrow,
		annotation.LineEndingStyleNone,
		annotation.LineEndingStyleButt,
		annotation.LineEndingStyleROpenArrow,
		annotation.LineEndingStyleRClosedArrow,
		annotation.LineEndingStyleSlash,
	}

	for _, style := range lineEndingStyles {
		line := &annotation.Line{
			Common: annotation.Common{
				Rect: pdf.Rectangle{
					LLx: pdf.Round(leftColStart-10, 2),
					LLy: pdf.Round(w.yPos-10, 2),
					URx: pdf.Round(leftColEnd+10, 2),
					URy: pdf.Round(w.yPos+10, 2),
				},
				Contents: string(style),
				Color:    color.Black,
				Flags:    annotation.FlagPrint,
			},
			Coords: [4]float64{
				pdf.Round(leftColStart, 2),
				pdf.Round(w.yPos, 2),
				pdf.Round(leftColEnd, 2),
				pdf.Round(w.yPos, 2),
			},
			BorderStyle:     lineStyle,
			LineEndingStyle: [2]annotation.LineEndingStyle{style, style},
			FillColor:       color.Red,
		}
		err = w.addAnnotationPair(line)
		if err != nil {
			return err
		}

		w.yPos -= lineEndingStep
	}

	// -----------------------------------------------------------------------

	// Group 2: Border comparison test
	w.yPos -= 24 // extra gap before next group

	// line with Common.Border
	borderLine1 := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.yPos-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.yPos+10, 2),
			},
			Contents: "Common.Border with dash",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
			Border:   &annotation.Border{Width: 2, DashArray: []float64{10, 2}, SingleUse: true},
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.yPos, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.yPos, 2),
		},
	}
	err = w.addAnnotationPair(borderLine1)
	if err != nil {
		return err
	}

	w.yPos -= borderTestStep

	// line with BorderStyle
	borderLine2 := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.yPos-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.yPos+10, 2),
			},
			Contents: "BorderStyle with dash",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.yPos, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.yPos, 2),
		},
		BorderStyle: &annotation.BorderStyle{Width: 2, Style: "D", DashArray: []float64{10, 2}, SingleUse: true},
	}
	err = w.addAnnotationPair(borderLine2)
	if err != nil {
		return err
	}

	w.yPos -= borderTestStep

	// line with no border or border style
	borderLine3 := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.yPos-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.yPos+10, 2),
			},
			Contents: "no line style specified",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
			// the PDF default, which is written by leaving Border out
			Border: annotation.PDFDefaultBorder,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.yPos, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.yPos, 2),
		},
	}
	err = w.addAnnotationPair(borderLine3)
	if err != nil {
		return err
	}

	w.yPos -= borderTestStep

	// line with width 0, which asks for a hairline; a nil Border is written
	// as [0 0 0]
	borderLine4 := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.yPos-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.yPos+10, 2),
			},
			Contents: "Border width 0",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.yPos, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.yPos, 2),
		},
	}
	err = w.addAnnotationPair(borderLine4)
	if err != nil {
		return err
	}

	// -----------------------------------------------------------------------

	// Group 3: Caption tests
	w.yPos -= 48 // extra gap before next group

	// caption inline
	captionInline := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.yPos-15, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.yPos+15, 2),
			},
			Contents: "inline caption",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.yPos, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.yPos, 2),
		},
		BorderStyle: lineStyle,
		Caption:     true,
	}
	err = w.addAnnotationPair(captionInline)
	if err != nil {
		return err
	}

	w.yPos -= captionTestStep

	// shifted inline caption
	captionInline = &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.yPos-30, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.yPos+30, 2),
			},
			Contents: "shifted inline caption",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.yPos, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.yPos, 2),
		},
		BorderStyle:   lineStyle,
		Caption:       true,
		CaptionOffset: []float64{20, 3},
	}
	err = w.addAnnotationPair(captionInline)
	if err != nil {
		return err
	}

	w.yPos -= captionTestStep

	// caption on top
	captionTop := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.yPos-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.yPos+20, 2),
			},
			Contents: "top caption",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.yPos, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.yPos, 2),
		},
		BorderStyle:  lineStyle,
		Caption:      true,
		CaptionAbove: true,
	}
	err = w.addAnnotationPair(captionTop)
	if err != nil {
		return err
	}

	// captions on a slanted line, on a line running from right to left, and
	// with two lines of text
	moreCaptions := []struct {
		contents string
		x0, dy0  float64
		x1, dy1  float64
		above    bool
	}{
		{"slanted caption", leftColStart, -15, leftColEnd, 15, false},
		{"right to left, top", leftColEnd, 0, leftColStart, 0, true},
		{"two-line\ncaption", leftColStart, 0, leftColEnd, 0, false},
	}
	for _, c := range moreCaptions {
		w.yPos -= 2 * captionTestStep
		line := &annotation.Line{
			Common: annotation.Common{
				Rect: pdf.Rectangle{
					LLx: pdf.Round(leftColStart-10, 2),
					LLy: pdf.Round(w.yPos-30, 2),
					URx: pdf.Round(leftColEnd+10, 2),
					URy: pdf.Round(w.yPos+30, 2),
				},
				Contents: c.contents,
				Color:    color.Black,
				Flags:    annotation.FlagPrint,
			},
			Coords: [4]float64{
				pdf.Round(c.x0, 2),
				pdf.Round(w.yPos+c.dy0, 2),
				pdf.Round(c.x1, 2),
				pdf.Round(w.yPos+c.dy1, 2),
			},
			BorderStyle:  lineStyle,
			Caption:      true,
			CaptionAbove: c.above,
		}
		err = w.addAnnotationPair(line)
		if err != nil {
			return err
		}
	}

	// -----------------------------------------------------------------------

	// Group 4: Leader line tests, on a page of their own
	err = page.Close()
	if err != nil {
		return err
	}
	page = doc.AddPage()
	page.DrawShading(pageBackground(paper))
	w.page = page
	w.yPos = startY

	// positive LL
	w.drawGuide()

	leaderPos := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.yPos-30, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.yPos+30, 2),
			},
			Contents: "LL=24 (positive)",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.yPos, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.yPos, 2),
		},
		BorderStyle: lineStyle,
		LL:          24,
	}
	err = w.addAnnotationPair(leaderPos)
	if err != nil {
		return err
	}

	w.yPos -= 36

	// negative LL
	w.drawGuide()

	leaderNeg := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.yPos-30, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.yPos+30, 2),
			},
			Contents: "LL=-24 (negative)",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.yPos, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.yPos, 2),
		},
		BorderStyle: lineStyle,
		LL:          -24,
	}
	err = w.addAnnotationPair(leaderNeg)
	if err != nil {
		return err
	}

	w.yPos -= 120

	// combined LL, LLE, LLO
	w.drawGuide()

	leaderCombo := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.yPos-30, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.yPos+75, 2),
			},
			Contents: "LL=50, LLE=10, LLO=10",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.yPos, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.yPos, 2),
		},
		BorderStyle: lineStyle,
		FillColor:   color.DeviceRGB{1, 1, 0.5},
		LLE:         10,
		LL:          50,
		LLO:         10,
	}
	err = w.addAnnotationPair(leaderCombo)
	if err != nil {
		return err
	}

	w.yPos -= 72

	// caption on a line with leader lines
	w.drawGuide()
	leaderCaption := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.yPos-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.yPos+40, 2),
			},
			Contents: "LL=24 with caption",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.yPos, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.yPos, 2),
		},
		BorderStyle: lineStyle,
		LL:          24,
		Caption:     true,
	}
	err = w.addAnnotationPair(leaderCaption)
	if err != nil {
		return err
	}

	err = page.Close()
	if err != nil {
		return err
	}
	return doc.Close()
}

type writer struct {
	page  *document.Page
	style *fallback.Generator
	yPos  float64
}

// drawGuide marks, in both columns, where the coordinates of a line with
// leader lines lie
func (w *writer) drawGuide() {
	page := w.page
	page.PushGraphicsState()
	page.SetLineWidth(5)
	page.SetStrokeColor(color.DeviceGray(0.9))
	for _, x0 := range []float64{leftColStart, rightColStart} {
		x1 := x0 + leftColEnd - leftColStart
		page.MoveTo(pdf.Round(x0, 2), pdf.Round(w.yPos, 2))
		page.LineTo(pdf.Round(x1, 2), pdf.Round(w.yPos, 2))
	}
	page.Stroke()
	page.PopGraphicsState()
}

func (w *writer) addAnnotation(a annotation.Annotation) {
	w.page.Page.AddAnnots(a)
}

func (w *writer) addAnnotationPair(line *annotation.Line) error {
	// add left annotation as-is
	w.addAnnotation(line)

	// create shallow copy for right column
	rightLine := clone(line)

	// adjust coordinates for right column
	deltaX := rightColStart - leftColStart
	rightLine.Rect.LLx += deltaX
	rightLine.Rect.URx += deltaX
	rightLine.Coords[0] += deltaX
	rightLine.Coords[2] += deltaX

	err := w.style.AddAppearance(rightLine)
	if err != nil {
		return err
	}

	// add right annotation
	w.addAnnotation(rightLine)
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
		Common: shading.Common{ColorSpace: color.SpaceDeviceRGB},
		P0:     vec.Vec2{X: pdf.Round(t0*nx, 1), Y: pdf.Round(t0*ny, 1)},
		P1:     vec.Vec2{X: pdf.Round(t1*nx, 1), Y: pdf.Round(t1*ny, 1)},
		F:      F,
		TMin:   t0,
		TMax:   t1,
	}
	return background
}
