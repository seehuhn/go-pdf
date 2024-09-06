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
	"fmt"

	"seehuhn.de/go/pdf"
)

// This file implements the "XObject operator".
// The operator is defined in table 86 of ISO 32000-2:2020.

// XObject represents a PDF XObject.
type XObject interface {
	Subtype() pdf.Name
	pdf.Embedder[pdf.Unused]
}

// DrawXObject draws a PDF XObject on the page.
// This can be used to draw images, forms, or other XObjects.
//
// This implements the PDF graphics operator "Do".
func (w *Writer) DrawXObject(obj XObject) {
	if !w.isValid("DrawImage", objPage) {
		return
	}

	name, _, err := writerGetResourceName(w, catXObject, obj)
	if err != nil {
		w.Err = err
		return
	}

	w.writeObject(name)
	if w.Err != nil {
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, "", "Do")
}
