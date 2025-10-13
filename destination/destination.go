// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package destination

import (
	"math"

	"seehuhn.de/go/pdf"
)

// Destination represents a PDF destination that specifies a particular view of
// a document. Destinations can be explicit (specifying page and view
// parameters) or named (referencing a destination by name that must be looked
// up in the document catalog).
type Destination interface {
	DestinationType() Type
	pdf.Encoder
}

// Type identifies the type of destination.
type Type pdf.Name

const (
	TypeXYZ   Type = "XYZ"
	TypeFit   Type = "Fit"
	TypeFitH  Type = "FitH"
	TypeFitV  Type = "FitV"
	TypeFitR  Type = "FitR"
	TypeFitB  Type = "FitB"
	TypeFitBH Type = "FitBH"
	TypeFitBV Type = "FitBV"
	TypeNamed Type = "Named"
)

// Target specifies the destination page. This can be:
//   - pdf.Reference: An indirect reference to a page object (most common case)
//   - pdf.Integer: A page number for remote/embedded go-to actions
//   - pdf.Reference to structure element: For structure destinations
type Target pdf.Object

// Unset is a sentinel value for coordinates that should retain their current value.
// Use math.IsNaN() to test for this value.
var Unset = math.NaN()

// Decode reads a destination from a PDF object.
// The object can be an array (explicit destination), a name/string (named destination),
// or a dictionary with a D entry.
func Decode(x *pdf.Extractor, obj pdf.Object) (Destination, error) {
	obj, err := pdf.Resolve(x.R, obj)
	if err != nil {
		return nil, err
	}

	// Handle named destinations (name or string)
	if name, ok := obj.(pdf.Name); ok {
		return &Named{Name: pdf.String(name)}, nil
	}
	if str, ok := obj.(pdf.String); ok {
		return &Named{Name: str}, nil
	}

	// Handle dictionary wrapper with D entry
	if dict, _ := pdf.GetDict(x.R, obj); dict != nil {
		if dObj := dict["D"]; dObj != nil {
			obj = dObj
		}
	}

	// Must be an array for explicit destination
	arr, err := pdf.GetArray(x.R, obj)
	if err != nil {
		return nil, err
	}
	if len(arr) < 2 {
		return nil, pdf.Error("destination array too short")
	}

	// First element is the page/target
	page := Target(arr[0])

	// Second element is the type name
	typeName, err := pdf.Optional(pdf.GetName(x.R, arr[1]))
	if err != nil {
		return nil, err
	}

	switch Type(typeName) {
	case TypeXYZ:
		if len(arr) < 5 {
			return nil, pdf.Error("XYZ destination requires 5 elements")
		}
		left := getOptionalNumber(x.R, arr[2])
		top := getOptionalNumber(x.R, arr[3])
		zoom := getOptionalNumber(x.R, arr[4])
		return &XYZ{Page: page, Left: left, Top: top, Zoom: zoom}, nil

	case TypeFit:
		return &Fit{Page: page}, nil

	case TypeFitH:
		if len(arr) < 3 {
			return nil, pdf.Error("FitH destination requires 3 elements")
		}
		top := getOptionalNumber(x.R, arr[2])
		return &FitH{Page: page, Top: top}, nil

	case TypeFitV:
		if len(arr) < 3 {
			return nil, pdf.Error("FitV destination requires 3 elements")
		}
		left := getOptionalNumber(x.R, arr[2])
		return &FitV{Page: page, Left: left}, nil

	case TypeFitR:
		if len(arr) < 6 {
			return nil, pdf.Error("FitR destination requires 6 elements")
		}
		left, _ := pdf.Optional(pdf.GetNumber(x.R, arr[2]))
		bottom, _ := pdf.Optional(pdf.GetNumber(x.R, arr[3]))
		right, _ := pdf.Optional(pdf.GetNumber(x.R, arr[4]))
		top, _ := pdf.Optional(pdf.GetNumber(x.R, arr[5]))
		return &FitR{Page: page, Left: float64(left), Bottom: float64(bottom),
			Right: float64(right), Top: float64(top)}, nil

	case TypeFitB:
		return &FitB{Page: page}, nil

	case TypeFitBH:
		if len(arr) < 3 {
			return nil, pdf.Error("FitBH destination requires 3 elements")
		}
		top := getOptionalNumber(x.R, arr[2])
		return &FitBH{Page: page, Top: top}, nil

	case TypeFitBV:
		if len(arr) < 3 {
			return nil, pdf.Error("FitBV destination requires 3 elements")
		}
		left := getOptionalNumber(x.R, arr[2])
		return &FitBV{Page: page, Left: left}, nil

	default:
		return nil, pdf.Error("unknown destination type: " + string(typeName))
	}
}

