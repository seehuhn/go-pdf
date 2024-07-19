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
	Version   pdf.Version // TODO(voss): remove
	Content   io.Writer
	Resources *pdf.Resources
	Err       error

	currentObject objectType

	State
	stack []State

	RM         *pdf.ResourceManager
	resName    map[catRes]pdf.Name
	resNameOld map[catRes]pdf.Name

	nesting       []pairType
	markedContent []*MarkedContent

	glyphBuf *font.GlyphSeq
}

type catRes struct {
	cat resourceCategory
	res any
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
func NewWriter(w io.Writer, rm *pdf.ResourceManager) *Writer {
	v := pdf.GetVersion(rm.Out)
	return &Writer{
		Version:       v,
		Content:       w,
		Resources:     &pdf.Resources{},
		currentObject: objPage,

		State: NewState(),

		RM:         rm,
		resName:    make(map[catRes]pdf.Name),
		resNameOld: make(map[catRes]pdf.Name),

		glyphBuf: &font.GlyphSeq{},
	}
}

// GetResourceName returns a name which can be used to refer to a resource from
// within the content stream.  If needed, the resource is embedded in the PDF
// file and/or added to the resource dictionary.
//
// Once Go supports methods with type parameters, this function can be turned
// into a method on [Writer].
func writerGetResourceName[T pdf.Resource](w *Writer, resource pdf.Embedder[T], category resourceCategory) (pdf.Name, error) {
	key := catRes{category, resource}
	name, ok := w.resName[key]
	if ok {
		return name, nil
	}

	embedded, err := pdf.ResourceManagerEmbed(w.RM, resource)
	if err != nil {
		return "", err
	}

	dict := w.getCategoryDict(category)
	name = w.generateName(category, dict, "")
	(*dict)[name] = embedded.PDFObject()

	w.resName[key] = name
	return name, nil
}

// GetResourceName returns a name which can be used to refer to a resource from
// within the content stream.  If needed, the resource is embedded in the PDF
// file and/or added to the resource dictionary.
//
// Once Go supports methods with type parameters, this function can be turned
// into a method on [Writer].
func writerSetResourceName[T pdf.Resource](w *Writer, resource pdf.Embedder[T], category resourceCategory, name pdf.Name) error {
	for k, v := range w.resName {
		if k.cat == category && v == name {
			return fmt.Errorf("name %q is already used for category %d", name, category)
		}
	}

	_, err := pdf.ResourceManagerEmbed(w.RM, resource)
	if err != nil {
		return err
	}

	key := catRes{category, resource}
	w.resName[key] = name
	return nil
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

func (w *Writer) generateName(category resourceCategory, dict *pdf.Dict, defName pdf.Name) pdf.Name {
	isUsed := func(name pdf.Name) bool {
		// Some names for color spaces are reserved,
		// see table 73 of ISO 32000-2:2020
		if category == catColorSpace &&
			(name == "DeviceGray" ||
				name == "DeviceRGB" ||
				name == "DeviceCMYK" ||
				name == "Pattern") {
			return true
		}

		_, isUsed := (*dict)[name]
		return isUsed
	}

	name := defName
	if name == "" || isUsed(name) {
		prefix := getCategoryPrefix(category)
		numUsed := len(*dict)
		for k := numUsed + 1; ; k-- {
			name = prefix + pdf.Name(strconv.Itoa(k))
			if !isUsed(name) {
				break
			}
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

// GetResourceName returns the name of a resource.
// If the resource is not yet in the resource dictionary, a new name is generated.
func (w *Writer) getResourceNameOld(category resourceCategory, r pdf.Resource) pdf.Name {
	name, ok := w.resNameOld[catRes{category, r}]
	if ok {
		return name
	}

	field := w.getCategoryDict(category)
	name = w.generateName(category, field, "")
	(*field)[name] = r.PDFObject()
	w.resNameOld[catRes{category, r}] = name

	return name
}

// SetFontNameInternal controls how the font is refered to in the content
// stream.  Normally, a name is allocated automatically, so use of this
// function is not normally required.
func (w *Writer) SetFontNameInternal(f font.Embedded, name pdf.Name) error {
	// TODO(voss): convert to use writerSetResourceName

	for k, v := range w.resNameOld {
		if k.cat == catFont && v == name {
			return fmt.Errorf("name %q is already used", name)
		}
	}

	field := w.getCategoryDict(catFont)
	(*field)[name] = f.PDFObject()
	w.resNameOld[catRes{catFont, f}] = name

	return nil
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
