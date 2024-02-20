// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	doc, err := document.CreateMultiPage("test.pdf", document.A4, nil)
	if err != nil {
		return err
	}

	F, err := type1.Helvetica.Embed(doc.Out, nil)
	if err != nil {
		return err
	}

	err = showLabColors(doc, F)
	if err != nil {
		return err
	}

	err = doc.Close()
	if err != nil {
		return err
	}

	return nil
}

func showLabColors(doc *document.MultiPage, F font.Layouter) error {
	page := doc.AddPage()

	Lab, err := graphics.Lab(graphics.WhitePointD65, nil, nil, "")
	if err != nil {
		return err
	}

	ref := page.Out.Alloc()
	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(256),
		"Height":           pdf.Integer(256),
		"ColorSpace":       Lab.PDFObject(),
		"BitsPerComponent": pdf.Integer(8),
	}
	compress := pdf.FilterFlate{
		"Predictor": pdf.Integer(12),
		"Colors":    pdf.Integer(3),
		"Columns":   pdf.Integer(256),
	}
	stm, err := page.Out.OpenStream(ref, dict, compress)
	if err != nil {
		return err
	}
	for i := 0; i < 256; i++ {
		for j := 0; j < 256; j++ {
			_, err := stm.Write([]byte{128, byte(j), byte(i)})
			if err != nil {
				return err
			}
		}
	}
	err = stm.Close()
	if err != nil {
		return err
	}
	img := &pdf.Res{Ref: ref}

	page.PushGraphicsState()
	M := graphics.Scale(500, -500)
	M = M.Mul(graphics.Translate(50, 800))
	page.Transform(M)
	page.DrawImage(img)
	page.PopGraphicsState()

	hTickLabel(page, F, 50, 300, "a*=-100")
	hTickLabel(page, F, 300, 300, "a*=0")
	hTickLabel(page, F, 550, 300, "a*=100")
	vTickLabel(page, F, 50, 300, "b*=-100")
	vTickLabel(page, F, 50, 550, "b*=0")
	vTickLabel(page, F, 50, 800, "b*=100")

	page.TextSetFont(F, 12)
	page.TextStart()
	page.TextFirstLine(50, 230)
	page.TextShow("Colors in the CIE L*a*b* color space, for L*=50 (color space ‘Lab’).")
	page.TextEnd()

	err = page.Close()
	if err != nil {
		return err
	}

	return nil
}

var black = graphics.DeviceGrayNew(0.0)

func hTickLabel(page *document.Page, F font.Layouter, x, y float64, label string) {
	page.SetStrokeColor(black)
	page.SetLineWidth(0.5)
	page.MoveTo(x, y+3)
	page.LineTo(x, y-3)
	page.Stroke()

	geom := F.GetGeometry()

	page.SetFillColor(black)
	page.TextSetFont(F, 10)
	gg := F.Layout(10, label)
	w := gg.TotalWidth()
	page.TextStart()
	page.TextFirstLine(x-w/2, y-5-10*geom.Ascent)
	page.TextShowGlyphs(gg)
	page.TextEnd()
}

func vTickLabel(page *document.Page, F font.Layouter, x, y float64, label string) {
	page.SetStrokeColor(black)
	page.SetLineWidth(0.5)
	page.MoveTo(x+3, y)
	page.LineTo(x-3, y)
	page.Stroke()

	geom := F.GetGeometry()
	mid := (geom.Ascent + geom.Descent) / 2

	page.SetFillColor(black)
	page.TextSetFont(F, 10)
	gg := F.Layout(10, label)
	w := gg.TotalWidth()
	page.TextStart()
	page.TextFirstLine(x-5-w, y-10*mid)
	page.TextShowGlyphs(gg)
	page.TextEnd()
}
