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
)

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
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

	all := []annotation.TextIcon{
		annotation.TextIconComment,
		annotation.TextIconKey,
		annotation.TextIconNote,
		annotation.TextIconHelp,
		annotation.TextIconNewParagraph,
		annotation.TextIconParagraph,
		annotation.TextIconInsert,
	}
	for i, icon := range all {
		doc.TextBegin()
		doc.TextSetFont(titleFont, 8)
		if len(icon) > 8 {
			doc.TextSetHorizontalScaling(0.8)
		}
		doc.TextFirstLine(36+float64(i)*50, 340)
		doc.TextShow(string(icon))
		doc.TextEnd()

		// First show the annotation in the viewer's default style.

		textRef := doc.RM.Out.Alloc()
		popupRef := doc.RM.Out.Alloc()

		rect := pdf.Rectangle{LLx: 36 + float64(i)*50, LLy: 300, URx: 36 + float64(i)*50 + 25, URy: 300 + 25}
		popup := &annotation.Popup{
			Common: annotation.Common{
				Rect: rect,
			},
			Parent: textRef,
		}
		text := &annotation.Text{
			Common: annotation.Common{
				Rect:     rect,
				Contents: fmt.Sprintf("Icon name %q", icon),
			},
			Markup: annotation.Markup{
				User:  "Jochen Voss",
				Popup: popupRef,
			},
			Icon: icon,
		}
		textNative, err := text.AsDict(doc.RM)
		if err != nil {
			return err
		}
		err = doc.RM.Out.Put(textRef, textNative)
		if err != nil {
			return err
		}

		popupNative, err := popup.AsDict(doc.RM)
		if err != nil {
			return err
		}
		err = doc.RM.Out.Put(popupRef, popupNative)
		if err != nil {
			return err
		}

		annots = append(annots, textRef, popupRef)

		// then show the annotation with our appearance dictionary.

		textRef = doc.RM.Out.Alloc()
		popupRef = doc.RM.Out.Alloc()

		rect = pdf.Rectangle{LLx: 36 + float64(i)*50, LLy: 250, URx: 36 + float64(i)*50 + 25, URy: 250 + 25}
		popup = &annotation.Popup{
			Common: annotation.Common{
				Rect: rect,
			},
			Parent: textRef,
		}
		text = &annotation.Text{
			Common: annotation.Common{
				Rect:     rect,
				Contents: fmt.Sprintf("Icon name %q", icon),
			},
			Markup: annotation.Markup{
				User:  "Jochen Voss",
				Popup: popupRef,
			},
			Icon: icon,
		}
		style.AddAppearance(text)
		textNative, err = text.AsDict(doc.RM)
		if err != nil {
			return err
		}
		err = doc.RM.Out.Put(textRef, textNative)
		if err != nil {
			return err
		}

		popupNative, err = popup.AsDict(doc.RM)
		if err != nil {
			return err
		}
		err = doc.RM.Out.Put(popupRef, popupNative)
		if err != nil {
			return err
		}

		annots = append(annots, textRef, popupRef)
	}

	doc.PageDict["Annots"] = annots

	return doc.Close()
}