// XYZ displays the page with coordinates (Left, Top) positioned at the
// upper-left corner of the window and contents magnified by Zoom factor. Use
// Unset (or any NaN value) for parameters that should retain their current
// value. A Zoom of 0 has the same meaning as Unset.
type XYZ struct {
	Page            Target
	Left, Top, Zoom float64
}

func (d *XYZ) DestinationType() Type { return TypeXYZ }

func (d *XYZ) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := validateFinite("Left", d.Left); err != nil {
		return nil, err
	}
	if err := validateFinite("Top", d.Top); err != nil {
		return nil, err
	}
	if err := validateFinite("Zoom", d.Zoom); err != nil {
		return nil, err
	}

	return pdf.Array{
		d.Page,
		pdf.Name(TypeXYZ),
		encodeOptionalNumber(d.Left),
		encodeOptionalNumber(d.Top),
		encodeOptionalNumber(d.Zoom),
	}, nil
}

// Fit displays the page magnified to fit entirely within the window
// both horizontally and vertically. If the required horizontal and vertical
// magnification factors are different, uses the smaller of the two,
// centering the page within the window in the other dimension.
type Fit struct {
	Page Target
}

func (d *Fit) DestinationType() Type { return TypeFit }

func (d *Fit) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	return pdf.Array{
		d.Page,
		pdf.Name(TypeFit),
	}, nil
}

// FitH displays the page with the vertical coordinate Top positioned at the
// top edge of the window and contents magnified to fit the entire width of
// the page within the window.
// Use Unset (or any NaN value) for Top to retain the current value.
type FitH struct {
	Page Target
	Top  float64
}

func (d *FitH) DestinationType() Type { return TypeFitH }

func (d *FitH) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := validateFinite("Top", d.Top); err != nil {
		return nil, err
	}

	return pdf.Array{
		d.Page,
		pdf.Name(TypeFitH),
		encodeOptionalNumber(d.Top),
	}, nil
}

// FitV displays the page with the horizontal coordinate Left positioned at
// the left edge of the window and contents magnified to fit the entire height
// of the page within the window.
// Use Unset (or any NaN value) for Left to retain the current value.
type FitV struct {
	Page Target
	Left float64
}

func (d *FitV) DestinationType() Type { return TypeFitV }

func (d *FitV) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := validateFinite("Left", d.Left); err != nil {
		return nil, err
	}

	return pdf.Array{
		d.Page,
		pdf.Name(TypeFitV),
		encodeOptionalNumber(d.Left),
	}, nil
}

// FitR displays the page with contents magnified to fit the rectangle
// specified by the coordinates entirely within the window. If the required
// horizontal and vertical magnification factors are different, uses the
// smaller of the two, centering the rectangle within the window in the other dimension.
type FitR struct {
	Page                     Target
	Left, Bottom, Right, Top float64
}

func (d *FitR) DestinationType() Type { return TypeFitR }

