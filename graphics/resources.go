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

	pdf.Embedder[pdf.Unused]
}

// Halftone represents a PDF halftone dictionary or stream.
type Halftone interface {
	// HalftoneType returns the type of the PDF halftone.
	// This is one of 1, 5, 6, 10 or 16.
	HalftoneType() int

	pdf.Embedder[pdf.Unused]
}
