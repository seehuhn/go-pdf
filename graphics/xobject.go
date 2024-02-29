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

	"seehuhn.de/go/pdf"
)

// Image represents an image which can be embedded in a PDF file.
type Image interface {
	Embed(w pdf.Putter, resName pdf.Name) (EmbeddedImage, error)
	Bounds() Rectangle
}

// EmbeddedImage represents an image which has been embedded in a PDF file.
type EmbeddedImage interface {
	pdf.Resource
	Image
}

// Rectangle gives the dimensions of an image.
type Rectangle struct {
	XMin, YMin, XMax, YMax int
}

// Dx returns the width of the rectangle.
func (r Rectangle) Dx() int {
	return r.XMax - r.XMin
}

// Dy returns the height of the rectangle.
func (r Rectangle) Dy() int {
	return r.YMax - r.YMin
}

// DrawImage draws an image on the page.
func (p *Writer) DrawImage(img pdf.Resource) {
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

// FormXObject represents a PDF Form XObject.
//
// See section 8.10 of ISO 32000-2:2020 for details.
type FormXObject struct {
	pdf.Res
}

// PaintFormXObject draws a Form XObject onto the page.
func (p *Writer) PaintFormXObject(x *FormXObject) {
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
