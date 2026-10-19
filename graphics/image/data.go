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
	// Each pixel occupies NComp consecutive float32 values.
	// The pixel at (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*NComp].
	Pix []float32

	// Stride is the number of float32 elements per row.
	Stride int

	// Rect is the image bounds.
	Rect image.Rectangle

	// CS is the color space of the pixel data.
	CS color.Space

	// NComp is the number of color components per pixel.
	NComp int

	// Alpha (optional) holds the opacity of each pixel, as values in the
	// range [0, 1].  The pixel at (x, y) has opacity
	// Alpha[(y-Rect.Min.Y)*Rect.Dx() + (x-Rect.Min.X)].
	// Alpha is nil if the image has no opacity information.
	// The colour values in Pix are not premultiplied by the opacity.
	Alpha []float32

	// KeyMask (optional) is the colour key mask given by the MaskColors
	// entry of the image dictionary, evaluated on the samples of this
	// image: 0 at pixels where every component lies in its key range,
	// and 255 elsewhere.  It has the bounds Rect.  KeyMask is nil if the
	// dictionary has no colour key, or one which does not have two
	// entries per colour component.
	KeyMask *image.Alpha
}

// NewData creates a new Data image with the given color space and dimensions.
// All pixels are initialized to the default color of the color space.
func NewData(cs color.Space, width, height int) *Data {
	ncomp := cs.Channels()
	stride := width * ncomp
	pix := make([]float32, height*stride)

	// fill with default color values
	defVals, _ := color.Values(cs.Default())
	if len(defVals) == ncomp {
		for i := 0; i < width*height; i++ {
			for c, v := range defVals {
				pix[i*ncomp+c] = float32(v)
			}
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

// At returns the color at position (x, y), ignoring any opacity.
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
	vals := make([]float64, d.NComp)
	for c, v := range d.Pix[offset : offset+d.NComp] {
		vals[c] = float64(v)
	}
	return color.FromValues(d.CS, vals, nil)
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
	for i := range d.NComp {
		d.Pix[offset+i] = float32(vals[i])
	}
}

// SampleNearest writes the nearest-neighbor pixel value into dst.
// Coordinates are in pixel space where pixel (i,j) is centered at (i+0.5, j+0.5).
// Out-of-bounds coordinates are clamped to the image edge.
func (d *Data) SampleNearest(x, y float64, dst []float32) {
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
func (d *Data) SampleBilinear(x, y float64, dst []float32) {
	w := d.Rect.Dx()
	h := d.Rect.Dy()

	// shift to pixel centers: pixel (i,j) is centered at (i+0.5, j+0.5)
	fx := x - 0.5
	fy := y - 0.5

	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	dx := float32(fx - float64(x0))
	dy := float32(fy - float64(y0))

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
// If d.Alpha is set, the result carries the opacity, premultiplied
// into the colour values as [image.RGBA] requires.
func (d *Data) ToRGBA() *image.RGBA {
	w := d.Rect.Dx()
	h := d.Rect.Dy()
	out := image.NewRGBA(d.Rect)
	vals := make([]float64, d.NComp)
	for y := range h {
		for x := range w {
			offset := y*d.Stride + x*d.NComp
			for i, v := range d.Pix[offset : offset+d.NComp] {
				vals[i] = float64(v)
			}
			c := color.FromValues(d.CS, vals, nil)
			r, g, b, a := c.RGBA()
			if d.Alpha != nil {
				alpha := min(max(float64(d.Alpha[y*w+x]), 0), 1)
				r = uint32(float64(r)*alpha + 0.5)
				g = uint32(float64(g)*alpha + 0.5)
				b = uint32(float64(b)*alpha + 0.5)
				a = uint32(float64(a)*alpha + 0.5)
			}
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
//
// For JPXDecode images, the image size and bit depth are taken from the
// JPEG 2000 data, and so is the colour space unless the dictionary
// specifies one.  If SMaskInData is non-zero, the opacity channel of the
// JPEG 2000 data, if any, is returned in [Data.Alpha].  Consumers must
// therefore not assume that [Data.Rect] matches d.Width and d.Height.
//
// If d.MaskColors gives a colour key, [Data.KeyMask] holds the mask it
// defines.  The key refers to the raw samples, before the Decode array
// is applied; samples missing from short data count as 0.  For a
// JPXDecode image, the samples are those of the JPEG 2000 data, at a
// common depth: that of the deepest colour component, at most 16 bits.
// Components of other depths are rescaled to it.  For an image with a
// palette whose colour space is Indexed, either given by d.ColorSpace or
// derived from the palette, the key refers to the palette indices; where
// the palette has to be applied to give the colour space its number of
// components, the key refers to the palette entries, at the depth of the
// palette.  Opacity channels are ignored.  Key values beyond the sample
// range never match.
//
// Load is equivalent to LoadReduced(0).
func (d *Dict) Load() (*Data, error) {
	return d.LoadReduced(0)
}

// LoadReduced is like [Dict.Load], but decodes JPXDecode images at
// reduced resolution: the result has roughly 1/2^reduce of the full
// width and height, where reduce is clamped to the resolution levels the
// JPEG 2000 data offers.  [Data.Rect] and [Data.Alpha] have the reduced
// size; the colour space, the Decode array and the colour key are
// handled as in Load.  Consumers must not assume that Data.Rect matches
// d.Width and d.Height.
//
// For all other encodings, LoadReduced returns the same result as Load.
func (d *Dict) LoadReduced(reduce int) (*Data, error) {
	if d.Data != nil && d.Data.IsJPX() {
		return d.loadJPX(reduce)
	}
	cs := d.ColorSpace
	if cs == nil {
		return nil, errors.New("missing ColorSpace")
	}
	if d.BitsPerComponent == 0 {
		return nil, errors.New("missing BitsPerComponent")
	}
	ncomp := cs.Channels()
	if ncomp == 0 {
		return nil, errors.New("pattern color spaces are not valid for images")
	}

	// a Decode array shorter than 2*ncomp would make readSamples index out of
	// range; fall back to the default, as the read path in ExtractDict does
	decode := d.Decode
	if len(decode) < 2*ncomp {
		decode = DefaultDecode(cs, d.BitsPerComponent)
	}

	data, err := d.Data.Pixels()
	if err != nil {
		return nil, err
	}
	data = normalizeData(data, expectedDataSize(d.Width, ncomp, d.BitsPerComponent, d.Height))

	width, height := d.Width, d.Height
	bpc := d.BitsPerComponent
	pix := make([]float32, width*height*ncomp)
	vals := make([]float32, ncomp)

	if bpc == 8 && len(data) >= len(pix) {
		// 8-bit samples are byte-aligned and contiguous, so decoding each
		// one is a table lookup
		lut := decodeLUT8(ncomp, decode)
		for i := 0; i < len(pix); i += ncomp {
			for c := range ncomp {
				pix[i+c] = lut[c][data[i+c]]
			}
		}
	} else {
		for y := range height {
			for x := range width {
				readSamples(data, width, ncomp, bpc, x, y, decode, vals)
				offset := (y*width + x) * ncomp
				copy(pix[offset:], vals)
			}
		}
	}

	var keyMask *image.Alpha
	if len(d.MaskColors) == 2*ncomp {
		keyMask = rawColorKey(data, width, height, ncomp, bpc, d.MaskColors)
	}

	stride := width * ncomp
	return &Data{
		Pix:     pix,
		Stride:  stride,
		Rect:    image.Rect(0, 0, width, height),
		CS:      cs,
		NComp:   ncomp,
		KeyMask: keyMask,
	}, nil
}

// decodeLUT8 tabulates the decoded component value of every 8-bit sample
// value, one table per colour channel.  The tables reproduce the arithmetic
// [readSamples] performs, so the two agree bit for bit.
func decodeLUT8(ncomp int, decode []float64) [][256]float32 {
	lut := make([][256]float32, ncomp)
	for c := range ncomp {
		lo, hi := decode[2*c], decode[2*c+1]
		for v := range 256 {
			lut[c][v] = float32(lo + float64(v)/255*(hi-lo))
		}
	}
	return lut
}

// readSamples reads n color component values for pixel (x, y) from raw image
// data, applying the Decode mapping to produce color space values.
// bpc is assumed to be one of the supported depths (1, 2, 4, 8, or 16).
func readSamples(data []byte, width, n, bpc, x, y int, decode []float64, values []float32) {
	switch bpc {
	case 8:
		rowStart := y * width * n
		pixStart := rowStart + x*n
		for c := range n {
			idx := pixStart + c
			if idx < len(data) {
				s := float64(data[idx]) / 255
				values[c] = float32(decode[2*c] + s*(decode[2*c+1]-decode[2*c]))
			} else {
				values[c] = float32(decode[2*c])
			}
		}
	case 16:
		rowStart := y * width * n * 2
		pixStart := rowStart + x*n*2
		for c := range n {
			idx := pixStart + c*2
			if idx+1 < len(data) {
				s := float64(uint16(data[idx])<<8|uint16(data[idx+1])) / 65535
				values[c] = float32(decode[2*c] + s*(decode[2*c+1]-decode[2*c]))
			} else {
				values[c] = float32(decode[2*c])
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
				values[c] = float32(decode[2*c] + s*(decode[2*c+1]-decode[2*c]))
			} else {
				values[c] = float32(decode[2*c])
			}
		}
	}
}
