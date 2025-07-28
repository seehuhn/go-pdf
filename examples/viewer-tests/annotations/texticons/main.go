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

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/extended"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/form"
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

	iconFont := extended.NimbusRomanBold.New()

	xComment := &form.Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceRGB(0.98, 0.96, 0.75))
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.Fill()

			w.TextBegin()
			w.TextSetFont(iconFont, 23)
			w.SetFillColor(color.DeviceGray(0.0))
			w.TextFirstLine(6, 2)
			w.TextSetHorizontalScaling(0.9)
			w.TextShow("“")
			w.TextEnd()

			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.Stroke()
			return nil
		},
		Matrix: matrix.Identity,
		BBox:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
	}

	xKey := &form.Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceRGB(0.98, 0.96, 0.75))
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.Fill()

			w.TextBegin()
			w.TextSetFont(iconFont, 23)
			w.SetFillColor(color.DeviceGray(0.0))
			w.TextFirstLine(6.5, 1)
			w.TextShow("*")
			w.TextEnd()

			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.Stroke()
			return nil
		},
		Matrix: matrix.Identity,
		BBox:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
	}

	xNote := &form.Form{
		Draw: func(w *graphics.Writer) error {
			delta := 7.0

			w.SetFillColor(color.DeviceRGB(0.98, 0.96, 0.75))
			w.MoveTo(23.5-delta, 0.25)
			w.LineTo(0.25, 0.25)
			w.LineTo(0.25, 23.5)
			w.LineTo(23.5, 23.5)
			w.LineTo(23.5, 0.25+delta)
			w.Fill()

			w.SetLineWidth(1.5)
			w.SetStrokeColor(color.DeviceGray(0.4))
			for y := 19.; y > 6; y -= 3.5 {
				w.MoveTo(4, y)
				if y > 10 {
					w.LineTo(17, y)
				} else {
					w.LineTo(12, y)
				}
			}
			w.Stroke()

			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.MoveTo(23.5-delta, 0.25)
			w.LineTo(0.25, 0.25)
			w.LineTo(0.25, 23.5)
			w.LineTo(23.5, 23.5)
			w.LineTo(23.5, 0.25+delta)
			w.LineTo(23.5-delta, 0.25)
			w.LineTo(23.5-delta, 0.25+delta)
			w.LineTo(23.5, 0.25+delta)
			w.Stroke()
			return nil
		},
		Matrix: matrix.Identity,
		BBox:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
	}

	xHelp := &form.Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceRGB(0.98, 0.96, 0.75))
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.Fill()

			w.TextBegin()
			w.TextSetFont(iconFont, 23)
			w.SetFillColor(color.DeviceGray(0.0))
			w.TextFirstLine(7, 4)
			w.TextShow("?")
			w.TextEnd()

			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.Stroke()
			return nil
		},
		Matrix: matrix.Identity,
		BBox:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
	}

	xNewParagraph := &form.Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceRGB(0.98, 0.96, 0.75))
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.Fill()

			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.Stroke()
			return nil
		},
		Matrix: matrix.Identity,
		BBox:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
	}

	xParagraph := &form.Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceRGB(0.98, 0.96, 0.75))
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.Fill()

			w.TextBegin()
			w.TextSetFont(iconFont, 16)
			w.SetFillColor(color.DeviceGray(0.0))
			w.TextFirstLine(6, 8)
			w.TextSetHorizontalScaling(1.4)
			w.TextShow("¶")
			w.TextEnd()

			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.Stroke()
			return nil
		},
		Matrix: matrix.Identity,
		BBox:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
	}

	xInsert := &form.Form{
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceRGB(0.98, 0.96, 0.75))
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.Fill()
			w.SetLineWidth(0.5)
			w.SetStrokeColor(color.DeviceGray(0.2))
			w.Rectangle(0.25, 0.25, 23.5, 23.5)
			w.Stroke()
			return nil
		},
		Matrix: matrix.Identity,
		BBox:   pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
	}

	doc.PushGraphicsState()
	doc.Transform(matrix.Translate(36, 250))
	doc.DrawXObject(xComment)
	doc.Transform(matrix.Translate(50, 0))
	doc.DrawXObject(xKey)
	doc.Transform(matrix.Translate(50, 0))
	doc.DrawXObject(xNote)
	doc.Transform(matrix.Translate(50, 0))
	doc.DrawXObject(xHelp)
	doc.Transform(matrix.Translate(50, 0))
	doc.DrawXObject(xNewParagraph)
	doc.Transform(matrix.Translate(50, 0))
	doc.DrawXObject(xParagraph)
	doc.Transform(matrix.Translate(50, 0))
	doc.DrawXObject(xInsert)
	doc.PopGraphicsState()

	var annots pdf.Array

	titleFont := standard.Helvetica.New()

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
		textRef := doc.RM.Out.Alloc()
		popupRef := doc.RM.Out.Alloc()

		doc.TextBegin()
		doc.TextSetFont(titleFont, 8)
		if len(icon) > 8 {
			doc.TextSetHorizontalScaling(0.8)
		}
		doc.TextFirstLine(36+float64(i)*50, 290)
		doc.TextShow(string(icon))
		doc.TextEnd()

		rect := pdf.Rectangle{LLx: 36 + float64(i)*50, LLy: 300, URx: 36 + float64(i+1)*50, URy: 325}
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
	}

	doc.PageDict["Annots"] = annots

	return doc.Close()
}
