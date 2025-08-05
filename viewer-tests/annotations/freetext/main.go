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
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/shading"
)

const (
	annotHeight = 40.0
	annotWidth  = 100.0

	mid1     = 260.0
	mid2     = 320.0
	yMidTop  = 620.0
	yMidStep = 35.0

	leftX0     = mid1 - 100 - annotWidth
	leftX1     = mid1 - 100
	rightX0    = mid2 + 100
	rightX1    = mid2 + 100 + annotWidth
	yOuterTop  = yMidTop + 140
	yOuterStep = 50.0

	lw = 1.0
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

	background, err := pageBackground(paper)
	if err != nil {
		return err
	}
	doc.DrawShading(background)

	var annots pdf.Array

	leStyles := []annotation.LineEndingStyle{
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
	numCallout := len(leStyles)

	doc.SetLineWidth(0.5)
	doc.SetStrokeColor(color.Blue)
	doc.MoveTo(mid1, yMidTop+20)
	doc.LineTo(mid1, yMidTop-float64(numCallout-1)*yMidStep-20)
	doc.MoveTo(mid2, yMidTop+20)
	doc.LineTo(mid2, yMidTop-float64(numCallout-1)*yMidStep-20)
	for i := range leStyles {
		doc.MoveTo(mid1-20, yMidTop-float64(i)*yMidStep)
		doc.LineTo(mid2+20, yMidTop-float64(i)*yMidStep)
	}
	doc.Stroke()

	embed := func(a *annotation.FreeText) error {
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

	styler := fallback.NewStyle()

	for i, style := range leStyles {
		yMid := yMidTop - float64(i)*yMidStep
		yTopOuter := yOuterTop - float64(i)*yOuterStep

		var col color.Color
		if i%2 == 0 {
			col = color.DeviceRGB(0.98, 0.96, 0.75)
		}

		aLeft := &annotation.FreeText{
			Common: annotation.Common{
				Rect: pdf.Rectangle{
					LLx: leftX0,
					LLy: yTopOuter - annotHeight,
					URx: leftX1,
					URy: yTopOuter,
				},
				Contents: string(annotation.FreeTextIntentCallout) + "\n" + string(style),
				Flags:    annotation.FlagPrint,
				Border:   &annotation.Border{Width: lw, SingleUse: true},
				Color:    col,
			},
			Markup: annotation.Markup{
				Intent: annotation.FreeTextIntentCallout,
			},
			// Margin:          []float64{},
			CalloutLine: []float64{
				mid1, yMid,
				mid1 - 50, yTopOuter - annotHeight/2,
				leftX1, yTopOuter - annotHeight/2,
			},
			LineEndingStyle: style,
		}
		err := embed(aLeft)
		if err != nil {
			return err
		}

		aRight := &annotation.FreeText{
			Common: annotation.Common{
				Rect: pdf.Rectangle{
					LLx: rightX0,
					LLy: yTopOuter - annotHeight,
					URx: rightX1,
					URy: yTopOuter,
				},
				Contents: string(annotation.FreeTextIntentCallout) + "\n" + string(style),
				Flags:    annotation.FlagPrint,
				Border:   &annotation.Border{Width: lw, SingleUse: true},
				Color:    col,
			},
			Markup: annotation.Markup{
				Intent: annotation.FreeTextIntentCallout,
			},
			// Margin:          []float64{},
			CalloutLine: []float64{
				mid2, yMid,
				mid2 + 50, yTopOuter - annotHeight/2,
				rightX0, yTopOuter - annotHeight/2,
			},
			LineEndingStyle: style,
		}
		styler.AddAppearance(aRight)

		err = embed(aRight)
		if err != nil {
			return err
		}
	}

	doc.PageDict["Annots"] = annots

	return doc.Close()
}

func pageBackground(paper *pdf.Rectangle) (graphics.Shading, error) {
	alpha := 30.0 / 360 * 2 * math.Pi

	nx := math.Cos(alpha)
	ny := math.Sin(alpha)

	t0 := math.Round((paper.LLx*nx+paper.LLy*ny)*10) / 10
	t1 := math.Round((paper.URx*nx+paper.URy*ny)*10) / 10

	F := &function.Type4{
		Domain:  []float64{t0, t1},
		Range:   []float64{0, 1, 0, 1, 0, 1},
		Program: "dup 16 div floor 16 mul sub 8 ge {0.9216 0.9216 1}{0.8510 0.9216 1}ifelse",
	}

	background := &shading.Type2{
		ColorSpace: color.DeviceRGBSpace,
		X0:         math.Round(t0*nx*10) / 10,
		Y0:         math.Round(t0*ny*10) / 10,
		X1:         math.Round(t1*nx*10) / 10,
		Y1:         math.Round(t1*ny*10) / 10,
		F:          F,
		TMin:       t0,
		TMax:       t1,
	}
	return background, nil
}
