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
	"seehuhn.de/go/pdf"
)

// Attributes specifies Page DefaultAttributes.
//
// These attributes are documented in section 7.7.3.3 of PDF 32000-1:2008.
type Attributes struct {
	Resources pdf.Dict
	MediaBox  *pdf.Rectangle
	CropBox   *pdf.Rectangle
	Rotate    int
}

// DefaultAttributes specifies inheritable Page Attributes.
//
// These attributes are documented in sections 7.7.3.3 and 7.7.3.4 of
// PDF 32000-1:2008.
type DefaultAttributes struct {
	Resources pdf.Dict
	MediaBox  *pdf.Rectangle
	CropBox   *pdf.Rectangle
	Rotate    int
}

// Default paper sizes as PDF rectangles.
// TODO(voss): should these be rounded to integers
var (
	A4     = &pdf.Rectangle{URx: 595.275, URy: 841.889}
	A5     = &pdf.Rectangle{URx: 419.527, URy: 595.275}
	Letter = &pdf.Rectangle{URx: 612, URy: 792}
	Legal  = &pdf.Rectangle{URx: 612, URy: 1008}
)