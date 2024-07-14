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

package pattern

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/matrix"
)

// TilingProperties describes the properties of a tiling pattern.
type TilingProperties struct {
	// TilingType is a a code that controls adjustments to the spacing of tiles
	// relative to the device pixel grid.
	TilingType int

	// The pattern cell's bounding box.
	// The pattern cell is clipped to this rectangle before it is painted.
	BBox *pdf.Rectangle

	// XStep is the horizontal spacing between pattern cells.
	XStep float64

	// YStep is the vertical spacing between pattern cells.
	YStep float64

	// Matrix is an array of six numbers specifying the pattern cell's matrix.
	// Leave this empty to use the identity matrix.
	Matrix matrix.Matrix
}
