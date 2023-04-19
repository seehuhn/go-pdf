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
	"fmt"
	"io"
	"math"
)

// A Number is either an Integer or a Real.
type Number float64

// PDF implements the [Object] interface.
func (x Number) PDF(w io.Writer) error {
	var obj Object
	if i := Integer(x); Number(i) == x {
		obj = i
	} else {
		obj = Real(x)
	}
	return obj.PDF(w)
}

// Rectangle represents a PDF rectangle.
type Rectangle struct {
	LLx, LLy, URx, URy float64
}

// asRectangle converts an array of 4 numbers to a Rectangle object.
// If the array does not have the correct format, an error is returned.
func (x Array) asRectangle() (*Rectangle, error) {
	if len(x) != 4 {
		return nil, errNoRectangle
	}
	values := [4]float64{}
	for i, obj := range x {
		switch obj := obj.(type) {
		case Integer:
			values[i] = float64(obj)
		case Number:
			values[i] = float64(obj)
		case Real:
			values[i] = float64(obj)
		default:
			return nil, errNoRectangle
		}
	}
	rect := &Rectangle{
		LLx: math.Min(values[0], values[2]),
		LLy: math.Min(values[1], values[3]),
		URx: math.Max(values[0], values[2]),
		URy: math.Max(values[1], values[3]),
	}
	return rect, nil
}

func (rect *Rectangle) String() string {
	return fmt.Sprintf("[%.2f %.2f %.2f %.2f]", rect.LLx, rect.LLy, rect.URx, rect.URy)
}

// PDF implements the [Object] interface.
func (rect *Rectangle) PDF(w io.Writer) error {
	res := Array{}
	for _, x := range []float64{rect.LLx, rect.LLy, rect.URx, rect.URy} {
		x = math.Round(100*x) / 100
		res = append(res, Number(x))
	}
	return res.PDF(w)
}

// IsZero is true if the rectangle is the zero rectangle object.
func (rect Rectangle) IsZero() bool {
	return rect.LLx == 0 && rect.LLy == 0 && rect.URx == 0 && rect.URy == 0
}

// NearlyEqual reports whether the corner coordinates of two rectangles
// differ by less than `eps`.
func (rect *Rectangle) NearlyEqual(other *Rectangle, eps float64) bool {
	return (math.Abs(rect.LLx-other.LLx) < eps &&
		math.Abs(rect.LLy-other.LLy) < eps &&
		math.Abs(rect.URx-other.URx) < eps &&
		math.Abs(rect.URy-other.URy) < eps)
}

func (rect *Rectangle) XPos(rel float64) float64 {
	return rect.LLx + rel*(rect.URx-rect.LLx)
}

func (rect *Rectangle) YPos(rel float64) float64 {
	return rect.LLy + rel*(rect.URy-rect.LLy)
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
