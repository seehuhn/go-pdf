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
	"strings"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/shading"
)

const (
	margin = 40.0 // margin in points

	stripHeight = 16.0
	stripGap    = 4.0
)

var paper = document.A4

func main() {
	fmt.Println("writing test.pdf ...")
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(fname string) error {
	doc, err := document.CreateMultiPage(fname, paper, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	writer := newWriter(doc)

	// Add introduction page
	writer.writeIntroduction()
	writer.newPage()

	samples := [][]float64{
		{0.0, 1.0, 1.0},
		{0.0, 0.0, 0.0},
		{1.0, 0.0, 0.0},
		{1.0, 1.0, 1.0},
		{0.0, 0.04, 0.39},

		{0.0, 1.0, 1.0},
		{0.0, 0.0, 0.0},
		{1.0, 0.0, 0.0},
		{1.0, 1.0, 1.0},
		{0.0, 0.04, 0.39},

		{0.0, 1.0, 1.0},
		{0.0, 0.0, 0.0},
		{1.0, 0.0, 0.0},
		{1.0, 1.0, 1.0},
		{0.0, 0.04, 0.39},

		{0.2, 0.2, 0.2},
		{0.2, 0.2, 0.2},
		{0.2, 0.2, 0.2},
		{0.2, 0.2, 0.2},
		{0.2, 0.2, 0.2},
	}

	for _, method := range []string{"linear", "cubic"} {
		validBitDepths := []int{1, 2, 4, 8, 12, 16, 24, 32}
		for _, bitDepth := range validBitDepths {
			writer.printf("Type 0 functions, %s interpolation, %d-bit samples",
				method, bitDepth)

			F := &function.Type0{
				Domain:        []float64{0, 1, 0, 1},
				Range:         []float64{0, 1, 0, 1, 0, 1},
				Size:          []int{5, 4},
				BitsPerSample: bitDepth,
				UseCubic:      method == "cubic",
				Encode:        []float64{0, 4, 0, 3},
				Decode:        []float64{0, 1, 0, 1, 0, 1},
				Samples:       encodeSamples(bitDepth, samples),
			}
			writer.test2DStrip(F)
		}
	}

	writer.printf("Type 2 functions")
	F2 := &function.Type2{
		Domain: []float64{0, 1},
		Range:  []float64{0, 1, 0, 1, 0, 1},
		C0:     []float64{1, 0.2, 0},
		C1:     []float64{0.1, 0.9, 1},
		N:      0.8,
	}
	writer.test1DStrip(F2)

	writer.printf("Type 3 functions")
	F3 := &function.Type3{
		Domain: []float64{0, 1},
		Range:  []float64{0, 1, 0, 1, 0, 1},
		Functions: []pdf.Function{
			&function.Type2{
				Domain: []float64{0, 1},
				Range:  []float64{0, 1, 0, 1, 0, 1},
				C0:     []float64{1, 0, 0},
				C1:     []float64{1, 1, 0},
				N:      1.0,
			},
			&function.Type4{
				Domain:  []float64{0, 1},
				Range:   []float64{0, 1, 0, 1, 0, 1},
				Program: `pop 0 0 0`,
			},
			&function.Type2{
				Domain: []float64{0, 1},
				Range:  []float64{0, 1, 0, 1, 0, 1},
				C0:     []float64{1, 1, 0},
				C1:     []float64{0, 0, 1},
				N:      2.0,
			},
		},
		Bounds: []float64{0.5, 0.52},
		Encode: []float64{0, 1, 0, 1, 0, 1},
	}
	writer.test1DStrip(F3)

	writer.printf("Type 4 functions")
	F4 := &function.Type4{
		Domain: []float64{0, 1, 0, 1},
		Range:  []float64{0, 1, 0, 1, 0, 1},
		Program: `
			0.3 sub dup mul 5 mul
			exch 0.7 sub dup mul 10 mul
			add neg
			2.71828 exch exp
			dup dup sqrt exch dup mul
		`,
	}
	writer.test2DStrip(F4)

	err = writer.Close()
	if err != nil {
		return err
	}

	return doc.Close()
}

type writer struct {
	doc  *document.MultiPage
	page *document.Page
	yPos float64

	label font.Layouter
	body  font.Layouter
}

func newWriter(doc *document.MultiPage) *writer {
	w := &writer{
		doc:   doc,
		yPos:  paper.URy - margin,
		label: standard.Helvetica.New(),
		body:  standard.TimesRoman.New(),
	}

	return w
}

func (w *writer) Close() error {
	if w.page != nil {
		return w.page.Close()
	}
	return nil
}

func (w *writer) printf(format string, args ...interface{}) {
	text := fmt.Sprintf(format, args...)
	lines := strings.Split(text, "\n")

	w.ensureSpace(15)
	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.label, 12)
	for i, line := range lines {
		w.yPos -= 10
		switch i {
		case 0:
			w.page.TextFirstLine(margin, w.yPos)
		case 1:
			w.page.TextSecondLine(0, -15)
		default:
			w.page.TextNextLine()
		}
		w.page.TextShow(line)
		w.yPos -= 5
	}
	w.page.TextEnd()
	w.page.PopGraphicsState()
}

func (w *writer) ensureSpace(v float64) error {
	if w.page == nil || w.yPos-v < margin {
		if w.page != nil {
			err := w.page.Close()
			if err != nil {
				return err
			}
		}
		w.page = w.doc.AddPage()
		w.yPos = paper.URy - margin
	}
	return nil
}

func (w *writer) newPage() error {
	if w.page != nil {
		err := w.page.Close()
		if err != nil {
			return err
		}
	}
	w.page = w.doc.AddPage()
	w.yPos = paper.URy - margin
	return nil
}

func (w *writer) writeIntroduction() {
	w.ensureSpace(200) // Make sure we have enough space

	w.yPos -= 60

	// Title
	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.label, 16)
	w.page.TextFirstLine(margin, w.yPos)
	w.page.TextShow("PDF Function Types Visual Test")
	w.page.TextEnd()
	w.page.PopGraphicsState()
	w.yPos -= 30

	// Introduction paragraphs
	textWidth := paper.URx - 2*margin

	w.typeParagraph(textWidth,
		"This document serves as a visual test for the PDF function types implemented in the go-pdf library. "+
			"Each function type is demonstrated by rendering identical data using two different methods side by side. "+
			"The test passes if both strips in each pair look identical, allowing for minor rasterization differences "+
			"in the lower strip which is rendered as a bitmap image.")

	w.yPos -= 8

	w.typeParagraph(textWidth,
		"The upper strip in each pair uses PDF's native shading operators, embedding "+
			"the function within the PDF file and executing it within the PDF viewer. "+
			"The lower strip uses software rasterization, where the function is "+
			"evaluated evaluated within the library at many sample points and rendered as an image. "+
			"If both strips look the same, the PDF viewer and the library interpret a function in the same way.")

	w.yPos -= 8

	w.typeParagraph(textWidth,
		"If the strips in a pair do not look identical, this indicates potential issues such as incorrect "+
			"sample data ordering in Type 0 functions, mathematical errors in interpolation algorithms, "+
			"problems with coordinate space transformations, or bugs in the PDF function embedding process. "+
			"Color differences might suggest issues with color space handling or bit depth encoding.")

	w.yPos -= 8

	w.typeParagraph(textWidth,
		"The test covers all function types supported by PDF: Type 0 functions for sample interpolation with all supported bit depths, "+
			"Type 2 functions for power interpolation, Type 3 functions for piecewise "+
			"defined functions, and Type 4 functions using PostScript calculator expressions.")
}

