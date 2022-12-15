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

// DrawImage draws an image on the page.
func (p *Page) DrawImage(imageRef *pdf.Reference) {
	// TODO(voss): check that the states are correct
	// if !p.valid("Stroke", statePath, stateClipped) {
	// 	return
	// }

	if p.imageNames == nil {
		p.imageNames = make(map[pdf.Reference]pdf.Name)
	}

	name, ok := p.imageNames[*imageRef]
	if !ok {
		name = pdf.Name(fmt.Sprintf("Im%d", len(p.imageNames)+1))
		p.imageNames[*imageRef] = name
		if p.resources == nil {
			p.resources = &pdf.Resources{}
		}
		if p.resources.XObject == nil {
			p.resources.XObject = pdf.Dict{}
		}
		p.resources.XObject[name] = imageRef
	}

	err := name.PDF(p.content)
	if err != nil {
		p.err = err
		return
	}
	_, p.err = fmt.Fprintln(p.content, "", "Do")
}