func (d *FitR) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if math.IsNaN(d.Left) || math.IsInf(d.Left, 0) {
		return nil, pdf.Error("Left must be a finite number")
	}
	if math.IsNaN(d.Bottom) || math.IsInf(d.Bottom, 0) {
		return nil, pdf.Error("Bottom must be a finite number")
	}
	if math.IsNaN(d.Right) || math.IsInf(d.Right, 0) {
		return nil, pdf.Error("Right must be a finite number")
	}
	if math.IsNaN(d.Top) || math.IsInf(d.Top, 0) {
		return nil, pdf.Error("Top must be a finite number")
	}

	if d.Left >= d.Right {
		return nil, pdf.Error("Left must be less than Right")
	}
	if d.Bottom >= d.Top {
		return nil, pdf.Error("Bottom must be less than Top")
	}

	return pdf.Array{
		d.Page,
		pdf.Name(TypeFitR),
		pdf.Number(d.Left),
		pdf.Number(d.Bottom),
		pdf.Number(d.Right),
		pdf.Number(d.Top),
	}, nil
}

// FitB displays the page with contents magnified to fit the page's bounding
// box entirely within the window. If the required horizontal and vertical
// magnification factors are different, uses the smaller of the two,
// centering the bounding box within the window in the other dimension.
// Requires PDF 1.1.
type FitB struct {
	Page Target
}

func (d *FitB) DestinationType() Type { return TypeFitB }

func (d *FitB) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "FitB destination", pdf.V1_1); err != nil {
		return nil, err
	}
	return pdf.Array{
		d.Page,
		pdf.Name(TypeFitB),
	}, nil
}

// FitBH displays the page with the vertical coordinate Top positioned at the
// top edge of the window and contents magnified to fit the entire width of
// the page's bounding box within the window.
// Use Unset (or any NaN value) for Top to retain the current value.
// Requires PDF 1.1.
type FitBH struct {
	Page Target
	Top  float64
}

func (d *FitBH) DestinationType() Type { return TypeFitBH }

func (d *FitBH) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "FitBH destination", pdf.V1_1); err != nil {
		return nil, err
	}
	if err := validateFinite("Top", d.Top); err != nil {
		return nil, err
	}

	return pdf.Array{
		d.Page,
		pdf.Name(TypeFitBH),
		encodeOptionalNumber(d.Top),
	}, nil
}

// FitBV displays the page with the horizontal coordinate Left positioned at
// the left edge of the window and contents magnified to fit the entire height
// of the page's bounding box within the window.
// Use Unset (or any NaN value) for Left to retain the current value.
// Requires PDF 1.1.
type FitBV struct {
	Page Target
	Left float64
}

func (d *FitBV) DestinationType() Type { return TypeFitBV }

func (d *FitBV) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "FitBV destination", pdf.V1_1); err != nil {
		return nil, err
	}
	if err := validateFinite("Left", d.Left); err != nil {
		return nil, err
	}

	return pdf.Array{
		d.Page,
		pdf.Name(TypeFitBV),
		encodeOptionalNumber(d.Left),
	}, nil
}

// Named represents a named destination that must be looked up in the document
// catalog's Dests dictionary or Names/Dests name tree. The Name field contains
// the lookup key.
type Named struct {
	Name pdf.String
}

func (d *Named) DestinationType() Type { return TypeNamed }

func (d *Named) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if len(d.Name) == 0 {
		return nil, pdf.Error("named destination must have a non-empty name")
	}
	// Always use pdf.String (modern PDF 1.2+ format)
	return d.Name, nil
}

// getOptionalNumber reads a number from a PDF object, treating null as Unset
func getOptionalNumber(r pdf.Getter, obj pdf.Object) float64 {
	if obj == nil {
		return Unset
	}
	num, err := pdf.Optional(pdf.GetNumber(r, obj))
	if err != nil {
		return Unset
	}
	return float64(num)
}

// encodeOptionalNumber converts a float64 to a PDF object, using null for Unset/NaN
func encodeOptionalNumber(v float64) pdf.Object {
	if math.IsNaN(v) {
		return nil // PDF null
	}
	return pdf.Number(v)
}

// validateFinite checks that a value is either Unset (NaN) or a finite number
func validateFinite(field string, v float64) error {
	if math.IsNaN(v) {
		return nil // Unset is valid
	}
	if math.IsInf(v, 0) {
		return pdf.Error(field + " must be either Unset or a finite number")
	}
	return nil
}
