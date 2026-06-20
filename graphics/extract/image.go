// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package extract

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/image"
)

// Image extracts an image dictionary from a PDF stream.
func Image(c pdf.Cursor, obj pdf.Object, isDirect bool) (*image.Dict, error) {
	return image.ExtractDict(c, obj, isDirect)
}

// ImageMask extracts an image mask from a PDF stream.
func ImageMask(c pdf.Cursor, obj pdf.Object, isDirect bool) (*image.Mask, error) {
	return image.ExtractMask(c, obj, isDirect)
}

// SoftMaskImage extracts a soft mask from a PDF stream.
func SoftMaskImage(c pdf.Cursor, obj pdf.Object, isDirect bool) (*image.SoftMask, error) {
	return image.ExtractSoftMask(c, obj, isDirect)
}
