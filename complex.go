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
	"errors"
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

// GetRectangle resolves references to indirect objects and makes sure the
// resulting object is a PDF rectangle object.
// If the object is null, nil is returned.
func GetRectangle(r Getter, obj Object) (*Rectangle, error) {
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

func (rect *Rectangle) String() string {
	return fmt.Sprintf("[%.2f %.2f %.2f %.2f]", rect.LLx, rect.LLy, rect.URx, rect.URy)
}

// PDF implements the [Object] interface.
func (rect *Rectangle) PDF(w io.Writer) error {
	res := Array{}
	for _, x := range []float64{rect.LLx, rect.LLy, rect.URx, rect.URy} {
		x = math.Round(100*x) / 100
		res = append(res, Number(x))
	}
	return res.PDF(w)
}

// IsZero is true if the rectangle is the zero rectangle object.
func (rect Rectangle) IsZero() bool {
	return rect.LLx == 0 && rect.LLy == 0 && rect.URx == 0 && rect.URy == 0
}

// NearlyEqual reports whether the corner coordinates of two rectangles
// differ by less than `eps`.
func (rect *Rectangle) NearlyEqual(other *Rectangle, eps float64) bool {
	return (math.Abs(rect.LLx-other.LLx) < eps &&
		math.Abs(rect.LLy-other.LLy) < eps &&
		math.Abs(rect.URx-other.URx) < eps &&
		math.Abs(rect.URy-other.URy) < eps)
}

func (rect *Rectangle) XPos(rel float64) float64 {
	return rect.LLx + rel*(rect.URx-rect.LLx)
}

func (rect *Rectangle) YPos(rel float64) float64 {
	return rect.LLy + rel*(rect.URy-rect.LLy)
}

// Extend enlarges the rectangle to also cover `other`.
func (rect *Rectangle) Extend(other *Rectangle) {
	if other.IsZero() {
		return
	}
	if rect.IsZero() {
		*rect = *other
		return
	}
	if other.LLx < rect.LLx {
		rect.LLx = other.LLx
	}
	if other.LLy < rect.LLy {
		rect.LLy = other.LLy
	}
	if other.URx > rect.URx {
		rect.URx = other.URx
	}
	if other.URy > rect.URy {
		rect.URy = other.URy
	}
}

// PageRotation describes how a page shall be rotated when displayed or
// printed.  The possible values are [RotateInherit], [Rotate0], [Rotate90],
// [Rotate180], [Rotate270].
type PageRotation int

func DecodeRotation(rot Integer) (PageRotation, error) {
	rot = rot % 360
	if rot < 0 {
		rot += 360
	}
	switch rot {
	case 0:
		return Rotate0, nil
	case 90:
		return Rotate90, nil
	case 180:
		return Rotate180, nil
	case 270:
		return Rotate270, nil
	default:
		return 0, errNoRotation
	}
}

var errNoRotation = errors.New("not a valid PDF rotation")

func (r PageRotation) ToPDF() Integer {
	switch r {
	case Rotate0:
		return 0
	case Rotate90:
		return 90
	case Rotate180:
		return 180
	case Rotate270:
		return 270
	default:
		return 0
	}
}

// Valid values for PageRotation.
const (
	RotateInherit PageRotation = iota // use inherited value

	Rotate0   // don't rotate
	Rotate90  // rotate 90 degrees clockwise
	Rotate180 // rotate 180 degrees clockwise
	Rotate270 // rotate 270 degrees clockwise
)

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
	MetaData          Reference    `pdf:"optional"`
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

// PageDict represents a PDF page dictionary.
//
// This structure is described in section 7.7.3.3 of PDF 32000-1:2008.
type PageDict struct {
	_ struct{} `pdf:"Type=Page"`

	// Parent specifies the immediate parent of this page object in the page
	// tree.
	Parent Reference

	// LastModified represents the date and time when the page contents were
	// last modified.
	LastModified time.Time `pdf:"optional"`

	// Resources lists the required resources (fonts, images, etc.) for the
	// page.  This is a [Resources] object.
	Resources Object

	// MediaBox defines the boundaries of the physical medium on which the page
	// will be displayed or printed, as a PDF [Rectangle].
	MediaBox Object

	// CropBox defines the visible region of the page's default user space. The
	// page contents will be clipped to this rectangle during display or
	// printing, as a PDF [Rectangle].
	//
	// Default value: the value of MediaBox.
	CropBox Object `pdf:"optional"`

	// BleedBox defines the region to which the contents of the page will be
	// clipped when output in a production environment, as a PDF [Rectangle].
	//
	// Default value: the value of CropBox.
	BleedBox Object `pdf:"optional"`

	// TrimBox defines the intended dimensions of the finished page after
	// trimming, as a PDF [Rectangle].
	//
	// Default value: the value of CropBox.
	TrimBox Object `pdf:"optional"`

	// ArtBox defines the extent of the page's meaningful content, including
	// potential white space, as intended by the page's creator, as a PDF
	// [Rectangle].
	//
	// Default value: the value of CropBox.
	ArtBox Object `pdf:"optional"`

	// BoxColorInfo describes the colors and visual characteristics for
	// displaying guidelines for the different page boundaries.
	BoxColorInfo Object `pdf:"optional"`

	// Contents describes the content stream for the page.  This can be a
	// single stream or an array of streams.
	Contents Object `pdf:"optional"`

	// Rotate describes how the page will be rotated when displayed or printed.
	//
	// Default value: RotateInherit.
	Rotate PageRotation `pdf:"optional"`

	// Group specifies the page's page group for use in the transparent imaging
	// model.
	Group Object `pdf:"optional"`

	// Thumb specifies an image to be used as a thumbnail representing the page
	// visually.
	Thumb Object `pdf:"optional"`

	// B is an array containing references to all article beads appearing on
	// the page, in natural reading order.
	B Object `pdf:"optional"`

	// Dur specifies the maximum length of time, in seconds, that the page will
	// be displayed during presentations before automatically advancing to the
	// next page.
	Dur Number `pdf:"optional"`

	// Trans describes the transition effect for the page during presentations.
	Trans Object `pdf:"optional"`

	// Annots is an array of references to all annotations associated with the
	// page.
	Annots Object `pdf:"optional"`

	// AA contains an additional-actions dictionary that defines actions to be
	// performed when the page is opened or closed.
	AA Object `pdf:"optional"`

	// Metadata contains a PDF stream with metadata for the page.
	Metadata Object `pdf:"optional"`

	// PieceInfo is a page-piece dictionary associated with the page.
	// This can be used to hold private PDF processor data.
	PieceInfo Object `pdf:"optional"`

	// StructParents is the integer key of the page's entry in the structural
	// parent tree.
	StructParents Integer `pdf:"optional"`

	// ID is the digital identifier of the page's parent Web Capture content
	// set.
	ID String `pdf:"optional"`

	// PZ specifies the page's preferred zoom factor when the page is part of a
	// web capture content set.
	PZ Number `pdf:"optional"`

	// SeparationInfo is a separation dictionary that contains information
	// needed to generate color separations for the page.
	SeparationInfo Object `pdf:"optional"`

	// Tabs specifies the tab order of the annotations on the page during
	// keyboard navigation.
	Tabs Object `pdf:"optional"`

	// TemplateInstantiated specifies the name of the template used to
	// instantiate this page.
	TemplateInstantiated Object `pdf:"optional"`

	// PresSteps specifies the presentation steps that will be executed when
	// the page is opened, for use with sub-page navigation.
	PresSteps Object `pdf:"optional"`

	// UserUnit specifies the default user space unit in multiples of 1/72.
	//
	// Default value: 1.0.
	UserUnit Number `pdf:"optional"`

	// VP is an array of viewport dictionaries that define the regions of the
	// page's content.
	VP Object `pdf:"optional"`

	// AF is an array of one or more file specification dictionaries that
	// denote the associated files for this page.
	AF Object `pdf:"optional"`

	// OutputIntents is an array of output intent dictionaries describing the
	// color characteristics of output devices on which the page might be
	// rendered.
	OutputIntents Object `pdf:"optional"`

	// DPart is an indirect reference to the DPart dictionary whose range of
	// pages includes this page object.
	DPart Reference `pdf:"optional"`
}

// Resources describes a PDF Resource Dictionary.
//
// See section 7.8.3 of PDF 32000-1:2008 for details.
type Resources struct {
	ExtGState  Dict  `pdf:"optional"` // maps resource names to graphics state parameter dictionaries
	ColorSpace Dict  `pdf:"optional"` // maps each resource name to either the name of a device-dependent colour space or an array describing a colour space
	Pattern    Dict  `pdf:"optional"` // maps resource names to pattern objects
	Shading    Dict  `pdf:"optional"` // maps resource names to shading dictionaries
	XObject    Dict  `pdf:"optional"` // maps resource names to external objects
	Font       Dict  `pdf:"optional"` // maps resource names to font dictionaries
	ProcSet    Array `pdf:"optional"` // predefined procedure set names
	Properties Dict  `pdf:"optional"` // maps resource names to property list dictionaries for marked content
}
