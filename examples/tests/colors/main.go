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
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
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

	err = showCalRGBColors(doc, F)
	if err != nil {
		return err
	}

	err = showLabColors(doc, F)
	if err != nil {
		return err
	}

	err = showIndexed(doc, F)
	if err != nil {
		return err
	}

	err = showTilingPatternUncolored(doc, F)
	if err != nil {
		return err
	}

	err = showTilingPatternColored(doc, F)
	if err != nil {
		return err
	}

	err = showShadingPattern(doc, F)
	if err != nil {
		return err
	}

	err = doc.Close()
	if err != nil {
		return err
	}

	return nil
}

func showCalRGBColors(doc *document.MultiPage, F font.Layouter) error {
	page := doc.AddPage()

	CalRGB, err := color.CalRGB(color.WhitePointD65, nil, nil, nil, "")
	if err != nil {
		return err
	}

	ref := page.Out.Alloc()
	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(256),
		"Height":           pdf.Integer(256),
		"ColorSpace":       CalRGB.PDFObject(),
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

	hTickLabel(page, F, 50, 300, "g=0.0")
	hTickLabel(page, F, 300, 300, "g=0.5")
	hTickLabel(page, F, 550, 300, "g=1.0")
	vTickLabel(page, F, 50, 300, "b=0.0")
	vTickLabel(page, F, 50, 550, "b=0.5")
	vTickLabel(page, F, 50, 800, "b=1.0")

	page.TextSetFont(F, 12)
	page.TextStart()
	page.TextFirstLine(50, 230)
	page.TextShow("Colors in a CIE-based RGB color space, for r=0.5 (color space CalRGB).")
	page.TextEnd()

	err = page.Close()
	if err != nil {
		return err
	}

	return nil
}

func showLabColors(doc *document.MultiPage, F font.Layouter) error {
	Lab, err := color.Lab(color.WhitePointD65, nil, nil, "")
	if err != nil {
		return err
	}

	ref := doc.Out.Alloc()
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
	stm, err := doc.Out.OpenStream(ref, dict, compress)
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

	page := doc.AddPage()

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

func showIndexed(doc *document.MultiPage, F font.Layouter) error {
	var cc []color.Color

	lab, err := color.Lab(color.WhitePointD65, nil, nil, "")
	if err != nil {
		return err
	}

	numColors := 32

	bases := []int{2, 3, 5}
	for i := 0; i < numColors; i++ {
		var x [3]float64
		for j := 0; j < 3; j++ {
			x[j] = halton(i, bases[j])
		}
		col, err := lab.New(x[0]*100, x[1]*200-100, x[2]*200-100)
		if err != nil {
			return err
		}
		cc = append(cc, col)
	}
	cs, err := color.Indexed(cc, "")
	if err != nil {
		return err
	}

	ref := doc.Out.Alloc()
	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(numColors),
		"Height":           pdf.Integer(1),
		"ColorSpace":       cs.PDFObject(),
		"BitsPerComponent": pdf.Integer(8),
	}
	compress := pdf.FilterFlate{
		"Predictor": pdf.Integer(12),
		"Colors":    pdf.Integer(1),
		"Columns":   pdf.Integer(numColors),
	}
	stm, err := doc.Out.OpenStream(ref, dict, compress)
	if err != nil {
		return err
	}
	for i := 0; i < numColors; i++ {
		_, err := stm.Write([]byte{byte(i)})
		if err != nil {
			return err
		}
	}
	err = stm.Close()
	if err != nil {
		return err
	}
	img := &pdf.Res{Ref: ref}

	page := doc.AddPage()

	page.PushGraphicsState()
	M := graphics.Scale(500, 100)
	M = M.Mul(graphics.Translate(50, 300))
	page.Transform(M)
	page.DrawImage(img)
	page.PopGraphicsState()

	page.TextSetFont(F, 12)
	page.TextStart()
	page.TextFirstLine(50, 230)
	page.TextShow("Colors in an indexed color space (color space ‘Indexed’).")
	page.TextEnd()

	err = page.Close()
	if err != nil {
		return err
	}

	return nil
}

func halton(i, base int) float64 {
	f := 1.0
	r := 0.0
	for i > 0 {
		f /= float64(base)
		r += f * float64(i%base)
		i /= base
	}
	return r
}

