// seehuhn.de/go/pdf - support for reading and writing PDF files
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

package pages

import (
	"math"

	"seehuhn.de/go/pdf"
)

// Rectangle represents a PDF rectangle, given by the coordinates of
// two diagonally opposite corners.
type Rectangle struct {
	LLx, LLy, URx, URy float64
}

// ToObject converts the rectangle to its representation in a PDF file.
func (rect *Rectangle) ToObject() pdf.Array {
	res := pdf.Array{}
	for _, x := range []float64{rect.LLx, rect.LLy, rect.URx, rect.URy} {
		if i := pdf.Integer(x); float64(i) == x {
			res = append(res, i)
		} else {
			res = append(res, pdf.Real(x))
		}
	}
	return res
}

// NearlyEqual reports whether the corner coordinates of two rectangles
// differ by less than `eps`.
func (rect *Rectangle) NearlyEqual(other *Rectangle, eps float64) bool {
	return (math.Abs(rect.LLx-other.LLx) < eps &&
		math.Abs(rect.LLy-other.LLy) < eps &&
		math.Abs(rect.URx-other.URx) < eps &&
		math.Abs(rect.URy-other.URy) < eps)
}

// Attributes specifies Page DefaultAttributes.
//
// These attributes are documented in section 7.7.3.3 of PDF 32000-1:2008.
type Attributes struct {
	Resources pdf.Dict
	MediaBox  *Rectangle
	CropBox   *Rectangle
	Rotate    int
}

// DefaultAttributes specifies inheritable Page Attributes.
//
// These attributes are documented in sections 7.7.3.3 and 7.7.3.4 of
// PDF 32000-1:2008.
type DefaultAttributes struct {
	Resources pdf.Dict
	MediaBox  *Rectangle
	CropBox   *Rectangle
	Rotate    int
}

// Default paper sizes as PDF rectangles.
var (
	A4     = &Rectangle{0, 0, 595.275, 841.889}
	A5     = &Rectangle{0, 0, 419.527, 595.275}
	Letter = &Rectangle{0, 0, 612, 792}
	Legal  = &Rectangle{0, 0, 612, 1008}
)
