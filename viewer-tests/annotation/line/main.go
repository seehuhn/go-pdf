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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"

	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics/color"
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
	doc, err := document.CreateSinglePage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	var annots pdf.Array

	embed := func(a annotation.Annotation) error {
		dict, err := a.Encode(doc.RM)
		if err != nil {
			return err
		}
		ref := doc.RM.Out.Alloc()
		err = doc.RM.Out.Put(ref, dict)
		if err != nil {
			return err
		}
		annots = append(annots, ref)
		return nil
	}

	lineStyle := &annotation.BorderStyle{
		Width: defaultLineWidth,
	}

	// styler := fallback.NewStyle()

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

	currentY := startY
	for _, style := range lineEndingStyles {
		line := &annotation.Line{
			Common: annotation.Common{
				Rect: pdf.Rectangle{
					LLx: pdf.Round(leftColStart-10, 2),
					LLy: pdf.Round(currentY-10, 2),
					URx: pdf.Round(leftColEnd+10, 2),
					URy: pdf.Round(currentY+10, 2),
				},
				Contents: string(style),
				Color:    color.Black,
				Flags:    annotation.FlagPrint,
			},
			Coords: [4]float64{
				pdf.Round(leftColStart, 2),
				pdf.Round(currentY, 2),
				pdf.Round(leftColEnd, 2),
				pdf.Round(currentY, 2),
			},
			BorderStyle:     lineStyle,
			LineEndingStyle: [2]annotation.LineEndingStyle{style, style},
			FillColor:       color.Red,
		}
		err = addAnnotationPair(line, embed)
		if err != nil {
			return err
		}

		currentY -= lineEndingStep
	}

	// -----------------------------------------------------------------------

	// Group 2: Border comparison test
	currentY -= 24 // extra gap before next group

	// line with Common.Border
	borderLine1 := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(currentY-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(currentY+10, 2),
			},
			Contents: "Common.Border with dash",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
			Border:   &annotation.Border{Width: 2, DashArray: []float64{10, 2}, SingleUse: true},
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(currentY, 2),
		},
	}
	err = addAnnotationPair(borderLine1, embed)
	if err != nil {
		return err
	}

	currentY -= borderTestStep

	// line with BorderStyle
	borderLine2 := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(currentY-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(currentY+10, 2),
			},
			Contents: "BorderStyle with dash",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(currentY, 2),
		},
		BorderStyle: &annotation.BorderStyle{Width: 2, Style: "D", DashArray: []float64{10, 2}, SingleUse: true},
	}
	err = addAnnotationPair(borderLine2, embed)
	if err != nil {
		return err
	}

	currentY -= borderTestStep

	// line with BorderStyle
	borderLine3 := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(currentY-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(currentY+10, 2),
			},
			Contents: "no line style specified",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(currentY, 2),
		},
	}
	err = addAnnotationPair(borderLine3, embed)
	if err != nil {
		return err
	}

	// -----------------------------------------------------------------------

	// Group 3: Caption tests
	currentY -= 48 // extra gap before next group

	// caption inline
	captionInline := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(currentY-15, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(currentY+15, 2),
			},
			Contents: "inline caption",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(currentY, 2),
		},
		BorderStyle: lineStyle,
		Caption:     true,
	}
	err = addAnnotationPair(captionInline, embed)
	if err != nil {
		return err
	}

	currentY -= captionTestStep

	// shifted inline caption
	captionInline = &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(currentY-30, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(currentY+30, 2),
			},
			Contents: "shifted inline caption",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(currentY, 2),
		},
		BorderStyle:   lineStyle,
		Caption:       true,
		CaptionOffset: []float64{20, 3},
	}
	err = addAnnotationPair(captionInline, embed)
	if err != nil {
		return err
	}

	currentY -= captionTestStep

	// caption on top
	captionTop := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(currentY-10, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(currentY+20, 2),
			},
			Contents: "top caption",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(currentY, 2),
		},
		BorderStyle:  lineStyle,
		Caption:      true,
		CaptionAbove: true,
	}
	err = addAnnotationPair(captionTop, embed)
	if err != nil {
		return err
	}

	// -----------------------------------------------------------------------

	// Group 4: Leader line tests
	currentY -= 72 // extra gap before next group

	// positive LL
	doc.PushGraphicsState()
	doc.SetLineWidth(5)
	doc.SetStrokeColor(color.DeviceGray(0.9))
	doc.MoveTo(pdf.Round(leftColStart, 2), pdf.Round(currentY, 2))
	doc.LineTo(pdf.Round(leftColEnd, 2), pdf.Round(currentY, 2))
	doc.Stroke()
	doc.PopGraphicsState()

	leaderPos := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(currentY-30, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(currentY+30, 2),
			},
			Contents: "LL=30 (positive)",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(currentY, 2),
		},
		BorderStyle: lineStyle,
		LL:          24,
	}
	err = addAnnotationPair(leaderPos, embed)
	if err != nil {
		return err
	}

	currentY -= 36

	// negative LL
	doc.PushGraphicsState()
	doc.SetLineWidth(5)
	doc.SetStrokeColor(color.DeviceGray(0.9))
	doc.MoveTo(pdf.Round(leftColStart, 2), pdf.Round(currentY, 2))
	doc.LineTo(pdf.Round(leftColEnd, 2), pdf.Round(currentY, 2))
	doc.Stroke()
	doc.PopGraphicsState()

	leaderNeg := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(currentY-30, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(currentY+30, 2),
			},
			Contents: "LL=-24 (negative)",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(currentY, 2),
		},
		BorderStyle: lineStyle,
		LL:          -24,
	}
	err = addAnnotationPair(leaderNeg, embed)
	if err != nil {
		return err
	}

	currentY -= 120

	// combined LL, LLE, LLO
	doc.PushGraphicsState()
	doc.SetLineWidth(5)
	doc.SetStrokeColor(color.DeviceGray(0.9))
	doc.MoveTo(pdf.Round(leftColStart, 2), pdf.Round(currentY, 2))
	doc.LineTo(pdf.Round(leftColEnd, 2), pdf.Round(currentY, 2))
	doc.Stroke()
	doc.PopGraphicsState()

	leaderCombo := &annotation.Line{
		Common: annotation.Common{
			Rect: pdf.Rectangle{
				LLx: pdf.Round(leftColStart-10, 2),
				LLy: pdf.Round(currentY-30, 2),
				URx: pdf.Round(leftColEnd+10, 2),
				URy: pdf.Round(currentY+30, 2),
			},
			Contents: "LL=20, LLE=15, LLO=10",
			Color:    color.Black,
			Flags:    annotation.FlagPrint,
		},
		Coords: [4]float64{
			pdf.Round(leftColStart, 2),
			pdf.Round(currentY, 2),
			pdf.Round(leftColEnd, 2),
			pdf.Round(currentY, 2),
		},
		BorderStyle: lineStyle,
		FillColor:   color.DeviceRGB(1, 1, 0.5),
		LLE:         10,
		LL:          50,
		LLO:         10,
	}
	err = addAnnotationPair(leaderCombo, embed)
	if err != nil {
		return err
	}

	doc.PageDict["Annots"] = annots

	return doc.Close()
}

func addAnnotationPair(line *annotation.Line, embed func(annotation.Annotation) error) error {
	// embed left annotation as-is
	err := embed(line)
	if err != nil {
		return err
	}

	// create shallow copy for right column
	rightLine := clone(line)

	// adjust coordinates for right column
	deltaX := rightColStart - leftColStart
	rightLine.Rect.LLx += deltaX
	rightLine.Rect.URx += deltaX
	rightLine.Coords[0] += deltaX
	rightLine.Coords[2] += deltaX

	// styler.AddAppearance(rightLine)

	// embed right annotation
	return embed(rightLine)
}

func clone[T any](v *T) *T {
	if v == nil {
		return nil
	}
	clone := *v
	return &clone
}
