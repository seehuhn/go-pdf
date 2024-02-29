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
	"strconv"

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

	resName map[catRes]pdf.Name

	nesting       []pairType
	markedContent []*MarkedContent
}

type catRes struct {
	cat resourceCategory
	res pdf.Resource
}

type catName struct {
	cat  resourceCategory
	name pdf.Name
}

type resourceCategory byte

// The valid resource categories.
// These corresponds to the fields in the Resources dictionary.
//
// See section 7.8.3 of ISO 32000-2:2020.
const (
	catExtGState resourceCategory = iota + 1
	catColorSpace
	catPattern
	catShading
	catXObject
	catFont
	catProperties
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
	return &Writer{
		Version:       v,
		Content:       w,
		Resources:     &pdf.Resources{},
		currentObject: objPage,

		State: NewState(),

		resName: make(map[catRes]pdf.Name),
	}
}

// GetResourceName returns the name of a resource.
// If the resource is not yet in the resource dictionary, a new name is generated.
func (w *Writer) getResourceName(category resourceCategory, r pdf.Resource) pdf.Name {
	name, ok := w.resName[catRes{category, r}]
	if ok {
		return name
	}

	var field *pdf.Dict
	var prefix pdf.Name
	switch category {
	case catFont:
		field = &w.Resources.Font
		prefix = "F"
	case catExtGState:
		field = &w.Resources.ExtGState
		prefix = "E"
	case catXObject:
		field = &w.Resources.XObject
		prefix = "X"
	case catColorSpace:
		field = &w.Resources.ColorSpace
		prefix = "C"
	case catPattern:
		field = &w.Resources.Pattern
		prefix = "P"
	case catShading:
		field = &w.Resources.Shading
		prefix = "S"
	case catProperties:
		field = &w.Resources.Properties
		prefix = "M"
	default:
		panic("invalid resource category " + strconv.Itoa(int(category)))
	}
	if *field == nil {
		*field = pdf.Dict{}
	}

	isUsed := func(name pdf.Name) bool {
		_, isUsed := (*field)[name]
		return isUsed
	}

	name = r.DefaultName()
	// Some names are forbidden for color spaces,
	// see table 73 of ISO 32000-2:2020
	if category == catColorSpace {
		if n, ok := r.PDFObject().(pdf.Name); ok {
			name = n
		} else if name == "DeviceGray" || name == "DeviceRGB" || name == "DeviceCMYK" || name == "Pattern" {
			name = ""
		}
	}
	if name == "" || isUsed(name) {
		numUsed := len(*field)
		for k := numUsed + 1; ; k-- {
			name = prefix + pdf.Name(strconv.Itoa(k))
			if !isUsed(name) {
				break
			}
		}
	}
	(*field)[name] = r.PDFObject()
	w.resName[catRes{category, r}] = name

	return name
}

// IsValid returns true, if the current graphics object is one of the given types
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
