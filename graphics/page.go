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
	"seehuhn.de/go/pdf/internal/float"
)

// Page represents a PDF content stream.
type Page struct {
	Content   io.Writer
	Resources *pdf.Resources
	Err       error

	currentObject objectType

	state State
	stack []State

	resName  map[catRes]pdf.Name
	nameUsed map[catName]struct{}
}

type catRes struct {
	cat resourceCategory
	res Resource
}

type catName struct {
	cat  resourceCategory
	name pdf.Name
}

// NewPage allocates a new Page object.
func NewPage(w io.Writer) *Page {
	return &Page{
		Content:       w,
		Resources:     &pdf.Resources{},
		currentObject: objPage,

		state: NewState(),

		resName:  make(map[catRes]pdf.Name),
		nameUsed: make(map[catName]struct{}),
	}
}

// ForgetGraphicsState removes all information about previous graphics state
// settings.
//
// TODO(voss): remove
func (p *Page) ForgetGraphicsState() {
	p.state.Set = 0
}

// PushGraphicsState saves the current graphics state.
func (p *Page) PushGraphicsState() {
	if !p.valid("PushGraphicsState", objPage, objText) {
		return
	}

	p.stack = append(p.stack, p.state.Clone())

	_, err := fmt.Fprintln(p.Content, "q")
	if p.Err == nil {
		p.Err = err
	}
}

// PopGraphicsState restores the previous graphics state.
func (p *Page) PopGraphicsState() {
	if !p.valid("PopGraphicsState", objPage, objText) {
		return
	}

	n := len(p.stack) - 1
	savedState := p.stack[n]
	p.stack = p.stack[:n]

	p.state = savedState

	_, err := fmt.Fprintln(p.Content, "Q")
	if p.Err == nil {
		p.Err = err
	}
}

// isSet returns true, if all of the given fields in the graphics state are set.
func (p *Page) isSet(bits StateBits) bool {
	return p.state.Set&bits == bits
}

func (p *Page) coord(x float64) string {
	// TODO(voss): Think about this some more.  Once we track the current
	// transformation matrix, we can use this to determine the number of digits
	// to keep.
	return float.Format(x, 2)
}

// Res represents a PDF resource.
type Res struct {
	DefName pdf.Name
	Ref     pdf.Reference
}

// DefaultName implements the [Resource] interface.
func (r Res) DefaultName() pdf.Name {
	return r.DefName
}

// PDFObject implements the [Resource] interface.
func (r Res) PDFObject() pdf.Object {
	return r.Ref
}

// Resource represents the different PDF Resource types.
// Implementations of this must be "comparable".
type Resource interface {
	DefaultName() pdf.Name // return "" to choose names automatically
	PDFObject() pdf.Object // value to use in the resource dictionary
}

type resourceCategory pdf.Name

// The valid resource categories.
// These corresponds to the fields in the Resources dictionary.
//
// See section 7.8.3 of ISO 32000-2:2020.
const (
	catExtGState  resourceCategory = "ExtGState"
	catColorSpace resourceCategory = "ColorSpace"
	catPattern    resourceCategory = "Pattern"
	catShading    resourceCategory = "Shading"
	catXObject    resourceCategory = "XObject"
	catFont       resourceCategory = "Font"
	catProperties resourceCategory = "Properties"
)

// GetResourceName returns the name of a resource.
// A new name is generated, if necessary, and the resource is added to the
// resource dictionary for the category.
func (p *Page) getResourceName(category resourceCategory, r Resource) pdf.Name {
	name, ok := p.resName[catRes{category, r}]
	if ok {
		return name
	}

	var field *pdf.Dict
	var tmpl string
	switch category {
	case catFont:
		field = &p.Resources.Font
		tmpl = "F%d"
	case catExtGState:
		field = &p.Resources.ExtGState
		tmpl = "E%d"
	case catXObject:
		field = &p.Resources.XObject
		tmpl = "X%d"
	case catColorSpace:
		field = &p.Resources.ColorSpace
		tmpl = "C%d"
	case catPattern:
		field = &p.Resources.Pattern
		tmpl = "P%d"
	case catShading:
		field = &p.Resources.Shading
		tmpl = "S%d"
	case catProperties:
		field = &p.Resources.Properties
		tmpl = "MC%d"
	default:
		panic("invalid resource category " + category)
	}

	isUsed := func(name pdf.Name) bool {
		_, isUsed := p.nameUsed[catName{category, name}]
		return isUsed
	}

	if defName := r.DefaultName(); defName != "" && !isUsed(defName) {
		name = defName
	} else {
		var numUsed int
		for item := range p.nameUsed {
			if item.cat == category {
				numUsed++
			}
		}
		for k := numUsed + 1; ; k-- {
			name = pdf.Name(fmt.Sprintf(tmpl, k))
			if !isUsed(name) {
				break
			}
		}
	}
	p.resName[catRes{category, r}] = name
	p.nameUsed[catName{category, name}] = struct{}{}

	if *field == nil {
		*field = pdf.Dict{}
	}
	(*field)[name] = r.PDFObject()

	return name
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

// valid returns true, if the current graphics object is one of the given types.
// Otherwise it sets p.Err and returns false.
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
