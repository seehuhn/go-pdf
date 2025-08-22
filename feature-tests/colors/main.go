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
	goimage "image"
	"log"
	"math"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/graphics/pattern"
	"seehuhn.de/go/pdf/graphics/shading"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	doc, err := document.CreateMultiPage("test.pdf", document.A4, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	F := standard.Helvetica.New()

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

	err = showShading(doc, F)
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

	CalRGB, err := color.CalRGB(color.WhitePointD65, nil, nil, nil)
	if err != nil {
		return err
	}

	imgData := goimage.NewNRGBA(goimage.Rect(0, 0, 256, 256))
	for i := range 256 {
		for j := range 256 {
			idx := i*imgData.Stride + j*4
			imgData.Pix[idx+0] = 128
			imgData.Pix[idx+1] = uint8(j)
			imgData.Pix[idx+2] = uint8(i)
			imgData.Pix[idx+3] = 255
		}
	}
	img := &image.PNG{
		Data:       imgData,
		ColorSpace: CalRGB,
	}

	page.PushGraphicsState()
	M := matrix.Scale(500, -500)
	M = M.Mul(matrix.Translate(50, 800))
	page.Transform(M)
	page.DrawXObject(img)
	page.PopGraphicsState()

	hTickLabel(page, F, 50, 300, "g=0.0")
	hTickLabel(page, F, 300, 300, "g=0.5")
	hTickLabel(page, F, 550, 300, "g=1.0")
	vTickLabel(page, F, 50, 300, "b=0.0")
	vTickLabel(page, F, 50, 550, "b=0.5")
	vTickLabel(page, F, 50, 800, "b=1.0")

	page.TextSetFont(F, 12)
	page.TextBegin()
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
	Lab, err := color.Lab(color.WhitePointD65, nil, nil)
	if err != nil {
		return err
	}

	imgData := goimage.NewNRGBA(goimage.Rect(0, 0, 256, 256))
	for i := range 256 {
		for j := range 256 {
			idx := i*imgData.Stride + j*4
			imgData.Pix[idx+0] = 128
			imgData.Pix[idx+1] = uint8(j)
			imgData.Pix[idx+2] = uint8(i)
			imgData.Pix[idx+3] = 255
		}
	}
	img := &image.PNG{
		Data:       imgData,
		ColorSpace: Lab,
	}

	page := doc.AddPage()

	page.PushGraphicsState()
	M := matrix.Scale(500, -500)
	M = M.Mul(matrix.Translate(50, 800))
	page.Transform(M)
	page.DrawXObject(img)
	page.PopGraphicsState()

	hTickLabel(page, F, 50, 300, "a*=-100")
	hTickLabel(page, F, 300, 300, "a*=0")
	hTickLabel(page, F, 550, 300, "a*=100")
	vTickLabel(page, F, 50, 300, "b*=-100")
	vTickLabel(page, F, 50, 550, "b*=0")
	vTickLabel(page, F, 50, 800, "b*=100")

	page.TextSetFont(F, 12)
	page.TextBegin()
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

	lab, err := color.Lab(color.WhitePointD65, nil, nil)
	if err != nil {
		return err
	}

	numColors := 32

	bases := []int{2, 3, 5}
	for i := range numColors {
		var x [3]float64
		for j := range 3 {
			x[j] = halton(i, bases[j])
		}
		col, err := lab.New(x[0]*100, x[1]*200-100, x[2]*200-100)
		if err != nil {
			return err
		}
		cc = append(cc, col)
	}
	cs, err := color.Indexed(cc)
	if err != nil {
		return err
	}

	img := image.NewIndexed(numColors, 1, cs)
	for i := range numColors {
		img.Pix[i] = uint8(i)
	}

	page := doc.AddPage()

	page.PushGraphicsState()
	M := matrix.Scale(500, 100)
	M = M.Mul(matrix.Translate(50, 300))
	page.Transform(M)
	page.DrawXObject(img)
	page.PopGraphicsState()

	page.TextSetFont(F, 12)
	page.TextBegin()
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

	pat := &pattern.Type1{
		TilingType: 1,
		BBox:       &pdf.Rectangle{URx: w, URy: h},
		XStep:      w,
		YStep:      h,
		Matrix:     matrix.Identity,
		Color:      false,
		Draw: func(builder *graphics.Writer) error {
			builder.Circle(0, 0, r)
			builder.Circle(w, 0, r)
			builder.Circle(w/2, h/2, r)
			builder.Circle(0, h, r)
			builder.Circle(w, h, r)
			builder.Fill()
			return nil
		},
	}
	col := color.PatternUncolored(pat, color.DeviceRGB(1, 0, 0))

	page := doc.AddPage()

	page.PushGraphicsState()
	page.SetFillColor(col)
	page.Rectangle(50, 300, 500, 500)
	page.FillAndStroke()
	page.PopGraphicsState()

	page.TextSetFont(F, 12)
	page.TextBegin()
	page.TextFirstLine(50, 230)
	page.TextShow("A square filled with an uncolored tiling pattern (color space ‘Pattern’).")
	page.TextEnd()

	err := page.Close()
	if err != nil {
		return err
	}

	return nil
}

