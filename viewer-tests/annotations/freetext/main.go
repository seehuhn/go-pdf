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
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/shading"
	"seehuhn.de/go/pdf/internal/gibberish"
)

const (
	leftMargin        = 72.0
	annotationWidth   = 150.0
	annotationSpacing = 170.0
	annotationHeight  = 60.0
	titleY            = 670.0
	defaultRowY       = 600.0
	styledRowY        = 500.0
	pinkRowY          = 400.0
	styledPinkRowY    = 300.0
)

// annotationConfig defines the parameters for creating a free text annotation
type annotationConfig struct {
	intent          pdf.Name
	yPos            float64
	text            string
	backgroundColor color.Color
	useStyle        bool
}

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

	titleFont := standard.Helvetica.New()

	style := fallback.NewStyle()
	pink := color.DeviceRGB(0.96, 0.87, 0.90)

	// configuration for the different annotation styling scenarios
	allIntents := []pdf.Name{
		annotation.FreeTextIntentPlain,
		annotation.FreeTextIntentCallout,
		annotation.FreeTextIntentTypeWriter,
	}

	// configuration for the four annotation rows
	configs := []annotationConfig{
		{yPos: defaultRowY, backgroundColor: nil, useStyle: false},    // viewer default style
		{yPos: styledRowY, backgroundColor: nil, useStyle: true},      // with appearance dictionary
		{yPos: pinkRowY, backgroundColor: pink, useStyle: false},      // pink background
		{yPos: styledPinkRowY, backgroundColor: pink, useStyle: true}, // styled with pink background
	}

	// create intent labels at the top
	for i, intent := range allIntents {
		err := createTitle(doc, titleFont, intent, i)
		if err != nil {
			return err
		}
	}

	// create annotations for each intent and configuration
	for i, intent := range allIntents {
		for _, config := range configs {
			annotRef, err := createFreeTextAnnotation(doc, intent, i, config, style)
			if err != nil {
				return err
			}
			annots = append(annots, annotRef)
		}
	}

	doc.PageDict["Annots"] = annots

	return doc.Close()
}

// createTitle creates the title text for an annotation type
func createTitle(doc *document.Page, titleFont *type1.Instance, intent pdf.Name, index int) error {
	doc.TextBegin()
	doc.TextSetFont(titleFont, 8)
	x := leftMargin + float64(index)*annotationSpacing
	doc.TextFirstLine(x, titleY)
	doc.TextShow(string(intent))
	doc.TextEnd()
	return nil
}

// createFreeTextAnnotation creates a free text annotation
func createFreeTextAnnotation(doc *document.Page, intent pdf.Name, index int, config annotationConfig, style *fallback.Style) (pdf.Reference, error) {
	x := leftMargin + float64(index)*annotationSpacing
	rect := pdf.Rectangle{
		LLx: x,
		LLy: config.yPos,
		URx: x + annotationWidth,
		URy: config.yPos + annotationHeight,
	}

	freeText := &annotation.FreeText{
		Common: annotation.Common{
			Rect:     rect,
			Contents: gibberish.Generate(12, uint64(index+1)),
			Color:    config.backgroundColor,
		},
		Markup: annotation.Markup{
			User:   "Test User",
			Intent: intent,
		},
		DefaultAppearance: "0 0 0 rg /Helvetica 12 Tf",
		Align:             annotation.FreeTextAlignLeft,
	}

	// Add callout line for FreeTextIntentCallout
	if intent == annotation.FreeTextIntentCallout {
		freeText.CalloutLine = []float64{
			x + 72, config.yPos - 18,
			x, config.yPos - 18,
			x, config.yPos,
		}
		freeText.LineEndingStyle = annotation.LineEndingStyleOpenArrow
	}

	if config.useStyle {
		err := style.AddAppearance(freeText, config.backgroundColor)
		if err != nil {
			return 0, err
		}
	}

	annotRef := doc.RM.Out.Alloc()
	annotNative, err := freeText.Encode(doc.RM)
	if err != nil {
		return 0, err
	}
	err = doc.RM.Out.Put(annotRef, annotNative)
	if err != nil {
		return 0, err
	}

	return annotRef, nil
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
