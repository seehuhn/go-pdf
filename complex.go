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

// Rectangle represents a PDF rectangle, given by the coordinates of
// two diagonally opposite corners in a PDF Array.
// TODO(voss): should the values be integers?
type Rectangle struct {
	LLx, LLy, URx, URy float64
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

// PageRotation describes how a page shall be rotated when displayed or
// printed.  The possible values are [RotateInherit], [Rotate0], [Rotate90],
// [Rotate180], [Rotate270].
type PageRotation int

func DecodeRotation(rot Integer) (PageRotation, error) {
	rot = rot % 360
	if rot < 0 {
		rot += 360
	}
	switch rot {
	case 0:
		return Rotate0, nil
	case 90:
		return Rotate90, nil
	case 180:
		return Rotate180, nil
	case 270:
		return Rotate270, nil
	default:
		return 0, errNoRotation
	}
}

func (r PageRotation) ToPDF() Integer {
	switch r {
	case Rotate0:
		return 0
	case Rotate90:
		return 90
	case Rotate180:
		return 180
	case Rotate270:
		return 270
	default:
		return 0
	}
}

// Valid values for PageRotation.
//
// We can't use the pdf integer values directly, because then
// we could not tell apart 0 degree rotations from unspecified
// rotations.
const (
	RotateInherit PageRotation = iota // use inherited value

	Rotate0   // don't rotate
	Rotate90  // rotate 90 degrees clockwise
	Rotate180 // rotate 180 degrees clockwise
	Rotate270 // rotate 270 degrees clockwise
)
