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

package image

import (
	"bytes"
	"errors"
	"image"
	gocolor "image/color"
	"image/draw"

	"seehuhn.de/go/pdf/graphics/color"
)

// compile-time interface checks
var (
	_ image.Image = (*Data)(nil)
	_ draw.Image  = (*Data)(nil)
)

// Data holds decoded image pixel data in a PDF color space.
// It implements [draw.Image], allowing pixel-level read/write access
// using native PDF color objects.
type Data struct {
	pix    []float64
	cs     color.Space
	width  int
	height int
	ncomp  int
}

// NewData creates a new Data image with the given color space and dimensions.
// All pixels are initialized to the default color of the color space.
func NewData(cs color.Space, width, height int) *Data {
	ncomp := cs.Channels()
	pix := make([]float64, width*height*ncomp)

	// fill with default color values
	defVals, _ := color.Values(cs.Default())
	if len(defVals) == ncomp {
		for i := 0; i < width*height; i++ {
			copy(pix[i*ncomp:], defVals)
		}
	}

	return &Data{
		pix:    pix,
		cs:     cs,
		width:  width,
		height: height,
		ncomp:  ncomp,
	}
}

// ColorModel returns the color space as a [gocolor.Model].
func (d *Data) ColorModel() gocolor.Model {
	return d.cs
}

// Bounds returns the image bounds.
func (d *Data) Bounds() image.Rectangle {
	return image.Rect(0, 0, d.width, d.height)
}

// At returns the color at position (x, y).
// If (x, y) is out of bounds, the default color is returned.
func (d *Data) At(x, y int) gocolor.Color {
	if x < 0 || x >= d.width || y < 0 || y >= d.height {
		return d.cs.Default()
	}
	offset := (y*d.width + x) * d.ncomp
	return color.FromValues(d.cs, d.pix[offset:offset+d.ncomp], nil)
}

// Set sets the color at position (x, y).
// If (x, y) is out of bounds, the call is a no-op.
func (d *Data) Set(x, y int, c gocolor.Color) {
	if x < 0 || x >= d.width || y < 0 || y >= d.height {
		return
	}
	converted := d.cs.Convert(c)
	vals, _ := color.Values(converted.(color.Color))
	offset := (y*d.width + x) * d.ncomp
	copy(d.pix[offset:], vals[:d.ncomp])
}

// Load decodes the pixel data from a Dict into a Data image.
func (d *Dict) Load() (*Data, error) {
	cs := d.ColorSpace
	if cs == nil {
		return nil, errors.New("missing color space")
	}
	ncomp := cs.Channels()
	if ncomp == 0 {
		return nil, errors.New("pattern color spaces are not valid for images")
	}

	decode := d.Decode
	if decode == nil {
		decode = DefaultDecode(cs, d.BitsPerComponent)
	}

	var buf bytes.Buffer
	if err := d.WriteData(&buf); err != nil {
		return nil, err
	}
	data := buf.Bytes()

	width, height := d.Width, d.Height
	bpc := d.BitsPerComponent
	pix := make([]float64, width*height*ncomp)
	vals := make([]float64, ncomp)

	for y := range height {
		for x := range width {
			readSamples(data, width, ncomp, bpc, x, y, decode, vals)
			offset := (y*width + x) * ncomp
			copy(pix[offset:], vals)
		}
	}

	return &Data{
		pix:    pix,
		cs:     cs,
		width:  width,
		height: height,
		ncomp:  ncomp,
	}, nil
}

// readSamples reads n color component values for pixel (x, y) from raw image
// data, applying the Decode mapping to produce color space values.
func readSamples(data []byte, width, n, bpc, x, y int, decode, values []float64) {
	switch bpc {
	case 8:
		rowStart := y * width * n
		pixStart := rowStart + x*n
		for c := range n {
			idx := pixStart + c
			if idx < len(data) {
				s := float64(data[idx]) / 255
				values[c] = decode[2*c] + s*(decode[2*c+1]-decode[2*c])
			} else {
				values[c] = decode[2*c]
			}
		}
	case 16:
		rowStart := y * width * n * 2
		pixStart := rowStart + x*n*2
		for c := range n {
			idx := pixStart + c*2
			if idx+1 < len(data) {
				s := float64(uint16(data[idx])<<8|uint16(data[idx+1])) / 65535
				values[c] = decode[2*c] + s*(decode[2*c+1]-decode[2*c])
			} else {
				values[c] = decode[2*c]
			}
		}
	case 1, 2, 4:
		samplesPerByte := 8 / bpc
		mask := uint8(1<<bpc - 1)
		maxVal := float64(mask)
		rowBytes := (width*n*bpc + 7) / 8
		rowStart := y * rowBytes
		for c := range n {
			sampleIdx := x*n + c
			byteIdx := rowStart + sampleIdx/samplesPerByte
			bitOffset := (samplesPerByte - 1 - sampleIdx%samplesPerByte) * bpc
			if byteIdx < len(data) {
				s := float64((data[byteIdx]>>bitOffset)&mask) / maxVal
				values[c] = decode[2*c] + s*(decode[2*c+1]-decode[2*c])
			} else {
				values[c] = decode[2*c]
			}
		}
	}
}
