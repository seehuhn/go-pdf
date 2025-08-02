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
	"seehuhn.de/go/pdf/annotation/fallback"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics/color"
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

const (
	leftMargin     = 72.0
	iconSpacing    = 50.0
	iconSize       = 24.0
	titleY         = 340.0
	defaultRowY    = 300.0
	styledRowY     = 250.0
	pinkRowY       = 150.0
	styledPinkRowY = 100.0
)

// annotationConfig defines the parameters for creating an annotation pair
type annotationConfig struct {
	yPos            float64
	backgroundColor color.Color
	useStyle        bool
}

func createDocument(filename string) error {
	paper := document.A5r
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	doc, err := document.CreateSinglePage(filename, paper, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	var annots pdf.Array

	titleFont := standard.Helvetica.New()

	style := fallback.NewStyle()
	pink := color.DeviceRGB(0.96, 0.87, 0.90)

	// configuration for the four annotation rows
	configs := []annotationConfig{
		{yPos: defaultRowY, backgroundColor: nil, useStyle: false},    // viewer default style
		{yPos: styledRowY, backgroundColor: nil, useStyle: true},      // with appearance dictionary
		{yPos: pinkRowY, backgroundColor: pink, useStyle: false},      // pink background
		{yPos: styledPinkRowY, backgroundColor: pink, useStyle: true}, // styled with pink background
	}

	allIcons := []annotation.TextIcon{
		annotation.TextIconComment,
		annotation.TextIconKey,
		annotation.TextIconNote,
		annotation.TextIconHelp,
		annotation.TextIconNewParagraph,
		annotation.TextIconParagraph,
		annotation.TextIconInsert,
	}
	// create icon labels at the top
	for i, icon := range allIcons {
		err := createIconLabel(doc, titleFont, icon, i)
		if err != nil {
			return err
		}
	}

	// create annotations for each icon and configuration
	for i, icon := range allIcons {
		for _, config := range configs {
			textRef, popupRef, err := createTextAnnotationPair(doc, icon, i, config, style)
			if err != nil {
				return err
			}
			annots = append(annots, textRef, popupRef)
		}
	}

	doc.PageDict["Annots"] = annots

	return doc.Close()
}

// createIconLabel creates the title text for an icon
func createIconLabel(doc *document.Page, titleFont *type1.Instance, icon annotation.TextIcon, index int) error {
	doc.TextBegin()
	doc.TextSetFont(titleFont, 8)
	if len(icon) > 8 {
		doc.TextSetHorizontalScaling(0.8)
	}
	doc.TextFirstLine(leftMargin+float64(index)*iconSpacing, titleY)
	doc.TextShow(string(icon))
	doc.TextEnd()
	return nil
}

// createTextAnnotationPair creates a text annotation and its popup
func createTextAnnotationPair(doc *document.Page, icon annotation.TextIcon, index int, config annotationConfig, style *fallback.Style) (pdf.Reference, pdf.Reference, error) {
	textRef := doc.RM.Out.Alloc()
	popupRef := doc.RM.Out.Alloc()

	x := leftMargin + float64(index)*iconSpacing
	rect := pdf.Rectangle{LLx: x, LLy: config.yPos, URx: x + iconSize, URy: config.yPos + iconSize}

	popup := &annotation.Popup{
		Common: annotation.Common{
			Rect:  rect,
			Color: config.backgroundColor,
		},
		Parent: textRef,
	}

	text := &annotation.Text{
		Common: annotation.Common{
			Rect:     rect,
			Contents: fmt.Sprintf("Icon name %q", icon),
			Color:    config.backgroundColor,
		},
		Markup: annotation.Markup{
			User:  "Jochen Voss",
			Popup: popupRef,
		},
		Icon: icon,
	}

	if config.useStyle {
		style.AddAppearance(text, config.backgroundColor)
	}

	textNative, err := text.Encode(doc.RM)
	if err != nil {
		return 0, 0, err
	}
	err = doc.RM.Out.Put(textRef, textNative)
	if err != nil {
		return 0, 0, err
	}

	popupNative, err := popup.Encode(doc.RM)
	if err != nil {
		return 0, 0, err
	}
	err = doc.RM.Out.Put(popupRef, popupNative)
	if err != nil {
		return 0, 0, err
	}

	return textRef, popupRef, nil
}
