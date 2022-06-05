// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package pdf

// This file contains more complex PDF data structures, which are composed
// of the elementary types from "objects.go".

import (
	"io"
	"math"
)

// A Number is either an Integer or a Real.
type Number float64

// PDF implements the Object interface.
func (x Number) PDF(w io.Writer) error {
	var obj Object
	if i := Integer(x); Number(i) == x {
		obj = i
	} else {
		obj = Real(x)
	}
	return obj.PDF(w)
}

// Rectangle represents a PDF rectangle, given by the coordinates of
// two diagonally opposite corners in a PDF Array.
// TODO(voss): should the values be integers?
type Rectangle struct {
	LLx, LLy, URx, URy float64
}

// PDF implements the Object interface.
func (rect *Rectangle) PDF(w io.Writer) error {
	res := Array{}
	for _, x := range []float64{rect.LLx, rect.LLy, rect.URx, rect.URy} {
		x = math.Round(100*x) / 100
		res = append(res, Number(x))
	}
	return res.PDF(w)
}

// NearlyEqual reports whether the corner coordinates of two rectangles
// differ by less than `eps`.
func (rect *Rectangle) NearlyEqual(other *Rectangle, eps float64) bool {
	return (math.Abs(rect.LLx-other.LLx) < eps &&
		math.Abs(rect.LLy-other.LLy) < eps &&
		math.Abs(rect.URx-other.URx) < eps &&
		math.Abs(rect.URy-other.URy) < eps)
}

// IsZero is true if the glyph leaves no marks on the page.
func (rect Rectangle) IsZero() bool {
	return rect.LLx == 0 && rect.LLy == 0 && rect.URx == 0 && rect.URy == 0
}

// Extend enlarges the rectangle to also cover `other`.
func (rect *Rectangle) Extend(other *Rectangle) {
	if other.IsZero() {
		return
	}
	if rect.IsZero() {
		*rect = *other
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
