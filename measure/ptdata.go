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

package measure

import (
	"seehuhn.de/go/pdf"
)

// Predefined Names for PtData
const (
	PtDataNameLat = "LAT" // latitude in degrees (number type)
	PtDataNameLon = "LON" // longitude in degrees (number type)
	PtDataNameAlt = "ALT" // altitude in metres (number type)
)

// Predefined Subtypes for PtData
const (
	PtDataSubtypeCloud = "Cloud" // only defined subtype
)

// PDF 2.0 sections: 12.10

// PtData represents a point data dictionary containing extended geospatial data.
// (PDF 2.0) Point data dictionaries store collections of geographic points
// with associated metadata in a columnar format.
type PtData struct {
	// Subtype specifies the point data type.
	// Currently only "Cloud" is defined in the specification.
	Subtype string

	// Names identifies the data elements in each point array.
	// Predefined names: "LAT" (latitude°), "LON" (longitude°), "ALT" (altitude m).
	// Custom names may be used for domain-specific data.
	Names []string

	// XPTS contains point data as arrays of values.
	// Each inner array corresponds to Names and must have the same length.
	// The collection represents unordered tuples without guaranteed relationships.
	XPTS [][]pdf.Object

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractPtData extracts a PtData object from a PDF dictionary.
func ExtractPtData(r pdf.Getter, obj pdf.Object) (*PtData, error) {
	dict, err := pdf.GetDictTyped(r, obj, "PtData")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing point data dictionary")
	}

	ptData := &PtData{}

	// Extract Subtype (required, should be "Cloud")
	if subtype, err := pdf.Optional(pdf.GetName(r, dict["Subtype"])); err != nil {
		return nil, err
	} else if subtype != "" {
		ptData.Subtype = string(subtype)
	} else {
		ptData.Subtype = PtDataSubtypeCloud
	}

	// Extract Names (required)
	if namesArray, err := pdf.Optional(pdf.GetArray(r, dict["Names"])); err != nil {
		return nil, err
	} else if namesArray != nil {
		names := make([]string, 0, len(namesArray))
		for _, nameObj := range namesArray {
			if name, err := pdf.Optional(pdf.GetName(r, nameObj)); err != nil {
				return nil, err
			} else if name != "" {
				names = append(names, string(name))
			}
		}
		ptData.Names = names
	}

	// Extract XPTS (required)
	if xptsArray, err := pdf.Optional(pdf.GetArray(r, dict["XPTS"])); err != nil {
		return nil, err
	} else if xptsArray != nil {
		xpts := make([][]pdf.Object, 0, len(xptsArray))
		expectedLen := len(ptData.Names)

		for _, pointObj := range xptsArray {
			if pointArray, err := pdf.Optional(pdf.GetArray(r, pointObj)); err != nil {
				return nil, err
			} else if pointArray != nil {
				// Be permissive: truncate or pad to match Names length
				point := make([]pdf.Object, expectedLen)
				for i := 0; i < expectedLen && i < len(pointArray); i++ {
					point[i] = pointArray[i]
				}
				xpts = append(xpts, point)
			}
		}
		ptData.XPTS = xpts
	}

	return ptData, nil
}

// Embed converts the PtData into a PDF object.
func (pd *PtData) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	// Check PDF 2.0 requirement
	if err := pdf.CheckVersion(rm.Out(), "point data dictionaries", pdf.V2_0); err != nil {
		return nil, err
	}

	// Validate Subtype
	if pd.Subtype != PtDataSubtypeCloud {
		return nil, pdf.Error("point data subtype must be \"Cloud\"")
	}

	// Validate Names not empty
	if len(pd.Names) == 0 {
		return nil, pdf.Error("Names array cannot be empty")
	}

	// Build dictionary
	dict := pdf.Dict{
		"Type":    pdf.Name("PtData"),
		"Subtype": pdf.Name(pd.Subtype),
	}

	// Encode Names
	namesArray := make(pdf.Array, len(pd.Names))
	for i, name := range pd.Names {
		namesArray[i] = pdf.Name(name)
	}
	dict["Names"] = namesArray

	// Validate and encode XPTS
	if len(pd.XPTS) > 0 {
		expectedLen := len(pd.Names)
		xptsArray := make(pdf.Array, len(pd.XPTS))

		for i, point := range pd.XPTS {
			if len(point) != expectedLen {
				return nil, pdf.Errorf("XPTS point %d has %d elements, expected %d", i, len(point), expectedLen)
			}

			pointArray := make(pdf.Array, len(point))
			copy(pointArray, point)
			xptsArray[i] = pointArray
		}
		dict["XPTS"] = xptsArray
	} else {
		// Empty XPTS array is valid
		dict["XPTS"] = pdf.Array{}
	}

	if pd.SingleUse {
		return dict, nil
	}

	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}

	return ref, nil
}
