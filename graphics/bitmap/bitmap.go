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

package bitmap

import "image"

// Bitmap is a 1-bit-per-pixel image where each pixel is either black or white.
// Pixels are packed MSB-first within bytes, with rows padded to byte boundaries.
// A set bit (1) represents black (foreground).
//
// Bitmap implements [image.Image] and [draw.Image].
type Bitmap struct {
	// Pix holds the packed pixel data.
	// The pixel at (x, y) is stored in Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)/8],
	// at bit position 7 - (x-Rect.Min.X)%8 (MSB-first).
	Pix []byte

	// Stride is the number of bytes per row.
	Stride int

	// Rect is the image bounds.
	Rect image.Rectangle
}

// maxBitmapBytes is a safety cap on the Pix buffer size (128 MB).
// Callers handling untrusted input should validate dimensions before
// calling New; this guard exists only as defence-in-depth.
const maxBitmapBytes = 1 << 27

// New creates a new Bitmap with the given dimensions.
// The bitmap is initially all white (zero).
// If the resulting pixel buffer would exceed [maxBitmapBytes],
// an empty bitmap is returned.
func New(width, height int) *Bitmap {
	if width <= 0 || height <= 0 {
		return &Bitmap{Rect: image.Rectangle{}}
	}
	stride := (width + 7) / 8
	if int64(stride)*int64(height) > maxBitmapBytes {
		return &Bitmap{Rect: image.Rectangle{}}
	}
	return &Bitmap{
		Pix:    make([]byte, stride*height),
		Stride: stride,
		Rect:   image.Rect(0, 0, width, height),
	}
}

// GetPixel returns the pixel value at (x, y).
// It returns true for black (set bit) and false for white.
// If (x, y) is outside the bitmap bounds, it returns false.
func (b *Bitmap) GetPixel(x, y int) bool {
	if !image.Pt(x, y).In(b.Rect) {
		return false
	}
	dx := x - b.Rect.Min.X
	dy := y - b.Rect.Min.Y
	return b.Pix[dy*b.Stride+dx/8]>>(7-dx%8)&1 != 0
}

// SetPixel sets the pixel value at (x, y).
// If v is true, the pixel is set to black; otherwise white.
// If (x, y) is outside the bitmap bounds, the call is a no-op.
func (b *Bitmap) SetPixel(x, y int, v bool) {
	if !image.Pt(x, y).In(b.Rect) {
		return
	}
	dx := x - b.Rect.Min.X
	dy := y - b.Rect.Min.Y
	off := dy*b.Stride + dx/8
	bit := byte(1) << (7 - dx%8)
	if v {
		b.Pix[off] |= bit
	} else {
		b.Pix[off] &^= bit
	}
}

// SubImage returns a Bitmap representing the portion of b visible through r.
// The returned bitmap shares pixels with the original.
func (b *Bitmap) SubImage(r image.Rectangle) *Bitmap {
	r = r.Intersect(b.Rect)
	if r.Empty() {
		return &Bitmap{Rect: r}
	}
	dx := r.Min.X - b.Rect.Min.X
	dy := r.Min.Y - b.Rect.Min.Y
	// SubImage can only share pixel data when the horizontal offset
	// is byte-aligned. Otherwise we must copy.
	if dx%8 != 0 {
		return b.subImageCopy(r)
	}
	off := dy*b.Stride + dx/8
	return &Bitmap{
		Pix:    b.Pix[off:],
		Stride: b.Stride,
		Rect:   r,
	}
}

func (b *Bitmap) subImageCopy(r image.Rectangle) *Bitmap {
	width := r.Dx()
	height := r.Dy()
	dst := New(width, height)
	dst.Rect = r
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dst.SetPixel(x, y, b.GetPixel(x, y))
		}
	}
	return dst
}

// Width returns the width of the bitmap in pixels.
func (b *Bitmap) Width() int {
	return b.Rect.Dx()
}

// Height returns the height of the bitmap in pixels.
func (b *Bitmap) Height() int {
	return b.Rect.Dy()
}
