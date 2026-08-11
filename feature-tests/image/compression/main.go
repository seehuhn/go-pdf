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

// size is the width and height of every test image, in pixels.
const size = 51

var paper = &pdf.Rectangle{
	URx: 300,
	URy: 310,
}

func main() {
	err := run("test.pdf", "data.csv")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// run writes the test document, and a table of the compressed length of every
// image it contains.
func run(pdfName, csvName string) error {
	csv, err := os.Create(csvName)
	if err != nil {
		return err
	}
	defer csv.Close()

	if _, err := fmt.Fprintln(csv, "predictor,colors,depth,length"); err != nil {
		return err
	}

	doc, err := document.CreateMultiPage(pdfName, paper, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	// a five-component space, to reach beyond the four of DeviceCMYK; the
	// tint transform drops the extra component
	dropFifth := &function.Type4{
		Domain:  []float64{0, 1, 0, 1, 0, 1, 0, 1, 0, 1},
		Range:   []float64{0, 1, 0, 1, 0, 1, 0, 1},
		Program: "pop",
	}
	gold, err := color.DeviceN(
		[]pdf.Name{"Cyan", "Magenta", "Yellow", "Black", "Gold"},
		color.SpaceDeviceCMYK, dropFifth, nil)
	if err != nil {
		return err
	}

	g := &generator{
		doc:  doc,
		font: font.Must(standard.Helvetica.New()),
		spaces: map[int]color.Space{
			1: color.SpaceDeviceGray,
			3: color.SpaceDeviceRGB,
			4: color.SpaceDeviceCMYK,
			5: gold,
		},
		data: csv,
	}

	if err := g.titlePage(); err != nil {
		return err
	}
	for _, colors := range []int{1, 3, 4, 5} {
		for _, bpc := range []int{1, 2, 4, 8, 16} {
			for _, predictor := range []int{1, 2, 10, 11, 12, 13, 14, 15} {
				if err := g.imagePage(predictor, colors, bpc); err != nil {
					return err
				}
			}
		}
	}

	if err := doc.Close(); err != nil {
		return err
	}

	// the deferred close covers the error paths above; this one reports a
	// failure to flush the table, which would leave the figures incomplete
	return csv.Close()
}

type generator struct {
	doc    *document.MultiPage
	font   font.Layouter
	spaces map[int]color.Space
	data   io.Writer
}

func (g *generator) titlePage() error {
	page := g.doc.AddPage()
	text.Show(page.Builder,
		text.F{Font: g.font, Size: 10},
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
		text.NL,
		text.Wrap(190,
			"Predictor 2 at bit depths other than 1 and 8 is rare in practice, and some viewers have bugs in this area.",
		),
	)
	return page.Close()
}

// imagePage adds one page showing the test image under a single combination of
// predictor, channel count and bit depth, and records its compressed length.
func (g *generator) imagePage(predictor, colors, bpc int) error {
	page := g.doc.AddPage()

	img := &testImage{
		predictor: predictor,
		bpc:       bpc,
		cs:        g.spaces[colors],
	}

	// embed before drawing, so that the object reference and the compressed
	// length can be shown on the page itself
	obj, err := page.RM.Embed(img)
	if err != nil {
		return err
	}
	ref, ok := obj.(pdf.Reference)
	if !ok {
		return fmt.Errorf("image embedded as %T, want a reference", obj)
	}

	c := pdf.NewCursor(page.RM.Out)
	stm, err := c.Stream(ref)
	if err != nil {
		return err
	}
	length := stm.Length()

	page.PushGraphicsState()
	page.Transform(matrix.Matrix{200, 0, 0, 200, 50, 50})
	page.DrawXObject(img)
	page.PopGraphicsState()

	text.Show(page.Builder,
		text.F{Font: g.font, Size: 10},
		text.M{X: 50, Y: 260},
		fmt.Sprintf("Predictor: %d, Colors: %d, BitsPerComponent: %d", predictor, colors, bpc),
	)
	text.Show(page.Builder,
		text.F{Font: g.font, Size: 10},
		text.M{X: 50, Y: 36},
		fmt.Sprintf("%d %d R, %d bytes", ref.Number(), ref.Generation(), length),
	)

	if _, err := fmt.Fprintf(g.data, "%d,%d,%d,%d\n", predictor, colors, bpc, length); err != nil {
		return err
	}

	return page.Close()
}

// testImage draws the same ripple pattern whatever the colour space, bit depth
// and predictor, so that the pages can be compared against one another.
type testImage struct {
	predictor int
	bpc       int
	cs        color.Space
}

var _ graphics.XObject = (*testImage)(nil)

func (img *testImage) Subtype() pdf.Name {
	return "Image"
}

func (img *testImage) ResourceName() pdf.Name {
	return ""
}

func (img *testImage) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	csEmbedded, err := rm.Embed(img.cs)
	if err != nil {
		return nil, err
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

	compress := pdf.FilterFlate{
		Predictor: pdf.FlatePredictor(img.predictor),
	}
	if img.predictor > 1 {
		compress.Colors = nChannels
		compress.BitsPerComponent = img.bpc
		compress.Columns = size
	}

	ref := rm.Alloc()
	stm, err := rm.Out().OpenStream(ref, dict, compress)
	if err != nil {
		return nil, err
	}

	maxValue := float64(uint(1)<<img.bpc - 1)
	pixels := image.NewPixelRow(size*nChannels, img.bpc)
	for row := range size {
		pixels.Reset()
		for col := range size {
			for ch := range nChannels {
				v := ripple(row, col, ch)
				// DeviceGray and DeviceRGB are additive, the other two
				// subtractive; inverting keeps all four looking alike
				if nChannels <= 3 {
					v = 1 - v
				}
				pixels.AppendBits(uint16(math.Round(v * maxValue)))
			}
		}
		if _, err := stm.Write(pixels.Bytes()); err != nil {
			return nil, err
		}
	}

	if err := stm.Close(); err != nil {
		return nil, err
	}

	return ref, nil
}

const (
	damping   = 4  // how fast the ripple fades away from its centre
	freq      = 40 // rings per unit distance at the centre
	chirpRate = 2  // how fast the rings tighten further out
)

// ripple is the test pattern: concentric rings which get finer towards the
// edge, so that a predictor decoded wrongly is easy to see.  The centre sits
// slightly off the middle of the image, and each channel is given its own
// frequency, so that no row or column repeats another.
func ripple(row, col, channel int) float64 {
	u := (float64(row)+0.5)/size - 0.45
	v := (float64(col)+0.5)/size - 0.4
	r := math.Sqrt(u*u + v*v)

	amplitude := math.Exp(-damping * r)
	phase := (freq-float64(channel))*r + chirpRate*r*r/2
	return amplitude * sqr(math.Cos(phase))
}

func sqr(x float64) float64 {
	return x * x
}