func showTilingPatternColored(doc *document.MultiPage, F font.Layouter) error {
	// 1/2^2 + (x/2)^2 = 1^2   =>   x = 2*sqrt(3)/2 = sqrt(3)

	width := 12.0
	height := width * math.Sqrt(3)
	r := 0.3 * width

	pat := &pattern.Type1{
		TilingType: 1,
		BBox:       &pdf.Rectangle{URx: width, URy: height},
		XStep:      width,
		YStep:      height,
		Matrix:     matrix.Identity,
		Color:      true,
		Draw: func(w *graphics.Writer) error {
			w.SetFillColor(color.DeviceGray(0.5))
			w.Circle(0, 0, r)
			w.Circle(width, 0, r)
			w.Circle(0, height, r)
			w.Circle(width, height, r)
			w.Fill()
			w.SetFillColor(color.DeviceRGB(1, 0, 0))
			w.Circle(width/2, height/2, r)
			w.Fill()
			return nil
		},
	}
	col := color.PatternColored(pat)

	page := doc.AddPage()

	page.PushGraphicsState()
	page.SetFillColor(col)
	page.Rectangle(50, 300, 500, 500)
	page.FillAndStroke()
	page.PopGraphicsState()

	page.TextSetFont(F, 12)
	page.TextBegin()
	page.TextFirstLine(50, 230)
	page.TextShow("A square filled with a colored tiling pattern (color space ’Pattern’).")
	page.TextEnd()

	err := page.Close()
	if err != nil {
		return err
	}

	return nil
}

func showShadingPattern(doc *document.MultiPage, F font.Layouter) error {
	fn := &function.Type2{
		XMin: 0,
		XMax: 1,
		C0:   []float64{1, 0, 0},
		C1:   []float64{0, 1, 0},
		N:    1,
	}

	shadingData := &shading.Type3{
		ColorSpace:  color.DeviceRGBSpace,
		Center1:     vec.Vec2{X: 100, Y: 350},
		R1:          10,
		Center2:     vec.Vec2{X: 500, Y: 750},
		R2:          200,
		F:           fn,
		ExtendStart: true,
		ExtendEnd:   true,

		SingleUse: true,
	}

	dict := &pattern.Type2{
		Shading:   shadingData,
		SingleUse: true,
	}
	col := color.PatternColored(dict)

	page := doc.AddPage()

	page.PushGraphicsState()
	page.SetFillColor(col)
	page.Rectangle(50, 300, 500, 500)
	page.FillAndStroke()
	page.PopGraphicsState()

	page.TextSetFont(F, 12)
	page.TextBegin()
	page.TextFirstLine(50, 230)
	page.TextShow("A square filled with a shading pattern (color space ’Pattern’).")
	page.TextEnd()

	err := page.Close()
	if err != nil {
		return err
	}

	return nil
}

