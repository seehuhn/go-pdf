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

package pages

import (
	"seehuhn.de/go/pdf"
)

// Resources describes a PDF Resource Dictionary
// See section 7.8.3 of PDF 32000-1:2008 for details.
type Resources struct {
	ExtGState  pdf.Dict  `pdf:"optional"` // maps resource names to graphics state parameter dictionaries
	ColorSpace pdf.Dict  `pdf:"optional"` // maps each resource name to either the name of a device-dependent colour space or an array describing a colour space
	Pattern    pdf.Dict  `pdf:"optional"` // maps resource names to pattern objects
	Shading    pdf.Dict  `pdf:"optional"` // maps resource names to shading dictionaries
	XObject    pdf.Dict  `pdf:"optional"` // maps resource names to external objects
	Font       pdf.Dict  `pdf:"optional"` // maps resource names to font dictionaries
	ProcSet    pdf.Array `pdf:"optional"` // predefined procedure set names
	Properties pdf.Dict  `pdf:"optional"` // maps resource names to property list dictionaries for marked content
}

// Attributes specifies Page DefaultAttributes.
//
// These attributes are documented in section 7.7.3.3 of PDF 32000-1:2008.
type Attributes struct {
	Resources *Resources

	// Mediabox defines the boundaries of the physical
	// medium on which the page shall be displayed or printed.
	MediaBox *pdf.Rectangle

	// Cropbox defines the visible region of default user space.  When the page
	// is displayed or printed, its contents shall be clipped (cropped) to this
	// rectangle and then shall be imposed on the output medium in some
	// implementation-defined manner.  Default value: the value of MediaBox.
	CropBox *pdf.Rectangle

	// Rotate gives the number of degrees by which the page shall be rotated
	// clockwise when displayed or printed.  The value shall be a multiple of
	// 90.
	Rotate int
}

// DefaultAttributes specifies inheritable Page Attributes.
//
// These attributes are documented in sections 7.7.3.3 and 7.7.3.4 of
// PDF 32000-1:2008.
type DefaultAttributes struct {
	Resources pdf.Dict // TODO(voss): use a struct here?
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
