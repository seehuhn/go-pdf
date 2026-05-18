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
	"bufio"
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
//
// JPEG header parsing happens synchronously inside Decode, so malformed
// headers and oversize-dimension errors surface as the returned error.
// Pixel conversion happens lazily as the returned reader is drained.
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

	pr, pw := io.Pipe()
	go func() {
		bw := bufio.NewWriter(pw)
		if err := convertImage(img, bw); err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := bw.Flush(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()
	return pr, nil
}

// convertImage writes raw pixel bytes for img to w, row by row.  See
// [Decode] for the wire format.
func convertImage(img image.Image, w io.Writer) error {
	bounds := img.Bounds()
	width := bounds.Dx()

	switch img := img.(type) {
	case *image.YCbCr:
		row := make([]byte, width*3)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			i := 0
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				yi := img.YOffset(x, y)
				ci := img.COffset(x, y)
				r, g, b := color.YCbCrToRGB(img.Y[yi], img.Cb[ci], img.Cr[ci])
				row[i] = r
				row[i+1] = g
				row[i+2] = b
				i += 3
			}
			if _, err := w.Write(row); err != nil {
				return err
			}
		}

	case *image.Gray:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			srcOff := (y-bounds.Min.Y)*img.Stride + (bounds.Min.X - img.Rect.Min.X)
			if _, err := w.Write(img.Pix[srcOff : srcOff+width]); err != nil {
				return err
			}
		}

	case *image.CMYK:
		// Go's JPEG decoder uses the Adobe convention where 0 means full ink.
		// PDF uses 0 = no ink, so we invert all CMYK bytes.
		row := make([]byte, width*4)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			srcOff := (y-bounds.Min.Y)*img.Stride + (bounds.Min.X-img.Rect.Min.X)*4
			for i := range width * 4 {
				row[i] = 255 - img.Pix[srcOff+i]
			}
			if _, err := w.Write(row); err != nil {
				return err
			}
		}

	default:
		// generic fallback: produce RGB
		row := make([]byte, width*3)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			i := 0
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				row[i] = uint8(r >> 8)
				row[i+1] = uint8(g >> 8)
				row[i+2] = uint8(b >> 8)
				i += 3
			}
			if _, err := w.Write(row); err != nil {
				return err
			}
		}
	}
	return nil
}
