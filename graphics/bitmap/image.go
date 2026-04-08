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

import (
	"image"
	"image/color"
)

var (
	_ image.Image = (*Bitmap)(nil)
	_ drawImage   = (*Bitmap)(nil)
)

// drawImage is the interface for a settable image, matching [draw.Image].
type drawImage interface {
	image.Image
	Set(x, y int, c color.Color)
}

// colorModel converts any color to black or white.
// Colors with luminance >= 50% become white (0), others become black (1).
var colorModel = color.ModelFunc(func(c color.Color) color.Color {
	r, g, b, _ := c.RGBA()
	// luminance approximation using BT.601 weights
	lum := (299*r + 587*g + 114*b) / 1000
	if lum >= 0x8000 {
		return color.White
	}
	return color.Black
})

// ColorModel returns a [color.Model] that converts colors to black or white.
func (b *Bitmap) ColorModel() color.Model {
	return colorModel
}

// Bounds returns the bitmap's bounding rectangle.
func (b *Bitmap) Bounds() image.Rectangle {
	return b.Rect
}

// At returns the color at (x, y).
// Black pixels return [color.Black], white pixels return [color.White].
func (b *Bitmap) At(x, y int) color.Color {
	if b.GetPixel(x, y) {
		return color.Black
	}
	return color.White
}

// Set sets the pixel at (x, y) to the given color.
// The color is converted to black or white using the bitmap's color model.
func (b *Bitmap) Set(x, y int, c color.Color) {
	r, g, b2, _ := c.RGBA()
	lum := (299*r + 587*g + 114*b2) / 1000
	b.SetPixel(x, y, lum < 0x8000)
}
