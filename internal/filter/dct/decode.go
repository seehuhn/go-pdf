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

package dct

import (
	"bytes"
	"image"
	"image/color"
	"io"

	"seehuhn.de/go/pdf/internal/filter/dct/jpeg"
)

// Decode decodes JPEG data from r and returns the raw pixel bytes.
// If colorTransform is non-nil, it overrides the JPEG's APP14-based
// color transform decision per PDF spec table 13.
//
// The output contains interleaved channel bytes, row by row, with no padding.
// For color images, the output is RGB (3 bytes per pixel).
// For grayscale images, the output is 1 byte per pixel.
// For CMYK images, the output is 4 bytes per pixel.
func Decode(r io.Reader, colorTransform *int) (io.ReadCloser, error) {
	var img image.Image
	var err error
	if colorTransform != nil {
		img, err = jpeg.DecodeWithOptions(r, colorTransform)
	} else {
		img, err = jpeg.Decode(r)
	}
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	var buf []byte

	switch img := img.(type) {
	case *image.YCbCr:
		buf = make([]byte, w*h*3)
		i := 0
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				yi := img.YOffset(x, y)
				ci := img.COffset(x, y)
				r, g, b := color.YCbCrToRGB(img.Y[yi], img.Cb[ci], img.Cr[ci])
				buf[i] = r
				buf[i+1] = g
				buf[i+2] = b
				i += 3
			}
		}

	case *image.Gray:
		buf = make([]byte, w*h)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			srcOff := (y-bounds.Min.Y)*img.Stride + (bounds.Min.X - img.Rect.Min.X)
			dstOff := (y - bounds.Min.Y) * w
			copy(buf[dstOff:dstOff+w], img.Pix[srcOff:srcOff+w])
		}

	case *image.CMYK:
		// Go's JPEG decoder uses the Adobe convention where 0 means full ink.
		// PDF uses 0 = no ink, so we invert all CMYK bytes.
		buf = make([]byte, w*h*4)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			srcOff := (y-bounds.Min.Y)*img.Stride + (bounds.Min.X-img.Rect.Min.X)*4
			dstOff := (y - bounds.Min.Y) * w * 4
			for i := range w * 4 {
				buf[dstOff+i] = 255 - img.Pix[srcOff+i]
			}
		}

	default:
		// generic fallback: produce RGB
		buf = make([]byte, w*h*3)
		i := 0
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				buf[i] = uint8(r >> 8)
				buf[i+1] = uint8(g >> 8)
				buf[i+2] = uint8(b >> 8)
				i += 3
			}
		}
	}

	return io.NopCloser(bytes.NewReader(buf)), nil
}
