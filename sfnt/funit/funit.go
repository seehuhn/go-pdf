// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package funit

import (
	"math"

	"seehuhn.de/go/pdf"
)

// Int16 is a 16-bit integer in font design units.
type Int16 int16

// AsInteger returns x*scale as a pdf.Integer.
func (x Int16) AsInteger(scale float64) pdf.Integer {
	return pdf.Integer(math.Round(float64(x) * scale))
}

// AsFloat returns x*scale as a float64.
func (x Int16) AsFloat(scale float64) float64 {
	return float64(x) * scale
}

// AsNumber returns x*scale as a pdf.Number.
func (x Int16) AsNumber(scale float64) pdf.Number {
	return pdf.Number(float64(x) * scale)
}

// Rect represents a rectangle in font design units.
type Rect struct {
	LLx, LLy, URx, URy Int16
}

// AsPDF returns the rectangle as a pdf.Rectangle.
func (rect Rect) AsPDF(scale float64) *pdf.Rectangle {
	return &pdf.Rectangle{
		LLx: rect.LLx.AsFloat(scale),
		LLy: rect.LLy.AsFloat(scale),
		URx: rect.URx.AsFloat(scale),
		URy: rect.URy.AsFloat(scale),
	}
}

// IsZero is true if the glyph leaves no marks on the page.
func (rect Rect) IsZero() bool {
	return rect.LLx == 0 && rect.LLy == 0 && rect.URx == 0 && rect.URy == 0
}

// Extend enlarges the rectangle to also cover `other`.
func (rect *Rect) Extend(other Rect) {
	if other.IsZero() {
		return
	}
	if rect.IsZero() {
		*rect = other
		return
	}
	if other.LLx < rect.LLx {
		rect.LLx = other.LLx
	}
	if other.LLy < rect.LLy {
		rect.LLy = other.LLy
	}
	if other.URx > rect.URx {
		rect.URx = other.URx
	}
	if other.URy > rect.URy {
		rect.URy = other.URy
	}
}
