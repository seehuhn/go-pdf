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

// Writer writes a PDF content stream.
type Writer struct {
	Version   pdf.Version
	Content   io.Writer
	Resources *pdf.Resources
	Err       error

	currentObject objectType

	State
	stack []State

	nesting []pairType

	resName  map[catRes]pdf.Name
	nameUsed map[catName]struct{}
}

type pairType byte

const (
	pairTypeQ   pairType = iota + 1 // q ... Q
	pairTypeBT                      // BT ... ET
	pairTypeBMC                     // BMC ... EMC and BDC ... EMC
	pairTypeBX                      // BX ... EX
)

type catRes struct {
	cat resourceCategory
	res Resource
}

type catName struct {
	cat  resourceCategory
	name pdf.Name
}

// NewWriter allocates a new Writer object.
func NewWriter(w io.Writer, v pdf.Version) *Writer {
	return &Writer{
		Version:       v,
		Content:       w,
		Resources:     &pdf.Resources{},
		currentObject: objPage,

		State: NewState(),

		resName:  make(map[catRes]pdf.Name),
		nameUsed: make(map[catName]struct{}),
	}
}

func (p *Writer) coord(x float64) string {
	// TODO(voss): Think about this some more.  Once we track the current
	// transformation matrix, we can use this to determine the number of digits
	// to keep.
	return float.Format(x, 2)
}

// Res represents a named PDF resource.
type Res struct {
	DefName pdf.Name
	Data    pdf.Reference // TODO(voss): can this be pdf.Object?
}

// DefaultName implements the [Resource] interface.
func (r Res) DefaultName() pdf.Name {
	return r.DefName
}

// PDFObject implements the [Resource] interface.
func (r Res) PDFObject() pdf.Object {
	return r.Data
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
func (p *Writer) getResourceName(category resourceCategory, r Resource) pdf.Name {
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
func (p *Writer) valid(cmd string, ss ...objectType) bool {
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

// PushGraphicsState saves the current graphics state.
func (p *Writer) PushGraphicsState() {
	var allowedStates []objectType
	if p.Version >= pdf.V2_0 {
		allowedStates = []objectType{objPage, objText}
	} else {
		allowedStates = []objectType{objPage}
	}
	if !p.valid("PushGraphicsState", allowedStates...) {
		return
	}

	p.stack = append(p.stack, State{
		Parameters: p.State.Parameters.Clone(),
		Set:        p.State.Set,
	})
	p.nesting = append(p.nesting, pairTypeQ)

	_, err := fmt.Fprintln(p.Content, "q")
	if p.Err == nil {
		p.Err = err
	}
}

// PopGraphicsState restores the previous graphics state.
func (p *Writer) PopGraphicsState() {
	var allowedStates []objectType
	if p.Version >= pdf.V2_0 {
		allowedStates = []objectType{objPage, objText}
	} else {
		allowedStates = []objectType{objPage}
	}
	if !p.valid("PopGraphicsState", allowedStates...) {
		return
	}

	if len(p.nesting) == 0 || p.nesting[len(p.nesting)-1] != pairTypeQ {
		p.Err = fmt.Errorf("PopGraphicsState: no matching PushGraphicsState")
		return
	}
	p.nesting = p.nesting[:len(p.nesting)-1]

	n := len(p.stack) - 1
	savedState := p.stack[n]
	p.stack = p.stack[:n]
	p.State = savedState

	_, err := fmt.Fprintln(p.Content, "Q")
	if p.Err == nil {
		p.Err = err
	}
}

// isSet returns true, if all of the given fields in the graphics state are set.
func (p *Writer) isSet(bits StateBits) bool {
	return p.State.Set&bits == bits
}
