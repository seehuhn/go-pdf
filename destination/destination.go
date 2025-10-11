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

// Destination represents a PDF destination that specifies a particular view of a document.
// Destinations can be explicit (specifying page and view parameters) or named
// (referencing a destination by name that must be looked up in the document catalog).
//
// PDF 2.0 section: 12.3.2
type Destination interface {
	DestinationType() Type
	Encode(rm *pdf.ResourceManager) (pdf.Object, error)
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

// XYZ displays the page with coordinates (Left, Top) positioned at the upper-left
// corner of the window and contents magnified by Zoom factor.
// Use Unset (or any NaN value) for parameters that should retain their current value.
// A Zoom of 0 has the same meaning as Unset.
type XYZ struct {
	Page            Target
	Left, Top, Zoom float64
}

func (d *XYZ) DestinationType() Type { return TypeXYZ }

func (d *XYZ) Encode(rm *pdf.ResourceManager) (pdf.Object, error) {
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

// encodeOptionalNumber converts a float64 to a PDF object, using null for Unset/NaN
func encodeOptionalNumber(v float64) pdf.Object {
	if math.IsNaN(v) {
		return nil // PDF null
	}
	return pdf.Number(v)
}

// Fit displays the page magnified to fit entirely within the window
// both horizontally and vertically. If the required horizontal and vertical
// magnification factors are different, uses the smaller of the two,
// centering the page within the window in the other dimension.
type Fit struct {
	Page Target
}

func (d *Fit) DestinationType() Type { return TypeFit }

func (d *Fit) Encode(rm *pdf.ResourceManager) (pdf.Object, error) {
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

func (d *FitH) Encode(rm *pdf.ResourceManager) (pdf.Object, error) {
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

func (d *FitV) Encode(rm *pdf.ResourceManager) (pdf.Object, error) {
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

func (d *FitR) Encode(rm *pdf.ResourceManager) (pdf.Object, error) {
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

func (d *FitB) Encode(rm *pdf.ResourceManager) (pdf.Object, error) {
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

func (d *FitBH) Encode(rm *pdf.ResourceManager) (pdf.Object, error) {
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

func (d *FitBV) Encode(rm *pdf.ResourceManager) (pdf.Object, error) {
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
