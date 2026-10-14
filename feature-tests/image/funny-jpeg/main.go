// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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
	gocol "image/color"
	"io"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	pdfimg "seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/graphics/text"
)

func main() {
	err := run("test.pdf")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("wrote test.pdf")
}

// stripe colors for the RGB row
var rgbStripes = []gocol.NRGBA{
	{R: 195, G: 75, B: 75, A: 255},  // red
	{R: 200, G: 155, B: 70, A: 255}, // gold
	{R: 80, G: 160, B: 75, A: 255},  // green
	{R: 55, G: 108, B: 208, A: 255}, // blue
}

// stripe colors for the CMYK row (C, M, Y, K)
var cmykStripes = [][4]byte{
	{0, 190, 200, 15}, // red
	{0, 15, 225, 5},   // yellow
	{200, 0, 200, 50}, // green
	{230, 130, 0, 10}, // blue
}

func run(filename string) error {
	const w, h = 64, 64

	rgbPixels := createRGBPixels(w, h)
	cmykPixels := createCMYKPixels(w, h)

	page, err := document.CreateSinglePage(filename, document.A4, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	helv := font.Must(standard.Helvetica.New())
	F := text.F{Font: helv, Size: 14}
	Fsmall := text.F{Font: helv, Size: 10}

	// title
	text.Show(page.Builder,
		text.M{X: 72, Y: 790},
		F, "DCTDecode ColorTransform Test",
	)

	const (
		imgSize = 170.0 // drawn size of each image
		colL    = 110.0 // left column x
		colR    = 320.0 // right column x
		rowT    = 570.0 // top row image bottom y
		rowB    = 360.0 // bottom row image bottom y
		labelX  = 90.0  // x position of rotated row labels
	)

	// column labels (centered above each image)
	text.Show(page.Builder,
		text.M{X: colL + 45, Y: rowT + imgSize + 15},
		Fsmall, "Colour transform on",
	)
	text.Show(page.Builder,
		text.M{X: colR + 40, Y: rowT + imgSize + 15},
		Fsmall, "Colour transform off",
	)

	// row labels (rotated 90° CCW, to the left of images)
	drawRotatedLabel(page, Fsmall, "DeviceRGB", labelX, rowT+imgSize/2, 50)
	drawRotatedLabel(page, Fsmall, "DeviceCMYK", labelX, rowB+imgSize/2, 56)

	// each cell stores the same samples, with the JPEG colour transform
	// switched on (left) or off (right)
	cells := []struct {
		cs        color.Space
		pixels    []byte
		transform bool
		x, y      float64
	}{
		{color.SpaceDeviceRGB, rgbPixels, true, colL, rowT},
		{color.SpaceDeviceRGB, rgbPixels, false, colR, rowT},
		{color.SpaceDeviceCMYK, cmykPixels, true, colL, rowB},
		{color.SpaceDeviceCMYK, cmykPixels, false, colR, rowB},
	}
	for _, c := range cells {
		src := pdfimg.NewDCTSource(w, h, c.cs, writeBytes(c.pixels))
		src.ColorTransform = c.transform
		src.Quality = 100
		img := &pdfimg.Dict{
			Width:            w,
			Height:           h,
			ColorSpace:       c.cs,
			BitsPerComponent: 8,
			Data:             src,
		}
		drawImage(page, img, c.x, c.y, imgSize)
	}

	// footnote
	text.Show(page.Builder,
		text.M{X: 72, Y: rowB - 30},
		Fsmall, "Within each row, both images should look the same: the transform only changes how the data is stored.",
	)

	return page.Close()
}

func drawRotatedLabel(page *document.Page, f text.F, label string, x, y, textWidth float64) {
	page.PushGraphicsState()
	page.Transform(matrix.Translate(x, y))
	page.Transform(matrix.Rotate(math.Pi / 2))
	text.Show(page.Builder, text.M{X: -textWidth / 2, Y: 0}, f, label)
	page.PopGraphicsState()
}

func drawImage(page *document.Page, img graphics.XObject, x, y, size float64) {
	page.PushGraphicsState()
	page.Transform(matrix.Translate(x, y))
	page.Transform(matrix.Scale(size, size))
	page.DrawXObject(img)
	page.PopGraphicsState()
}

func createRGBPixels(w, h int) []byte {
	pixels := make([]byte, 0, w*h*3)
	stripeH := h / len(rgbStripes)
	for y := range h {
		c := rgbStripes[min(y/stripeH, len(rgbStripes)-1)]
		for range w {
			pixels = append(pixels, c.R, c.G, c.B)
		}
	}
	return pixels
}

func writeBytes(data []byte) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	}
}

func createCMYKPixels(w, h int) []byte {
	pixels := make([]byte, w*h*4)
	stripeH := h / len(cmykStripes)
	for y := range h {
		idx := min(y/stripeH, len(cmykStripes)-1)
		for x := range w {
			copy(pixels[(y*w+x)*4:], cmykStripes[idx][:])
		}
	}
	return pixels
}