func showTilingPatternUncolored(doc *document.MultiPage, F font.Layouter) error {
	// 1/2^2 + (x/2)^2 = 1^2   =>   x = 2*sqrt(3)/2 = sqrt(3)

	w := 12.0
	h := w * math.Sqrt(3)
	r := 0.3 * w

	prop := graphics.TilingProperties{
		Colored:    false,
		TilingType: 1,
		BBox:       &pdf.Rectangle{URx: w, URy: h},
		XStep:      w,
		YStep:      h,
		Matrix:     graphics.IdentityMatrix,
	}
	builder := graphics.NewTilingPattern(doc.Out, prop)

	builder.Circle(0, 0, r)
	builder.Circle(w, 0, r)
	builder.Circle(w/2, h/2, r)
	builder.Circle(0, h, r)
	builder.Circle(w, h, r)
	builder.Fill()

	pat, err := builder.Make()
	if err != nil {
		return err
	}

	col := color.NewTilingPatternUncolored(pat, color.DeviceRGB.New(1, 0, 0))

	page := doc.AddPage()

	page.PushGraphicsState()
	page.SetFillColor(col)
	page.Rectangle(50, 300, 500, 500)
	page.FillAndStroke()
	page.PopGraphicsState()

	page.TextSetFont(F, 12)
	page.TextStart()
	page.TextFirstLine(50, 230)
	page.TextShow("A square filled with an uncolored tiling pattern (color space ’Pattern’).")
	page.TextEnd()

	err = page.Close()
	if err != nil {
		return err
	}

	return nil
}

func showTilingPatternColored(doc *document.MultiPage, F font.Layouter) error {
	// 1/2^2 + (x/2)^2 = 1^2   =>   x = 2*sqrt(3)/2 = sqrt(3)

	w := 12.0
	h := w * math.Sqrt(3)
	r := 0.3 * w

	prop := graphics.TilingProperties{
		Colored:    true,
		TilingType: 1,
		BBox:       &pdf.Rectangle{URx: w, URy: h},
		XStep:      w,
		YStep:      h,
		Matrix:     graphics.IdentityMatrix,
	}
	builder := graphics.NewTilingPattern(doc.Out, prop)

	builder.SetFillColor(color.DeviceGray.New(0.5))
	builder.Circle(0, 0, r)
	builder.Circle(w, 0, r)
	builder.Circle(0, h, r)
	builder.Circle(w, h, r)
	builder.Fill()
	builder.SetFillColor(color.DeviceRGB.New(1, 0, 0))
	builder.Circle(w/2, h/2, r)
	builder.Fill()

	pat, err := builder.Make()
	if err != nil {
		return err
	}

	col := color.NewTilingPatternColored(pat)

	page := doc.AddPage()

	page.PushGraphicsState()
	page.SetFillColor(col)
	page.Rectangle(50, 300, 500, 500)
	page.FillAndStroke()
	page.PopGraphicsState()

	page.TextSetFont(F, 12)
	page.TextStart()
	page.TextFirstLine(50, 230)
	page.TextShow("A square filled with a colored tiling pattern (color space ’Pattern’).")
	page.TextEnd()

	err = page.Close()
	if err != nil {
		return err
	}

	return nil
}

func showShadingPattern(doc *document.MultiPage, F font.Layouter) error {
	shadeDict := pdf.Dict{
		"ShadingType": pdf.Integer(3),
		"ColorSpace":  pdf.Name("DeviceRGB"),
		"Background":  pdf.Array{pdf.Number(1), pdf.Number(1), pdf.Number(0)},
		"Coords": pdf.Array{
			pdf.Integer(100), pdf.Integer(350), pdf.Integer(10),
			pdf.Integer(500), pdf.Integer(750), pdf.Integer(200),
		},
		"Function": pdf.Dict{
			"FunctionType": pdf.Integer(2),
			"Domain":       pdf.Array{pdf.Integer(0), pdf.Integer(1)},
			"C0":           pdf.Array{pdf.Real(1), pdf.Real(0), pdf.Real(0)},
			"C1":           pdf.Array{pdf.Real(0), pdf.Real(1), pdf.Real(0)},
			"N":            pdf.Real(1),
		},
		"Extend": pdf.Array{pdf.Boolean(true), pdf.Boolean(true)},
	}
	col, err := graphics.NewShadingPattern(doc.Out, shadeDict, graphics.IdentityMatrix, nil)
	if err != nil {
		return err
	}

	page := doc.AddPage()

	page.PushGraphicsState()
	page.SetFillColor(col)
	page.Rectangle(50, 300, 500, 500)
	page.FillAndStroke()
	page.PopGraphicsState()

	page.TextSetFont(F, 12)
	page.TextStart()
	page.TextFirstLine(50, 230)
	page.TextShow("A square filled with a shading pattern (color space ’Pattern’).")
	page.TextEnd()

	err = page.Close()
	if err != nil {
		return err
	}

	return nil
}

var black = color.DeviceGray.New(0.0)

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