func (w *writer) typeParagraph(width float64, content string) {
	// Simple paragraph rendering with basic word wrapping
	words := strings.Fields(content)

	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.body, 11)
	w.page.TextFirstLine(margin, w.yPos)

	var currentLine []string
	estimatedWidth := 0.0
	avgCharWidth := 6.0 // Rough estimation for Times Roman at 11pt

	for _, word := range words {
		testWidth := estimatedWidth
		if len(currentLine) > 0 {
			testWidth += avgCharWidth // space
		}
		testWidth += float64(len(word)) * avgCharWidth

		if testWidth > width && len(currentLine) > 0 {
			// Output current line and start new one
			w.page.TextShow(strings.Join(currentLine, " "))
			w.page.TextSecondLine(0, -13)
			w.yPos -= 13
			currentLine = []string{word}
			estimatedWidth = float64(len(word)) * avgCharWidth
		} else {
			currentLine = append(currentLine, word)
			estimatedWidth = testWidth
		}
	}

	// Output remaining text
	if len(currentLine) > 0 {
		w.page.TextShow(strings.Join(currentLine, " "))
		w.yPos -= 13
	}

	w.page.TextEnd()
	w.page.PopGraphicsState()

	// Add some spacing after paragraph
	w.yPos -= 6
}

func (w *writer) vSpace(v float64) {
	if w.page == nil {
		return
	}
	w.yPos = max(w.yPos-v, margin)
}

