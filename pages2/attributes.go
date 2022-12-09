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

package pages2

import "seehuhn.de/go/pdf"

// InheritableAttributes specifies inheritable Page Attributes.
//
// These attributes are documented in sections 7.7.3.3 and 7.7.3.4 of
// PDF 32000-1:2008.
type InheritableAttributes struct {
	Resources *pdf.Resources

	// Mediabox defines the boundaries of the physical
	// medium on which the page shall be displayed or printed.
	MediaBox *pdf.Rectangle

	// Cropbox defines the visible region of default user space.  When
	// the page is displayed or printed, its contents shall be clipped
	// (cropped) to this rectangle and then shall be imposed on the output
	// medium in some implementation-defined manner.
	// Default value: the value of MediaBox.
	CropBox *pdf.Rectangle

	// Rotate describes how the page shall be rotated when displayed or
	// printed.  Default value: RotateInherit.
	Rotate PageRotation
}

func mergeAttributes(dict pdf.Dict, attr *InheritableAttributes) {
	if attr.Resources != nil && dict["Resources"] == nil {
		// TODO(voss): is inheritance per field, or for the whole resources
		// dictionary?
		dict["Resources"] = pdf.AsDict(attr.Resources)
	}
	if attr.MediaBox != nil && dict["MediaBox"] == nil {
		dict["MediaBox"] = attr.MediaBox
	}
	if attr.CropBox != nil && dict["CropBox"] == nil {
		dict["CropBox"] = attr.CropBox
	}
	if attr.Rotate != RotateInherit && dict["Rotate"] == nil {
		dict["Rotate"] = pdf.Integer(attr.Rotate)
	}
}

// PageRotation describes how a page shall be rotated when displayed or
// printed.  The possible values are [RotateInherit], [Rotate0], [Rotate90],
// [Rotate180], [Rotate270].
type PageRotation int

// Valid values for PageRotation.
const (
	RotateInherit PageRotation = iota // use inherited value

	Rotate0   // don't rotate
	Rotate90  // rotate 90 degrees clockwise
	Rotate180 // rotate 180 degrees clockwise
	Rotate270 // rotate 270 degrees clockwise
)

// Default paper sizes as PDF rectangles.
// TODO(voss): should these be rounded to integers?
var (
	A4     = &pdf.Rectangle{URx: 595.275, URy: 841.889}
	A5     = &pdf.Rectangle{URx: 419.527, URy: 595.275}
	Letter = &pdf.Rectangle{URx: 612, URy: 792}
	Legal  = &pdf.Rectangle{URx: 612, URy: 1008}
)
