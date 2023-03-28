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
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/internal/float"
)

type Page struct {
	Content   io.Writer
	Resources *pdf.Resources
	Err       error

	currentObject objectType
	stack         []objectType

	font     font.Embedded
	fontSize float64
	textRise pdf.Integer

	resNames map[pdf.Reference]pdf.Name
}

func NewPage(w io.Writer) *Page {
	return &Page{
		Content:       w,
		currentObject: objPage,
		resNames:      make(map[pdf.Reference]pdf.Name),
	}
}

type objectType int

// See Figure 9 (p. 113) of PDF 32000-1:2008.
const (
	objPage objectType = iota
	objPath
	objText
	objClippingPath
	objShading
	objInlineImage
	objExternal
)

func (s objectType) String() string {
	switch s {
	case objPage:
		return "page"
	case objPath:
		return "path"
	case objText:
		return "text"
	case objClippingPath:
		return "clipping path"
	case objShading:
		return "shading"
	case objInlineImage:
		return "inline image"
	case objExternal:
		return "external"
	default:
		return fmt.Sprintf("objectType(%d)", s)
	}
}

func (p *Page) valid(cmd string, ss ...objectType) bool {
	if p.Err != nil {
		return false
	}

	for _, s := range ss {
		if p.currentObject == s {
			return true
		}
	}

	p.Err = fmt.Errorf("unexpected state %q for %q", p.currentObject, cmd)
	return false
}

func (p *Page) coord(x float64) string {
	// TODO(voss): think about this some more
	return float.Format(x, 2)
}

type Resource interface {
	Reference() pdf.Reference
	ResourceName() pdf.Name
}

func (p *Page) resourceName(obj Resource, d pdf.Dict, nameTmpl string) pdf.Name {
	ref := obj.Reference()
	name, ok := p.resNames[ref]
	if ok {
		return name
	}

	name = obj.ResourceName()
	if _, exists := d[name]; name != "" && !exists {
		d[name] = ref
		p.resNames[ref] = name
		return name
	}

	for k := len(d) + 1; ; k-- {
		name = pdf.Name(fmt.Sprintf(nameTmpl, k))
		if _, exists := d[name]; exists {
			continue
		}

		d[name] = ref
		p.resNames[ref] = name
		return name
	}
}
