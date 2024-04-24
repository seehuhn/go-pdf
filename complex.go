// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package pdf

// This file contains more complex PDF data structures, which are composed
// of the elementary types from "objects.go".

import (
	"fmt"
	"io"
	"math"
	"time"

	"golang.org/x/text/language"
)

// A Number is either an Integer or a Real.
type Number float64

// PDF implements the [Object] interface.
func (x Number) PDF(w io.Writer) error {
	var obj Object
	if i := Integer(x); Number(i) == x {
		obj = i
	} else {
		obj = Real(x)
	}
	return obj.PDF(w)
}

// GetNumber is a helper function for reading numeric values from a PDF file.
// This resolves indirect references and makes sure the resulting object is an
// Integer or a Real.
func GetNumber(r Getter, obj Object) (Number, error) {
	obj, err := Resolve(r, obj)
	if err != nil {
		return 0, err
	}
	switch x := obj.(type) {
	case Integer:
		return Number(x), nil
	case Real:
		return Number(x), nil
	case Number:
		return x, nil
	case nil:
		return 0, nil
	default:
		return 0, &MalformedFileError{
			Err: fmt.Errorf("expected number but got %T", obj),
		}
	}
}

// Rectangle represents a PDF rectangle.
type Rectangle struct {
	LLx, LLy, URx, URy float64
}

// Dx returns the width of the rectangle.
func (r *Rectangle) Dx() float64 {
	return r.URx - r.LLx
}

// Dy returns the height of the rectangle.
func (r *Rectangle) Dy() float64 {
	return r.URy - r.LLy
}

// GetRectangle resolves references to indirect objects and makes sure the
// resulting object is a PDF rectangle object.
// If the object is null, nil is returned.
func GetRectangle(r Getter, obj Object) (*Rectangle, error) {
	if rect, ok := obj.(*Rectangle); ok {
		return rect, nil
	}

	a, err := GetArray(r, obj)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, nil
	}

	return asRectangle(r, a)
}

// asRectangle converts an array of 4 numbers to a Rectangle object.
// If the array does not have the correct format, an error is returned.
func asRectangle(r Getter, a Array) (*Rectangle, error) {
	if len(a) != 4 {
		return nil, errNoRectangle
	}
	values := [4]float64{}
	for i, obj := range a {
		xi, err := GetNumber(r, obj)
		if err != nil {
			return nil, err
		}
		values[i] = float64(xi)
	}
	rect := &Rectangle{
		LLx: math.Min(values[0], values[2]),
		LLy: math.Min(values[1], values[3]),
		URx: math.Max(values[0], values[2]),
		URy: math.Max(values[1], values[3]),
	}
	return rect, nil
}

func (r *Rectangle) String() string {
	return fmt.Sprintf("[%.2f %.2f %.2f %.2f]", r.LLx, r.LLy, r.URx, r.URy)
}

// PDF implements the [Object] interface.
func (r *Rectangle) PDF(w io.Writer) error {
	res := Array{}
	for _, x := range []float64{r.LLx, r.LLy, r.URx, r.URy} {
		x = math.Round(100*x) / 100
		res = append(res, Number(x))
	}
	return res.PDF(w)
}

// IsZero is true if the rectangle is the zero rectangle object.
func (r Rectangle) IsZero() bool {
	return r.LLx == 0 && r.LLy == 0 && r.URx == 0 && r.URy == 0
}

// NearlyEqual reports whether the corner coordinates of two rectangles
// differ by less than `eps`.
func (r *Rectangle) NearlyEqual(other *Rectangle, eps float64) bool {
	return (math.Abs(r.LLx-other.LLx) < eps &&
		math.Abs(r.LLy-other.LLy) < eps &&
		math.Abs(r.URx-other.URx) < eps &&
		math.Abs(r.URy-other.URy) < eps)
}

func (r *Rectangle) XPos(rel float64) float64 {
	return r.LLx + rel*(r.URx-r.LLx)
}

func (r *Rectangle) YPos(rel float64) float64 {
	return r.LLy + rel*(r.URy-r.LLy)
}

// Extend enlarges the rectangle to also cover `other`.
func (r *Rectangle) Extend(other *Rectangle) {
	if other.IsZero() {
		return
	}
	if r.IsZero() {
		*r = *other
		return
	}
	if other.LLx < r.LLx {
		r.LLx = other.LLx
	}
	if other.LLy < r.LLy {
		r.LLy = other.LLy
	}
	if other.URx > r.URx {
		r.URx = other.URx
	}
	if other.URy > r.URy {
		r.URy = other.URy
	}
}

