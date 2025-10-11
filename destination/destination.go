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
