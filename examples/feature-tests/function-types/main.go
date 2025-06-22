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

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
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
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	doc, err := document.CreateMultiPage(fname, paper, pdf.V2_0, opt)
	if err != nil {
		return err
	}

	writer := newWriter(doc)

	F := &function.Type0{
		Domain:        []float64{0, 1, 0, 1},
		Range:         []float64{0, 1, 0, 1, 0, 1},
		Size:          []int{2, 1},
		BitsPerSample: 8,
		Encode:        []float64{0, 1, 0, 1},
		Decode:        []float64{0, 1, 0, 1, 0, 1},
		Samples:       []byte{0, 255, 255, 0, 10, 100},
	}
	writer.testStrip(F)

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
}

func newWriter(doc *document.MultiPage) *writer {
	w := &writer{
		doc:  doc,
		yPos: paper.URy - margin,
	}

	return w
}

func (w *writer) Close() error {
	if w.page != nil {
		return w.page.Close()
	}
	return nil
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

func (w *writer) testStrip(f pdf.Function) error {
	err := w.ensureSpace(2*stripHeight + stripGap)
	if err != nil {
		return err
	}

	area1 := rect.Rect{
		LLx: margin,
		LLy: w.yPos - stripHeight,
		URx: paper.URx - margin,
		URy: w.yPos,
	}
	w.yPos -= stripHeight + stripGap
	area2 := rect.Rect{
		LLx: margin,
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

	s := &shading.Type1{
		ColorSpace: cs,
		F:          f,
		Matrix:     []float64{area1.Dx(), 0, 0, area1.Dy(), area1.LLx, area1.LLy},
		BBox:       &pdf.Rectangle{LLx: area1.LLx, LLy: area1.LLy, URx: area1.URx, URy: area1.URy},
	}
	w.page.DrawShading(s)

	// method 2: Render the function to an image and draw the image.

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
	for i := range img.height {
		y := float64(i) / float64(img.height-1)
		for j := range img.width {
			x := float64(j) / float64(img.width-1)

			res := img.f.Apply(x, y)
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