func (w *writer) test2DStrip(f pdf.Function) error {
	err := w.ensureSpace(2*stripHeight + 2*stripGap)
	if err != nil {
		return err
	}

	area1 := rect.Rect{
		LLx: margin + 30,
		LLy: w.yPos - stripHeight,
		URx: paper.URx - margin,
		URy: w.yPos,
	}
	w.yPos -= stripHeight + stripGap
	area2 := rect.Rect{
		LLx: margin + 30,
		LLy: w.yPos - stripHeight,
		URx: paper.URx - margin,
		URy: w.yPos,
	}
	w.yPos -= stripHeight + stripGap

	// method 1: Use the "sh" operator to draw a function-based shading.

	m, n := f.Shape()
	var cs color.Space
	switch n {
	case 1:
		cs = color.DeviceGraySpace
	case 3:
		cs = color.DeviceRGBSpace
	case 4:
		cs = color.DeviceCMYKSpace
	default:
		return fmt.Errorf("unsupported function shape: %d->%d", m, n)
	}

	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.label, 8)
	w.page.TextFirstLine(margin, area1.LLy+5)
	w.page.TextShow("function")
	w.page.TextEnd()

	s := &shading.Type1{
		ColorSpace: cs,
		F:          f,
		Matrix:     []float64{area1.Dx(), 0, 0, area1.Dy(), area1.LLx, area1.LLy},
		BBox:       &pdf.Rectangle{LLx: area1.LLx, LLy: area1.LLy, URx: area1.URx, URy: area1.URy},
	}
	w.page.DrawShading(s)

	// method 2: Render the function to an image and draw the image.

	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.label, 8)
	w.page.TextFirstLine(margin, area2.LLy+5)
	w.page.TextShow("image")
	w.page.TextEnd()

	img := &imageStrip{
		width:  int(math.Round(area2.Dx())),
		height: int(math.Round(area2.Dy())),
		n:      n,
		f:      f,
		cs:     cs,
	}
	w.page.PushGraphicsState()
	w.page.Transform(matrix.Matrix{
		area2.Dx(), 0, 0, area2.Dy(), area2.LLx, area2.LLy,
	})
	w.page.DrawXObject(img)
	w.page.PopGraphicsState()

	w.vSpace(10)

	return nil
}

func (w *writer) test1DStrip(f pdf.Function) error {
	err := w.ensureSpace(2*stripHeight + stripGap)
	if err != nil {
		return err
	}

	area1 := rect.Rect{
		LLx: margin + 30,
		LLy: w.yPos - stripHeight,
		URx: paper.URx - margin,
		URy: w.yPos,
	}
	w.yPos -= stripHeight + stripGap
	area2 := rect.Rect{
		LLx: margin + 30,
		LLy: w.yPos - stripHeight,
		URx: paper.URx - margin,
		URy: w.yPos,
	}
	w.yPos -= stripHeight + stripGap

	// method 1: Use the "sh" operator to draw an axial shading.

	_, n := f.Shape()
	var cs color.Space
	switch n {
	case 1:
		cs = color.DeviceGraySpace
	case 3:
		cs = color.DeviceRGBSpace
	case 4:
		cs = color.DeviceCMYKSpace
	default:
		return fmt.Errorf("unsupported function shape: 1->%d", n)
	}

	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.label, 8)
	w.page.TextFirstLine(margin, area1.LLy+5)
	w.page.TextShow("function")
	w.page.TextEnd()

	s := &shading.Type2{
		ColorSpace: cs,
		F:          f,
		X0:         area1.LLx,
		Y0:         area1.LLy + area1.Dy()/2,
		X1:         area1.URx,
		Y1:         area1.LLy + area1.Dy()/2,
		TMin:       0,
		TMax:       1,
		BBox:       &pdf.Rectangle{LLx: area1.LLx, LLy: area1.LLy, URx: area1.URx, URy: area1.URy},
	}
	w.page.DrawShading(s)

	// method 2: Render the function to an image and draw the image.

	w.page.PushGraphicsState()
	w.page.TextBegin()
	w.page.TextSetFont(w.label, 8)
	w.page.TextFirstLine(margin, area2.LLy+5)
	w.page.TextShow("image")
	w.page.TextEnd()

	img := &axialImageStrip{
		width:  int(math.Round(area2.Dx())),
		height: int(math.Round(area2.Dy())),
		n:      n,
		f:      f,
		cs:     cs,
	}
	w.page.PushGraphicsState()
	w.page.Transform(matrix.Matrix{
		area2.Dx(), 0, 0, area2.Dy(), area2.LLx, area2.LLy,
	})
	w.page.DrawXObject(img)
	w.page.PopGraphicsState()

	w.vSpace(10)

	return nil
}

