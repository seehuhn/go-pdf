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
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
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

	resName  map[catRes]pdf.Name
	nameUsed map[catName]struct{}

	nesting []pairType
}

type catRes struct {
	cat resourceCategory
	res pdf.Resource
}

type catName struct {
	cat  resourceCategory
	name pdf.Name
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

type pairType byte

const (
	pairTypeQ   pairType = iota + 1 // q ... Q
	pairTypeBT                      // BT ... ET
	pairTypeBMC                     // BMC ... EMC and BDC ... EMC
	pairTypeBX                      // BX ... EX
)

// NewWriter allocates a new Writer object.
func NewWriter(w io.Writer, v pdf.Version) *Writer {
	nameUsed := make(map[catName]struct{})

	// See table 73 of ISO 32000-2:2020
	nameUsed[catName{catColorSpace, "DeviceGray"}] = struct{}{}
	nameUsed[catName{catColorSpace, "DeviceRGB"}] = struct{}{}
	nameUsed[catName{catColorSpace, "DeviceCMYK"}] = struct{}{}
	nameUsed[catName{catColorSpace, "Pattern"}] = struct{}{}

	return &Writer{
		Version:       v,
		Content:       w,
		Resources:     &pdf.Resources{},
		currentObject: objPage,

		State: NewState(),

		resName:  make(map[catRes]pdf.Name),
		nameUsed: nameUsed,
	}
}

// SetStrokeColor sets the stroke color in the graphics state.
// If col is nil, the stroke color is not changed.
func (w *Writer) SetStrokeColor(col color.Color) {
	if !w.isValid("SetStrokeColor", objPage|objText) {
		return
	}
	if w.isSet(StateColorStroke) && col == w.StrokeColor {
		return
	}
	w.StrokeColor = col
	w.Set |= StateColorStroke
	w.Err = col.SetStroke(w.Content)
}

// SetFillColor sets the fill color in the graphics state.
// If col is nil, the fill color is not changed.
func (w *Writer) SetFillColor(col color.Color) {
	if !w.isValid("SetFillColor", objPage|objText) {
		return
	}
	if w.isSet(StateColorFill) && col == w.FillColor {
		return
	}
	w.FillColor = col
	w.Set |= StateColorFill
	w.Err = col.SetFill(w.Content)
}

// GetResourceName returns the name of a resource.
// A new name is generated, if necessary, and the resource is added to the
// resource dictionary for the category.
func (w *Writer) getResourceName(category resourceCategory, r pdf.Resource) pdf.Name {
	name, ok := w.resName[catRes{category, r}]
	if ok {
		return name
	}

	var field *pdf.Dict
	var tmpl string
	switch category {
	case catFont:
		field = &w.Resources.Font
		tmpl = "F%d"
	case catExtGState:
		field = &w.Resources.ExtGState
		tmpl = "E%d"
	case catXObject:
		field = &w.Resources.XObject
		tmpl = "X%d"
	case catColorSpace:
		field = &w.Resources.ColorSpace
		tmpl = "C%d"
	case catPattern:
		field = &w.Resources.Pattern
		tmpl = "P%d"
	case catShading:
		field = &w.Resources.Shading
		tmpl = "S%d"
	case catProperties:
		field = &w.Resources.Properties
		tmpl = "MC%d"
	default:
		panic("invalid resource category " + category)
	}

	isUsed := func(name pdf.Name) bool {
		_, isUsed := w.nameUsed[catName{category, name}]
		return isUsed
	}

	origName := r.DefaultName()
	defName := origName
	if strings.HasPrefix(string(defName), "/") {
		defName = defName[1:]
	}
	if origName != "" && !isUsed(defName) {
		name = defName
	} else {
		var numUsed int
		for item := range w.nameUsed {
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
	w.resName[catRes{category, r}] = name
	w.nameUsed[catName{category, name}] = struct{}{}

	if *field == nil {
		*field = pdf.Dict{}
	}
	(*field)[name] = r.PDFObject()

	return name
}

// isValid returns true, if the current graphics object is one of the given types
// and if p.Err is nil.  Otherwise it sets p.Err and returns false.
func (w *Writer) isValid(cmd string, ss objectType) bool {
	if w.Err != nil {
		return false
	}

	if w.currentObject&ss != 0 {
		return true
	}

	w.Err = fmt.Errorf("unexpected state %q for %q", w.currentObject, cmd)
	return false
}

func (w *Writer) coord(x float64) string {
	// TODO(voss): use the current transformation matrix to determine the
	// number of digits to keep.
	return float.Format(x, 2)
}

// See Figure 9 (p. 113) of PDF 32000-1:2008.
type objectType int

const (
	objPage objectType = 1 << iota
	objPath
	objText
	objClippingPath
	objInlineImage
	// objShading
	// objExternal
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
	case objInlineImage:
		return "inline image"
	default:
		return fmt.Sprintf("objectType(%d)", s)
	}
}
