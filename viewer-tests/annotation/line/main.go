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
		page:     page,
		style:    fallback.NewStyle(),
		currentY: startY,
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
					LLy: pdf.Round(w.currentY-10, 2),
					URx: pdf.Round(leftColEnd+10, 2),
					URy: pdf.Round(w.currentY+10, 2),
				},
				Contents: string(style),
				Color:    color.Black,
				Flags:    annotation.FlagPrint,
			},
			Coords: [4]float64{
				pdf.Round(leftColStart, 2),
				pdf.Round(w.currentY, 2),
				pdf.Round(leftColEnd, 2),
				pdf.Round(w.currentY, 2),
			},
			BorderStyle:     lineStyle,
			LineEndingStyle: [2]annotation.LineEndingStyle{style, style},
			FillColor:       color.Red,
		}
		w.addAnnotationPair(line)

		w.currentY -= lineEndingStep
	}

	// -----------------------------------------------------------------------

	// Group 2: Border comparison test
	w.currentY -= 24 // extra gap before next group

	// line with Common.Border
	borderLine1 := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.currentY-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.currentY+10, 2),
			},
			Contents: "Common.Border with dash",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
			Border:   &annotation.Border{Width: 2, DashArray: []float64{10, 2}, SingleUse: true},
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.currentY, 2),
		},
	}
	w.addAnnotationPair(borderLine1)

	w.currentY -= borderTestStep

	// line with BorderStyle
	borderLine2 := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.currentY-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.currentY+10, 2),
			},
			Contents: "BorderStyle with dash",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.currentY, 2),
		},
		BorderStyle: &annotation.BorderStyle{Width: 2, Style: "D", DashArray: []float64{10, 2}, SingleUse: true},
	}
	w.addAnnotationPair(borderLine2)

	w.currentY -= borderTestStep

	// line with BorderStyle
	borderLine3 := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.currentY-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.currentY+10, 2),
			},
			Contents: "no line style specified",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.currentY, 2),
		},
	}
	w.addAnnotationPair(borderLine3)

	// -----------------------------------------------------------------------

	// Group 3: Caption tests
	w.currentY -= 48 // extra gap before next group

	// caption inline
	captionInline := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.currentY-15, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.currentY+15, 2),
			},
			Contents: "inline caption",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.currentY, 2),
		},
		BorderStyle: lineStyle,
		Caption:     true,
	}
	w.addAnnotationPair(captionInline)

	w.currentY -= captionTestStep

	// shifted inline caption
	captionInline = &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.currentY-30, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.currentY+30, 2),
			},
			Contents: "shifted inline caption",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.currentY, 2),
		},
		BorderStyle:   lineStyle,
		Caption:       true,
		CaptionOffset: []float64{20, 3},
	}
	w.addAnnotationPair(captionInline)

	w.currentY -= captionTestStep

	// caption on top
	captionTop := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.currentY-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.currentY+20, 2),
			},
			Contents: "top caption",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.currentY, 2),
		},
		BorderStyle:  lineStyle,
		Caption:      true,
		CaptionAbove: true,
	}
	w.addAnnotationPair(captionTop)

	// -----------------------------------------------------------------------

	// Group 4: Leader line tests
	w.currentY -= 72 // extra gap before next group

	// positive LL
	page.PushGraphicsState()
	page.SetLineWidth(5)
	page.SetStrokeColor(color.DeviceGray(0.9))
	page.MoveTo(pdf.Round(leftColStart, 2), pdf.Round(w.currentY, 2))
	page.LineTo(pdf.Round(leftColEnd, 2), pdf.Round(w.currentY, 2))
	page.Stroke()
	page.PopGraphicsState()

	leaderPos := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.currentY-30, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.currentY+30, 2),
			},
			Contents: "LL=30 (positive)",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.currentY, 2),
		},
		BorderStyle: lineStyle,
		LL:          24,
	}
	w.addAnnotationPair(leaderPos)

	w.currentY -= 36

	// negative LL
	page.PushGraphicsState()
	page.SetLineWidth(5)
	page.SetStrokeColor(color.DeviceGray(0.9))
	page.MoveTo(pdf.Round(leftColStart, 2), pdf.Round(w.currentY, 2))
	page.LineTo(pdf.Round(leftColEnd, 2), pdf.Round(w.currentY, 2))
	page.Stroke()
	page.PopGraphicsState()

	leaderNeg := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.currentY-30, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.currentY+30, 2),
			},
			Contents: "LL=-24 (negative)",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.currentY, 2),
		},
		BorderStyle: lineStyle,
		LL:          -24,
	}
	w.addAnnotationPair(leaderNeg)

	w.currentY -= 120

	// combined LL, LLE, LLO
	page.PushGraphicsState()
	page.SetLineWidth(5)
	page.SetStrokeColor(color.DeviceGray(0.9))
	page.MoveTo(pdf.Round(leftColStart, 2), pdf.Round(w.currentY, 2))
	page.LineTo(pdf.Round(leftColEnd, 2), pdf.Round(w.currentY, 2))
	page.Stroke()
	page.PopGraphicsState()

	leaderCombo := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(w.currentY-30, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(w.currentY+30, 2),
			},
			Contents: "LL=20, LLE=15, LLO=10",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(w.currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(w.currentY, 2),
		},
		BorderStyle: lineStyle,
		FillColor:   color.DeviceRGB{1, 1, 0.5},
		LLE:         10,
		LL:          50,
		LLO:         10,
	}
	w.addAnnotationPair(leaderCombo)

	return page.Close()
}

type writer struct {
	page     *document.Page
	style    *fallback.Style
	currentY float64
}

func (w *writer) addAnnotation(a annotation.Annotation) {
	w.page.Page.Annots = append(w.page.Page.Annots, a)
}

func (w *writer) addAnnotationPair(line *annotation.Line) {
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

	w.style.AddAppearance(rightLine)

	// add right annotation
	w.addAnnotation(rightLine)
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
		ColorSpace: color.SpaceDeviceRGB,
		P0:         vec.Vec2{X: pdf.Round(t0*nx, 1), Y: pdf.Round(t0*ny, 1)},
		P1:         vec.Vec2{X: pdf.Round(t1*nx, 1), Y: pdf.Round(t1*ny, 1)},
		F:          F,
		TMin:       t0,
		TMax:       t1,
	}
	return background, nil
}
