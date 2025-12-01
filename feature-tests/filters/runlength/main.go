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

// This program creates a test PDF with a RunLengthDecode-encoded image.
// The image shows concentric circles in 9 rainbow colors with 25-pixel-wide
// bands, demonstrating the compression benefits of run-length encoding for
// patterns with repeated values.
package main

import (
	"math"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics/color"
)

const size = 301

type runlengthImage struct {
	data []byte
}

func (img *runlengthImage) Subtype() pdf.Name {
	return pdf.Name("Image")
}

func (img *runlengthImage) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	colors := []color.Color{
		color.DeviceRGB{1, 0, 0},     // red
		color.DeviceRGB{1, 0.5, 0},   // orange
		color.DeviceRGB{1, 1, 0},     // yellow
		color.DeviceRGB{0, 1, 0},     // green
		color.DeviceRGB{0, 1, 1},     // cyan
		color.DeviceRGB{0, 0, 1},     // blue
		color.DeviceRGB{0.3, 0, 0.5}, // indigo
		color.DeviceRGB{0.5, 0, 1},   // violet
		color.DeviceRGB{0, 0, 0},     // black
	}

	colorSpace, err := color.Indexed(colors)
	if err != nil {
		return nil, err
	}

	csEmbedded, err := rm.Embed(colorSpace)
	if err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Type":             pdf.Name("XObject"),
		"Subtype":          pdf.Name("Image"),
		"Width":            pdf.Integer(size),
		"Height":           pdf.Integer(size),
		"ColorSpace":       csEmbedded,
		"BitsPerComponent": pdf.Integer(8),
	}

	ref := rm.Alloc()
	w, err := rm.Out().OpenStream(ref, dict, pdf.FilterRunLength{})
	if err != nil {
		return nil, err
	}

	_, err = w.Write(img.data)
	if err != nil {
		w.Close()
		return nil, err
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
}

func (img *runlengthImage) Bounds() rect.IntRect {
	return rect.IntRect{
		XMin: 0,
		YMin: 0,
		XMax: size,
		YMax: size,
	}
}

func generateImageData() []byte {
	const center = 150.5
	const bandWidth = 25.0

	data := make([]byte, size*size)

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - center
			dy := float64(y) + 0.5 - center
			dist := math.Sqrt(dx*dx + dy*dy)

			band := int(dist / bandWidth)
			colorIndex := byte(band % 9)

			data[y*size+x] = colorIndex
		}
	}

	return data
}

func main() {
	img := &runlengthImage{
		data: generateImageData(),
	}

	pageSize := &pdf.Rectangle{
		LLx: 0,
		LLy: 0,
		URx: size,
		URy: size,
	}

	page, err := document.CreateSinglePage("test.pdf", pageSize, pdf.V1_7, nil)
	if err != nil {
		panic(err)
	}

	page.Transform(matrix.Matrix{size, 0, 0, size, 0, 0})
	page.DrawXObject(img)

	err = page.Close()
	if err != nil {
		panic(err)
	}
}