// Catalog represents a PDF Document Catalog.  The only required field in this
// structure is Pages, which specifies the root of the page tree.
// This struct can be used with the [DecodeDict] and [AsDict] functions.
//
// The Document Catalog is documented in section 7.7.2 of PDF 32000-1:2008.
type Catalog struct {
	_                 struct{} `pdf:"Type=Catalog"`
	Version           Version  `pdf:"optional"`
	Extensions        Object   `pdf:"optional"`
	Pages             Reference
	PageLabels        Object       `pdf:"optional"`
	Names             Object       `pdf:"optional"`
	Dests             Object       `pdf:"optional"`
	ViewerPreferences Object       `pdf:"optional"`
	PageLayout        Name         `pdf:"optional"`
	PageMode          Name         `pdf:"optional"`
	Outlines          Reference    `pdf:"optional"`
	Threads           Reference    `pdf:"optional"`
	OpenAction        Object       `pdf:"optional"`
	AA                Object       `pdf:"optional"`
	URI               Object       `pdf:"optional"`
	AcroForm          Object       `pdf:"optional"`
	Metadata          Reference    `pdf:"optional"`
	StructTreeRoot    Object       `pdf:"optional"`
	MarkInfo          Object       `pdf:"optional"`
	Lang              language.Tag `pdf:"optional"`
	SpiderInfo        Object       `pdf:"optional"`
	OutputIntents     Object       `pdf:"optional"`
	PieceInfo         Object       `pdf:"optional"`
	OCProperties      Object       `pdf:"optional"`
	Perms             Object       `pdf:"optional"`
	Legal             Object       `pdf:"optional"`
	Requirements      Object       `pdf:"optional"`
	Collection        Object       `pdf:"optional"`
	NeedsRendering    bool         `pdf:"optional"`
}

// Info represents a PDF Document Information Dictionary.
// All fields in this structure are optional.
//
// The Document Information Dictionary is documented in section
// 14.3.3 of PDF 32000-1:2008.
type Info struct {
	Title    string `pdf:"text string,optional"`
	Author   string `pdf:"text string,optional"`
	Subject  string `pdf:"text string,optional"`
	Keywords string `pdf:"text string,optional"`

	// Creator gives the name of the application that created the original
	// document, if the document was converted to PDF from another format.
	Creator string `pdf:"text string,optional"`

	// Producer gives the name of the application that converted the document,
	// if the document was converted to PDF from another format.
	Producer string `pdf:"text string,optional"`

	// CreationDate gives the date and time the document was created.
	CreationDate time.Time `pdf:"optional"`

	// ModDate gives the date and time the document was most recently modified.
	ModDate time.Time `pdf:"optional"`

	// Trapped indicates whether the document has been modified to include
	// trapping information.  (A trap is an overlap between adjacent areas of
	// of different colours, used to avoid visual problems caused by imprecise
	// alignment of different layers of ink.) Possible values are:
	//   * "True": The document has been fully trapped.  No further trapping is
	//     necessary.
	//   * "False": The document has not been trapped.
	//   * "Unknown" (default): Either it is unknown whether the document has
	//     been trapped, or the document has been partially trapped.  Further
	//     trapping may be necessary.
	Trapped Name `pdf:"optional,allowstring"`

	// Custom contains all non-standard fields in the Info dictionary.
	Custom map[string]string `pdf:"extra"`
}

// Resources describes a PDF Resource Dictionary.
//
// See section 7.8.3 of PDF 32000-1:2008 for details.
type Resources struct {
	ExtGState  Dict  `pdf:"optional"` // maps resource names to graphics state parameter dictionaries
	ColorSpace Dict  `pdf:"optional"` // maps resource names to colour spaces
	Pattern    Dict  `pdf:"optional"` // maps resource names to pattern objects
	Shading    Dict  `pdf:"optional"` // maps resource names to shading dictionaries
	XObject    Dict  `pdf:"optional"` // maps resource names to external objects
	Font       Dict  `pdf:"optional"` // maps resource names to font dictionaries
	ProcSet    Array `pdf:"optional"` // predefined procedure set names
	Properties Dict  `pdf:"optional"` // maps resource names to property list dictionaries for marked content
}

// IsEmpty returns true if no resources are defined.
func (r *Resources) IsEmpty() bool {
	if r == nil {
		return true
	}
	if len(r.ExtGState) > 0 {
		return false
	}
	if len(r.ColorSpace) > 0 {
		return false
	}
	if len(r.Pattern) > 0 {
		return false
	}
	if len(r.Shading) > 0 {
		return false
	}
	if len(r.XObject) > 0 {
		return false
	}
	if len(r.Font) > 0 {
		return false
	}
	if len(r.ProcSet) > 0 {
		return false
	}
	if len(r.Properties) > 0 {
		return false
	}
	return true
}
