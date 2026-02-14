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
	"math"

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
	// Pix holds pixel values in the color space specified by CS.
	// Each pixel occupies NComp consecutive float64 values.
	// The pixel at (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*NComp].
	Pix []float64

	// Stride is the number of float64 elements per row.
	Stride int

	// Rect is the image bounds.
	Rect image.Rectangle

	// CS is the color space of the pixel data.
	CS color.Space

	// NComp is the number of color components per pixel.
	NComp int
}

// NewData creates a new Data image with the given color space and dimensions.
// All pixels are initialized to the default color of the color space.
func NewData(cs color.Space, width, height int) *Data {
	ncomp := cs.Channels()
	stride := width * ncomp
	pix := make([]float64, height*stride)

	// fill with default color values
	defVals, _ := color.Values(cs.Default())
	if len(defVals) == ncomp {
		for i := 0; i < width*height; i++ {
			copy(pix[i*ncomp:], defVals)
		}
	}

	return &Data{
		Pix:    pix,
		Stride: stride,
		Rect:   image.Rect(0, 0, width, height),
		CS:     cs,
		NComp:  ncomp,
	}
}

// ColorModel returns the color space as a [gocolor.Model].
func (d *Data) ColorModel() gocolor.Model {
	return d.CS
}

// Bounds returns the image bounds.
func (d *Data) Bounds() image.Rectangle {
	return d.Rect
}

// At returns the color at position (x, y).
// If (x, y) is out of bounds, the default color is returned.
func (d *Data) At(x, y int) gocolor.Color {
	w := d.Rect.Dx()
	h := d.Rect.Dy()
	rx := x - d.Rect.Min.X
	ry := y - d.Rect.Min.Y
	if rx < 0 || rx >= w || ry < 0 || ry >= h {
		return d.CS.Default()
	}
	offset := ry*d.Stride + rx*d.NComp
	return color.FromValues(d.CS, d.Pix[offset:offset+d.NComp], nil)
}

// Set sets the color at position (x, y).
// If (x, y) is out of bounds, the call is a no-op.
func (d *Data) Set(x, y int, c gocolor.Color) {
	w := d.Rect.Dx()
	h := d.Rect.Dy()
	rx := x - d.Rect.Min.X
	ry := y - d.Rect.Min.Y
	if rx < 0 || rx >= w || ry < 0 || ry >= h {
		return
	}
	converted := d.CS.Convert(c)
	vals, _ := color.Values(converted.(color.Color))
	offset := ry*d.Stride + rx*d.NComp
	copy(d.Pix[offset:], vals[:d.NComp])
}

// SampleNearest writes the nearest-neighbor pixel value into dst.
// Coordinates are in pixel space where pixel (i,j) is centered at (i+0.5, j+0.5).
// Out-of-bounds coordinates are clamped to the image edge.
func (d *Data) SampleNearest(x, y float64, dst []float64) {
	w := d.Rect.Dx()
	h := d.Rect.Dy()
	ix := clampInt(int(math.Floor(x)), 0, w-1)
	iy := clampInt(int(math.Floor(y)), 0, h-1)
	offset := iy*d.Stride + ix*d.NComp
	copy(dst[:d.NComp], d.Pix[offset:offset+d.NComp])
}

// SampleBilinear writes bilinearly interpolated pixel values into dst.
// Coordinates are in pixel space where pixel (i,j) is centered at (i+0.5, j+0.5).
// Out-of-bounds coordinates are clamped to the image edge.
func (d *Data) SampleBilinear(x, y float64, dst []float64) {
	w := d.Rect.Dx()
	h := d.Rect.Dy()

	// shift to pixel centers: pixel (i,j) is centered at (i+0.5, j+0.5)
	fx := x - 0.5
	fy := y - 0.5

	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	dx := fx - float64(x0)
	dy := fy - float64(y0)

	// clamp to valid pixel range
	x0c := clampInt(x0, 0, w-1)
	x1c := clampInt(x0+1, 0, w-1)
	y0c := clampInt(y0, 0, h-1)
	y1c := clampInt(y0+1, 0, h-1)

	// offsets for the four neighbors
	off00 := y0c*d.Stride + x0c*d.NComp
	off10 := y0c*d.Stride + x1c*d.NComp
	off01 := y1c*d.Stride + x0c*d.NComp
	off11 := y1c*d.Stride + x1c*d.NComp

	w00 := (1 - dx) * (1 - dy)
	w10 := dx * (1 - dy)
	w01 := (1 - dx) * dy
	w11 := dx * dy

	for c := range d.NComp {
		dst[c] = w00*d.Pix[off00+c] +
			w10*d.Pix[off10+c] +
			w01*d.Pix[off01+c] +
			w11*d.Pix[off11+c]
	}
}

// ToRGBA converts the image to sRGB, returning an [*image.RGBA].
func (d *Data) ToRGBA() *image.RGBA {
	w := d.Rect.Dx()
	h := d.Rect.Dy()
	out := image.NewRGBA(d.Rect)
	for y := range h {
		for x := range w {
			offset := y*d.Stride + x*d.NComp
			c := color.FromValues(d.CS, d.Pix[offset:offset+d.NComp], nil)
			r, g, b, a := c.RGBA()
			i := out.PixOffset(x+d.Rect.Min.X, y+d.Rect.Min.Y)
			out.Pix[i+0] = uint8(r >> 8)
			out.Pix[i+1] = uint8(g >> 8)
			out.Pix[i+2] = uint8(b >> 8)
			out.Pix[i+3] = uint8(a >> 8)
		}
	}
	return out
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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

	stride := width * ncomp
	return &Data{
		Pix:    pix,
		Stride: stride,
		Rect:   image.Rect(0, 0, width, height),
		CS:     cs,
		NComp:  ncomp,
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
