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

package graphics

import (
	"seehuhn.de/go/pdf"
)

// Shading represents a PDF shading dictionary.
//
// Shadings can either be drawn to the page using the [Writer.DrawShading]
// method, or can be used as the basis of a shading pattern.
type Shading interface {
	ShadingType() int

	pdf.Embedder
}

// Image represents a raster image which can be embedded in a PDF file.
type Image interface {
	XObject
	Bounds() Rectangle
}

// Rectangle gives the dimensions of an image.
type Rectangle struct {
	XMin, YMin, XMax, YMax int
}

// Dx returns the width of the rectangle.
func (r Rectangle) Dx() int {
	return r.XMax - r.XMin
}

// Dy returns the height of the rectangle.
func (r Rectangle) Dy() int {
	return r.YMax - r.YMin
}
