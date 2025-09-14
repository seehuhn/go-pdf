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
	"io"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/graphics/text"
)

const size = 51

var dataFile io.WriteCloser

func main() {
	err := createDocument("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

var paper = &pdf.Rectangle{
	URx: 300,
	URy: 310,
}

func createDocument(filename string) error {
	fd, err := os.Create("data.csv")
	if err != nil {
		return err
	}
	defer fd.Close()

	dataFile = fd
	fmt.Fprintln(dataFile, "predictor,colors,depth,length")

	doc, err := document.CreateMultiPage(filename, paper, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	F5To4 := &function.Type4{
		Domain:  []float64{0, 1, 0, 1, 0, 1, 0, 1, 0, 1},
		Range:   []float64{0, 1, 0, 1, 0, 1, 0, 1},
		Program: "pop",
	}
	funny, err := color.DeviceN([]pdf.Name{"Cyan", "Magenta", "Yellow", "Black", "Gold"}, color.SpaceDeviceCMYK, F5To4, nil)
	if err != nil {
		return err
	}
	csMap := map[int]color.Space{
		1: color.SpaceDeviceGray,
		3: color.SpaceDeviceRGB,
		4: color.SpaceDeviceCMYK,
		5: funny,
	}

	g := &generator{
		doc:   doc,
		F:     standard.Helvetica.New(),
		csMap: csMap,
	}

	err = g.TitlePage()
	if err != nil {
		return err
	}

	for _, colors := range []int{1, 3, 4, 5} {
		for _, bpc := range []int{1, 2, 4, 8, 16} {
			for _, predictor := range []int{1, 2, 10, 11, 12, 13, 14, 15} {
				err = g.GenerateImage(predictor, colors, bpc)
				if err != nil {
					return err
				}
			}
		}
	}

	return doc.Close()
}

type generator struct {
	doc   *document.MultiPage
	F     font.Layouter
	csMap map[int]color.Space
}

func (g *generator) TitlePage() error {
	page := g.doc.AddPage()
	text.Show(page.Writer,
		text.F{Font: g.F, Size: 10},
		text.M{X: 40, Y: 250},
		"Image Compression Test",
		text.NL,
		text.NL,
		text.Wrap(190,
			"The images on the following pages use different numbers of color channels and bit depths.",
			"Each image is compressed using all eight predictors defined for the Flate filter for PDF streams.",
			"If everything works correctly, each group of eight consecutive images will look identical.",
		),
		text.NL,
		text.Wrap(190,
			"The text below the images shows the object reference of the image in the PDF document and the length of the compressed image data.",
		),
	)
	return page.Close()
}

func (g *generator) GenerateImage(predictor, colors, bpc int) error {
	page := g.doc.AddPage()

	page.TextBegin()
	page.TextSetFont(g.F, 10)
	page.TextFirstLine(50, 260)
	page.TextShow(fmt.Sprintf("Predictor: %d, Colors: %d, BitsPerComponent: %d", predictor, colors, bpc))
	page.TextEnd()

	img := &testImage{
		predictor: predictor,
		colors:    colors,
		bpc:       bpc,
		cs:        g.csMap[colors],
	}
	page.PushGraphicsState()
	page.Transform(matrix.Matrix{200, 0, 0, 200, 50, 50})
	page.DrawXObject(img)
	page.PopGraphicsState()

	ref, _, err := pdf.ResourceManagerEmbed(page.RM, img)
	if err != nil {
		return err
	}
	s, err := pdf.GetStream(page.RM.Out, ref)
	if err != nil {
		return err
	}

	page.TextBegin()
	page.TextSetFont(g.F, 10)
	page.TextFirstLine(50, 36)
	page.TextShow(fmt.Sprintf("%d 0 R, %d bytes",
		ref.(pdf.Reference), s.Dict["Length"]))
	page.TextEnd()

	if dataFile != nil {
		fmt.Fprintf(dataFile, "%d,%d,%d,%d\n",
			predictor, colors, bpc, s.Dict["Length"].(pdf.Integer))
	}

	return page.Close()
}

type testImage struct {
	predictor int
	colors    int
	bpc       int
	cs        color.Space
}

var _ graphics.XObject = (*testImage)(nil)

func (img *testImage) Subtype() pdf.Name {
	return "Image"
}

func (img *testImage) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	csEmbedded, _, err := pdf.ResourceManagerEmbed(rm, img.cs)
	if err != nil {
		return nil, zero, err
	}

	nChannels := img.cs.Channels()

	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(size),
		"Height":           pdf.Integer(size),
		"ColorSpace":       csEmbedded,
		"BitsPerComponent": pdf.Integer(img.bpc),
	}
	ref := rm.Out.Alloc()
	compress := pdf.FilterFlate{
		"Predictor": pdf.Integer(img.predictor),
	}
	if img.predictor > 1 {
		compress["Colors"] = pdf.Integer(nChannels)
		compress["BitsPerComponent"] = pdf.Integer(img.bpc)
		compress["Columns"] = pdf.Integer(size)
	}
	stm, err := rm.Out.OpenStream(ref, dict, compress)
	if err != nil {
		return nil, zero, err
	}

	q := float64(uint(1)<<img.bpc - 1)
	row := image.NewPixelRow(size*nChannels, img.bpc)
	for i := 0; i < size; i++ {
		row.Reset()
		for j := 0; j < size; j++ {
			for c := 0; c < nChannels; c++ {
				x := ripple(i, j, c)
				if nChannels <= 3 {
					x = 1 - x
				}
				row.AppendBits(uint16(math.Round(x * q)))
			}
		}
		_, err = stm.Write(row.Bytes())
		if err != nil {
			return nil, zero, err
		}
	}

	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

const damping float64 = 4
const freq float64 = 40
const chirpRate float64 = 2

func ripple(i, j, c int) float64 {
	x := (float64(i)+0.5)/size - 0.45
	y := (float64(j)+0.5)/size - 0.4
	r := math.Sqrt(x*x + y*y)

	amplitude := math.Exp(-damping * r)
	phase := (freq-float64(c))*r + chirpRate*r*r/2
	return amplitude * sqr(math.Cos(phase))
}

func sqr(x float64) float64 {
	return x * x
}
