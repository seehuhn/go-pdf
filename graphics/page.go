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

// TODO(voss): fill in the ProcSet resource.

// Page represents a PDF content stream.
type Page struct {
	Content   io.Writer
	Resources *pdf.Resources
	Err       error

	currentObject objectType
	stack         []*stackEntry

	state *State
	set   StateBits

	resNames map[pdf.Reference]pdf.Name
}

type stackEntry struct {
	state         *State
	isSet         StateBits
	currentObject objectType // TODO: is this needed?
}

// NewPage allocates a new Page object.
func NewPage(w io.Writer) *Page {
	state, isSet := NewState()

	return &Page{
		Content:       w,
		Resources:     &pdf.Resources{},
		currentObject: objPage,

		state: state,
		set:   isSet,

		resNames: make(map[pdf.Reference]pdf.Name),
	}
}

// ForgetGraphicsState removes all information about previous graphics state
// settings.
func (p *Page) ForgetGraphicsState() {
	p.set = 0
}

// PushGraphicsState saves the current graphics state.
func (p *Page) PushGraphicsState() {
	// TODO(voss): does this require certain states?

	state := &stackEntry{
		state:         p.state.Clone(),
		isSet:         p.set,
		currentObject: p.currentObject,
	}
	p.stack = append(p.stack, state)

	_, err := fmt.Fprintln(p.Content, "q")
	if p.Err == nil {
		p.Err = err
	}
}

// PopGraphicsState restores the previous graphics state.
func (p *Page) PopGraphicsState() {
	// TODO(voss): does this require certain states?

	n := len(p.stack) - 1
	entry := p.stack[n]
	p.stack = p.stack[:n]

	p.currentObject = entry.currentObject
	p.state = entry.state
	p.set = entry.isSet

	_, err := fmt.Fprintln(p.Content, "Q")
	if p.Err == nil {
		p.Err = err
	}
}

// isSet returns true, if all of the given fields in the graphics state are set.
func (p *Page) isSet(bits StateBits) bool {
	return p.set&bits == bits
}

func (p *Page) coord(x float64) string {
	// TODO(voss): Think about this some more.  Once we track the current
	// transformation matrix, we can use this to determine the number of digits
	// to keep.
	return float.Format(x, 2)
}

// TODO(voss): remove?
type resource interface {
	Reference() pdf.Reference
	ResourceName() pdf.Name
}

func (p *Page) resourceName(obj resource, d pdf.Dict, nameTmpl string) pdf.Name {
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

// valid returns true, if the current object is one of the given types.
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