func showShading(doc *document.MultiPage, F font.Layouter) error {
	vertices := []shading.Type4Vertex{
		{X: 0.95, Y: 0.6, Flag: 0, Color: []float64{0}}, // 0
		{X: 2.7, Y: 0.8, Flag: 0, Color: []float64{0}},
		{X: 1.9, Y: 2.0, Flag: 0, Color: []float64{0.1}},
		{X: 3.5, Y: 1.8, Flag: 1, Color: []float64{0.1}},
		{X: 4.6, Y: 0.3, Flag: 2, Color: []float64{0}},

		{X: 3.5, Y: 1.8, Flag: 0, Color: []float64{0.1}}, // 5
		{X: 1.9, Y: 2.0, Flag: 0, Color: []float64{0.1}},
		{X: 3.2, Y: 2.8, Flag: 0, Color: []float64{0.15}},
		{X: 2.2, Y: 3.9, Flag: 1, Color: []float64{0.2}},
		{X: 3.3, Y: 4.9, Flag: 1, Color: []float64{0.15}},
		{X: 2.0, Y: 6.0, Flag: 1, Color: []float64{0.2}}, // 10
		{X: 3.3, Y: 6.25, Flag: 1, Color: []float64{0.15}},
		{X: 1.5, Y: 7.0, Flag: 1, Color: []float64{0.15}},
		{X: 3.1, Y: 7.2, Flag: 1, Color: []float64{0.1}},
		{X: 1.2, Y: 7.9, Flag: 1, Color: []float64{0.25}},
		{X: 3.25, Y: 7.3, Flag: 1, Color: []float64{0.1}}, // 15
		{X: 3.5, Y: 9.2, Flag: 1, Color: []float64{0.35}},
		{X: 4.9, Y: 7.8, Flag: 1, Color: []float64{0.35}},
		{X: 6.0, Y: 9.4, Flag: 1, Color: []float64{0.4}},
		{X: 6.4, Y: 7.6, Flag: 1, Color: []float64{0.35}},
		{X: 8.0, Y: 8.0, Flag: 1, Color: []float64{0.35}}, // 20
		{X: 7.2, Y: 7.0, Flag: 1, Color: []float64{0.25}},
		{X: 8.3, Y: 6.3, Flag: 1, Color: []float64{0.35}},
		{X: 7.2, Y: 5.5, Flag: 1, Color: []float64{0.35}},
		{X: 8.5, Y: 5.3, Flag: 1, Color: []float64{0.35}},
		{X: 7.8, Y: 4.5, Flag: 1, Color: []float64{0.2}}, // 25
		{X: 8.9, Y: 4.0, Flag: 1, Color: []float64{0.35}},
		{X: 8.1, Y: 3.5, Flag: 1, Color: []float64{0.25}},
		{X: 9.05, Y: 2.7, Flag: 1, Color: []float64{0.35}},
		{X: 7.8, Y: 2.7, Flag: 1, Color: []float64{0.05}},
		{X: 8.8, Y: 1.5, Flag: 1, Color: []float64{0.2}}, // 30
		{X: 7.95, Y: 1.15, Flag: 1, Color: []float64{0.15}},
		{X: 7.0, Y: 1.7, Flag: 2, Color: []float64{0.1}},
		{X: 6.6, Y: 2.4, Flag: 2, Color: []float64{0.1}},
		{X: 7.2, Y: 3.3, Flag: 2, Color: []float64{0.1}},
		{X: 6.4, Y: 3.2, Flag: 1, Color: []float64{0.05}}, // 35
		{X: 6.6, Y: 3.95, Flag: 1, Color: []float64{0.1}},
		{X: 6.3, Y: 3.7, Flag: 1, Color: []float64{0.0}},
		{X: 6.1, Y: 4.3, Flag: 1, Color: []float64{0.1}},
		{X: 5.8, Y: 4.1, Flag: 1, Color: []float64{0.0}},
		{X: 5.0, Y: 4.3, Flag: 1, Color: []float64{0.0}}, // 40
	}
	cs, err := color.CalGray(color.WhitePointD65, nil, 1)
	if err != nil {
		return err
	}
	shadingData := &shading.Type4{
		ColorSpace:        cs,
		BitsPerFlag:       2,
		BitsPerCoordinate: 8,
		BitsPerComponent:  4,
		Decode: []float64{
			0, 10, 0, 10, 0, 1,
		},
		Vertices: vertices,
	}

	page := doc.AddPage()

	page.PushGraphicsState()
	m := matrix.Scale(50, 50)
	m = m.Mul(matrix.Translate(50, 300))
	page.Transform(m)
	page.DrawShading(shadingData)
	page.PopGraphicsState()

	page.TextSetFont(F, 12)
	page.TextBegin()
	page.TextFirstLine(50, 230)
	page.TextShow("A Type 4 shading drawn using the sh operator.")
	page.TextEnd()

	err = page.Close()
	if err != nil {
		return err
	}

	return nil
}

var black = color.DeviceGray(0.0)

func hTickLabel(page *document.Page, F font.Layouter, x, y float64, label string) {
	page.SetStrokeColor(black)
	page.SetLineWidth(0.5)
	page.MoveTo(x, y+3)
	page.LineTo(x, y-3)
	page.Stroke()

	geom := F.GetGeometry()

	page.SetFillColor(black)
	page.TextSetFont(F, 10)
	gg := page.TextLayout(nil, label)
	w := gg.TotalWidth()
	page.TextBegin()
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
	gg := page.TextLayout(nil, label)
	w := gg.TotalWidth()
	page.TextBegin()
	page.TextFirstLine(x-5-w, y-10*mid)
	page.TextShowGlyphs(gg)
	page.TextEnd()
}