type imageStrip struct {
	width  int
	height int
	n      int
	f      pdf.Function

	cs color.Space
}

func (img *imageStrip) Subtype() pdf.Name {
	return "Image"
}

func (img *imageStrip) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	csEmbedded, _, err := pdf.ResourceManagerEmbed(rm, img.cs)
	if err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(img.width),
		"Height":           pdf.Integer(img.height),
		"ColorSpace":       csEmbedded,
		"BitsPerComponent": pdf.Integer(8),
	}
	buf := make([]byte, 0, img.width*img.height*img.n)
	m, _ := img.f.Shape()

	for i := range img.height {
		y := 1 - float64(i)/float64(img.height-1)
		for j := range img.width {
			x := float64(j) / float64(img.width-1)

			var res []float64
			if m == 1 {
				res = img.f.Apply(x)
			} else {
				res = img.f.Apply(x, y)
			}
			for k := range img.n {
				b := byte(math.Round(res[k] * 255))
				buf = append(buf, b)
			}
		}
	}

	ref := rm.Out.Alloc()
	stm, err := rm.Out.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, zero, err
	}
	_, err = stm.Write(buf)
	if err != nil {
		return nil, zero, err
	}
	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

type axialImageStrip struct {
	width  int
	height int
	n      int
	f      pdf.Function

	cs color.Space
}

func (img *axialImageStrip) Subtype() pdf.Name {
	return "Image"
}

func (img *axialImageStrip) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	csEmbedded, _, err := pdf.ResourceManagerEmbed(rm, img.cs)
	if err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(img.width),
		"Height":           pdf.Integer(img.height),
		"ColorSpace":       csEmbedded,
		"BitsPerComponent": pdf.Integer(8),
	}
	buf := make([]byte, 0, img.width*img.height*img.n)

	for range img.height {
		for j := range img.width {
			t := float64(j) / float64(img.width-1)

			res := img.f.Apply(t)
			for k := range img.n {
				b := byte(math.Round(res[k] * 255))
				buf = append(buf, b)
			}
		}
	}

	ref := rm.Out.Alloc()
	stm, err := rm.Out.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, zero, err
	}
	_, err = stm.Write(buf)
	if err != nil {
		return nil, zero, err
	}
	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

func encodeSamples(bitDepth int, samples [][]float64) []byte {
	maxVal := (1 << bitDepth) - 1
	bitBuf := &bitBuffer{}

	for _, sample := range samples {
		for _, val := range sample {
			quantized := uint32(math.Round(val * float64(maxVal)))
			bitBuf.appendBits(quantized, bitDepth)
		}
	}

	return bitBuf.bytes()
}

type bitBuffer struct {
	buf   []byte
	bits  uint32
	nBits int
}

func (bb *bitBuffer) appendBits(value uint32, bitDepth int) {
	bb.bits = (bb.bits << bitDepth) | (value & ((1 << bitDepth) - 1))
	bb.nBits += bitDepth

	for bb.nBits >= 8 {
		bb.buf = append(bb.buf, byte(bb.bits>>(bb.nBits-8)))
		bb.nBits -= 8
		bb.bits &= (1 << bb.nBits) - 1
	}
}

func (bb *bitBuffer) bytes() []byte {
	if bb.nBits > 0 {
		bb.buf = append(bb.buf, byte(bb.bits<<(8-bb.nBits)))
	}
	return bb.buf
}
