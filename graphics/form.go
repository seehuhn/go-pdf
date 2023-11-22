// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

// FormXObject represents a PDF Form XObject.
//
// See section 8.10 of ISO 32000-2:2020 for details.
type FormXObject struct {
	DefName pdf.Name      // leave empty to generate new names automatically
	Ref     pdf.Reference // The Form XObject content stream
}

func (f *FormXObject) DefaultName() pdf.Name {
	return f.DefName
}

func (f *FormXObject) PDFDict() pdf.Object {
	return f.Ref
}

func (p *Page) PaintFormXObject(x *FormXObject) {
	if !p.valid("PaintFormXObject", objPage, objText) {
		return
	}

	name := p.getResourceName("XObject", x)
	err := name.PDF(p.Content)
	if err != nil {
		p.Err = err
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, " Do")
}
