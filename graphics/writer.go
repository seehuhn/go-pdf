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
	"seehuhn.de/go/pdf/font"
)

// Writer writes a PDF content stream.
type Writer struct {
	Content   io.Writer
	Resources *pdf.Resources
	Err       error

	currentObject objectType

	CurrentFont font.Layouter

	State
	stack []State

	RM      *pdf.ResourceManager
	resName map[catRes]objName

	nesting       []pairType
	markedContent []*MarkedContent

	glyphBuf *font.GlyphSeq
}

type catRes struct {
	cat resourceCategory
	res any
}

type objName struct {
	obj  any
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
	pairTypeBMC                     // BMC/BDC ... EMC
	pairTypeBX                      // BX ... EX
)

// NewWriter allocates a new Writer object.
func NewWriter(out io.Writer, rm *pdf.ResourceManager) *Writer {
	return &Writer{
		Content:       out,
		Resources:     &pdf.Resources{},
		currentObject: objPage,

		State: NewState(),

		RM:      rm,
		resName: make(map[catRes]objName),

		glyphBuf: &font.GlyphSeq{},
	}
}

// NewStream resets the writer for a new content stream.
// The new content stream shares the resource dictionary with the previous
// content stream.
func (w *Writer) NewStream(out io.Writer) {
	w.Content = out
	w.currentObject = objPage
	w.State = NewState()
	w.stack = w.stack[:0]
	w.Err = nil
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
	// number of digits to keep?
	return format(x)
}

// GetResourceName returns a name which can be used to refer to a resource from
// within the content stream.  If needed, the resource is embedded in the PDF
// file and/or added to the resource dictionary.
//
// Once Go supports methods with type parameters, this function can be turned
// into a method on [Writer].
func writerGetResourceName[T any](w *Writer, cat resourceCategory, resource pdf.Embedder[T]) (pdf.Name, T, error) {
	key := catRes{cat, resource}
	v, ok := w.resName[key]
	if ok {
		return v.name, v.obj.(T), nil
	}

	obj, embedded, err := pdf.ResourceManagerEmbed(w.RM, resource)
	if err != nil {
		var zero T
		return "", zero, err
	}

	dict := w.getCategoryDict(cat)
	name := w.generateName(cat, dict)
	(*dict)[name] = obj

	w.resName[key] = objName{embedded, name}
	return name, embedded, nil
}

// GetResourceName returns a name which can be used to refer to a resource from
// within the content stream.  If needed, the resource is embedded in the PDF
// file and/or added to the resource dictionary.
//
// Once Go supports methods with type parameters, this function can be turned
// into a method on [Writer].
func writerSetResourceName[T any](w *Writer, resource pdf.Embedder[T], category resourceCategory, name pdf.Name) error {
	for k, v := range w.resName {
		if k.cat == category && v.name == name {
			return fmt.Errorf("name %q is already used for category %d", name, category)
		}
	}

	// Some names for color spaces are reserved,
	// see table 73 of ISO 32000-2:2020
	if category == catColorSpace &&
		(name == "DeviceGray" ||
			name == "DeviceRGB" ||
			name == "DeviceCMYK" ||
			name == "Pattern") {
		return fmt.Errorf("name %q is reserved for color spaces", name)
	}

	dictData, embedded, err := pdf.ResourceManagerEmbed(w.RM, resource)
	if err != nil {
		return err
	}

	dict := w.getCategoryDict(category)
	(*dict)[name] = dictData

	key := catRes{category, resource}
	w.resName[key] = objName{embedded, name}
	return nil
}

// SetFontNameInternal controls how the font is refered to in the content
// stream.  Normally, a name is allocated automatically, so use of this
// function is not normally required.
func (w *Writer) SetFontNameInternal(f font.Font, name pdf.Name) error {
	return writerSetResourceName(w, f, catFont, name)
}

func (w *Writer) getCategoryDict(category resourceCategory) *pdf.Dict {
	var field *pdf.Dict
	switch category {
	case catFont:
		field = &w.Resources.Font
	case catExtGState:
		field = &w.Resources.ExtGState
	case catXObject:
		field = &w.Resources.XObject
	case catColorSpace:
		field = &w.Resources.ColorSpace
	case catPattern:
		field = &w.Resources.Pattern
	case catShading:
		field = &w.Resources.Shading
	case catProperties:
		field = &w.Resources.Properties
	default:
		panic("invalid resource category")
	}

	if *field == nil {
		*field = pdf.Dict{}
	}

	return field
}

func (w *Writer) generateName(category resourceCategory, dict *pdf.Dict) pdf.Name {
	var name pdf.Name

	prefix := getCategoryPrefix(category)
	numUsed := len(*dict)
	for k := numUsed + 1; ; k-- {
		name = prefix + pdf.Name(strconv.Itoa(k))
		if _, isUsed := (*dict)[name]; !isUsed {
			break
		}
	}

	return name
}

func getCategoryPrefix(category resourceCategory) pdf.Name {
	switch category {
	case catFont:
		return "F"
	case catExtGState:
		return "E"
	case catXObject:
		return "X"
	case catColorSpace:
		return "C"
	case catPattern:
		return "P"
	case catShading:
		return "S"
	case catProperties:
		return "M"
	default:
		panic("invalid resource category")
	}
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

func format(x float64) string {
	return strconv.FormatFloat(x, 'f', -1, 64)
}
