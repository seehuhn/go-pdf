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

package graphics

import (
	"seehuhn.de/go/pdf"
)

// This file implements the "XObject operator".
// The operator is defined in table 86 of ISO 32000-2:2020.

func IsImageMask(xobj XObject) bool {
	if xobj.Subtype() != "Image" {
		return false
	}
	if im, ok := xobj.(ImageMask); ok {
		return im.IsImageMask()
	}
	return false
}

// DrawXObject draws a PDF XObject on the page.
//
// This implements the PDF graphics operator "Do".
func (w *Writer) DrawXObject(obj XObject) {
	if !w.isValid("DrawXObject", objPage) {
		return
	}

	name, err := writerGetResourceName(w, catXObject, obj)
	if err != nil {
		w.Err = err
		return
	}

	w.writeObjects(name, pdf.Operator("Do"))
}
