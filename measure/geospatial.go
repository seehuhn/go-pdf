// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// PDF 2.0 sections: 12.10

// GeospatialMeasure represents a geospatial measure dictionary
// (PDF 2.0, Tables 269-272).
type GeospatialMeasure struct {
	// GCS is the geographic or projected coordinate system (required).
	GCS *CoordinateSystem

	// DCS is the display coordinate system (optional).
	DCS *CoordinateSystem

	// GPTS contains geospatial control points as coordinate pairs.
	// The interpretation depends on GCS: [lat,lon,...] for geographic,
	// [east,north,...] for projected systems.
	GPTS []float64

	// LPTS contains corresponding points in the unit square, pairwise.
	// If present, must have the same length as GPTS.
	LPTS []float64

	// Bounds defines the neatline polygon in the unit square.
	// Nil means the default (full unit square).
	Bounds []float64

	// PDU specifies preferred display units as [linear, area, angular].
	// A zero array means absent.
	PDU [3]pdf.Name

	// PCSM is a 12-element projected coordinate system matrix.
	// Nil means absent.
	PCSM []float64

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// MeasureType returns the type of measure dictionary.
func (gm *GeospatialMeasure) MeasureType() pdf.Name {
	return "GEO"
}

// Embed converts the GeospatialMeasure into a PDF object.
func (gm *GeospatialMeasure) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "geospatial measure dictionaries", pdf.V2_0); err != nil {
		return nil, err
	}

	// validate required fields
	if gm.GCS == nil {
		return nil, pdf.Error("missing required GCS")
	}
	if len(gm.GPTS) == 0 {
		return nil, pdf.Error("missing required GPTS")
	}
	if len(gm.GPTS)%2 != 0 {
		return nil, pdf.Error("GPTS must have even length")
	}
	if gm.LPTS != nil && len(gm.LPTS) != len(gm.GPTS) {
		return nil, pdf.Error("LPTS length must match GPTS length")
	}
	if gm.Bounds != nil && len(gm.Bounds)%2 != 0 {
		return nil, pdf.Error("Bounds must have even length")
	}
	if gm.PCSM != nil && len(gm.PCSM) != 12 {
		return nil, pdf.Error("PCSM must have exactly 12 elements")
	}

	// validate PDU: if any is set, all three must be set
	pduSet := gm.PDU[0] != "" || gm.PDU[1] != "" || gm.PDU[2] != ""
	if pduSet && (gm.PDU[0] == "" || gm.PDU[1] == "" || gm.PDU[2] == "") {
		return nil, pdf.Error("PDU must have all three units or none")
	}

	dict := pdf.Dict{}

	// optional Type field
	if e.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Measure")
	}

	dict["Subtype"] = pdf.Name("GEO")

	// embed GCS (required)
	gcsObj, err := e.Embed(gm.GCS)
	if err != nil {
		return nil, err
	}
	dict["GCS"] = gcsObj

	// embed DCS (optional)
	if gm.DCS != nil {
		dcsObj, err := e.Embed(gm.DCS)
		if err != nil {
			return nil, err
		}
		dict["DCS"] = dcsObj
	}

	// GPTS (required)
	dict["GPTS"] = floatsToArray(gm.GPTS)

	// LPTS (optional)
	if gm.LPTS != nil {
		dict["LPTS"] = floatsToArray(gm.LPTS)
	}

	// Bounds (optional, nil = default)
	if gm.Bounds != nil {
		dict["Bounds"] = floatsToArray(gm.Bounds)
	}

	// PDU (optional)
	if pduSet {
		dict["PDU"] = pdf.Array{gm.PDU[0], gm.PDU[1], gm.PDU[2]}
	}

	// PCSM (optional)
	if gm.PCSM != nil {
		dict["PCSM"] = floatsToArray(gm.PCSM)
	}

	if gm.SingleUse {
		return dict, nil
	}

	ref := e.Alloc()
	err = e.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// floatsToArray converts a float64 slice to a PDF array.
func floatsToArray(vals []float64) pdf.Array {
	arr := make(pdf.Array, len(vals))
	for i, v := range vals {
		arr[i] = pdf.Number(v)
	}
	return arr
}
