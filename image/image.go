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

// Package image provides functions for embedding images in PDF files.
package image

import (
	"image"

	"seehuhn.de/go/pdf"
)

type Image interface {
	Embed(w pdf.Putter, resName pdf.Name) (Embedded, error)
	Bounds() image.Rectangle
}

type Embedded interface {
	Image
	Reference() pdf.Reference
	ResourceName() pdf.Name
}
