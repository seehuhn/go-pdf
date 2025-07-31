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
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/internal/gibberish"
)

const (
	leftMargin        = 72.0
	annotationWidth   = 150.0
	annotationSpacing = 170.0
	annotationHeight  = 60.0
	firstRowY         = 600.0
	secondRowY        = 500.0
)

// annotationConfig defines the parameters for creating a free text annotation
type annotationConfig struct {
	intent pdf.Name
	yPos   float64
	text   string
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

	var annots pdf.Array

	titleFont := standard.Helvetica.New()

	// configuration for the three different free text intents
	configs := []annotationConfig{
		{
			intent: annotation.FreeTextIntentPlain,
			yPos:   firstRowY,
			text:   gibberish.Generate(12, 1),
		},
		{
			intent: annotation.FreeTextIntentCallout,
			yPos:   firstRowY,
			text:   gibberish.Generate(12, 2),
		},
		{
			intent: annotation.FreeTextIntentTypeWriter,
			yPos:   firstRowY,
			text:   gibberish.Generate(12, 3),
		},
	}

	// create title labels
	for i, config := range configs {
		err := createTitle(doc, titleFont, config.intent, i)
		if err != nil {
			return err
		}
	}

	// create first row of annotations (viewer default appearance)
	for i, config := range configs {
		annotRef, err := createFreeTextAnnotation(doc, config, i)
		if err != nil {
			return err
		}
		annots = append(annots, annotRef)
	}

	// create second row (placeholder for future appearance streams)
	// For now, just duplicate the first row at a different Y position
	secondRowConfigs := make([]annotationConfig, len(configs))
	copy(secondRowConfigs, configs)
	for i := range secondRowConfigs {
		secondRowConfigs[i].yPos = secondRowY
	}

	for i, config := range secondRowConfigs {
		annotRef, err := createFreeTextAnnotation(doc, config, i)
		if err != nil {
			return err
		}
		annots = append(annots, annotRef)
	}

	doc.PageDict["Annots"] = annots

	return doc.Close()
}

// createTitle creates the title text for an annotation type
func createTitle(doc *document.Page, titleFont *type1.Instance, intent pdf.Name, index int) error {
	doc.TextBegin()
	doc.TextSetFont(titleFont, 8)
	x := leftMargin + float64(index)*annotationSpacing
	doc.TextFirstLine(x, firstRowY+annotationHeight+10)
	doc.TextShow(string(intent))
	doc.TextEnd()
	return nil
}

// createFreeTextAnnotation creates a free text annotation
func createFreeTextAnnotation(doc *document.Page, config annotationConfig, index int) (pdf.Reference, error) {
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
			Contents: config.text,
		},
		Markup: annotation.Markup{
			User:   "Test User",
			Intent: config.intent,
		},
		DefaultAppearance: "0 0 0 rg /Helvetica 12 Tf",
		Align:             annotation.FreeTextAlignLeft,
	}

	// Add callout line for FreeTextIntentCallout
	if config.intent == annotation.FreeTextIntentCallout {
		freeText.CalloutLine = []float64{
			x + 72, config.yPos - 18,
			x, config.yPos - 18,
			x, config.yPos,
		}
		freeText.LineEndingStyle = annotation.LineEndingStyleOpenArrow
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
