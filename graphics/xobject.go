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
	"fmt"

	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/image"
)

// DrawImage draws an image on the page.
func (p *Writer) DrawImage(img *image.Embedded) {
	if !p.isValid("DrawImage", objPage) {
		return
	}

	name := p.getResourceName(catXObject, img)
	err := name.PDF(p.Content)
	if err != nil {
		p.Err = err
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, "", "Do")
}

// DrawFormXObject draws a Form XObject onto the page.
//
// This implements the PDF graphics operator "Do".
func (p *Writer) DrawFormXObject(x *form.Embedded) {
	if !p.isValid("PaintFormXObject", objPage|objText) {
		return
	}

	name := p.getResourceName(catXObject, x)
	err := name.PDF(p.Content)
	if err != nil {
		p.Err = err
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, " Do")
}
