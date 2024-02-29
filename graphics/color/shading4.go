// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package color

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
)

// ShadingType4 represents a type 4 (free-form Gouraud-shaded triangle mesh) shading.
type ShadingType4 struct {
	ColorSpace        Space
	BitsPerCoordinate int
	BitsPerComponent  int
	BitsPerFlag       int
	Decode            pdf.Array

	Vertices []ShadingType4Vertex

	F          function.Func
	Background []float64
	BBox       *pdf.Rectangle
	AntiAlias  bool
}

type ShadingType4Vertex struct {
	X, Y  float64
	Color []float64
}
