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

// CoordinateSystemType identifies the type of coordinate system.
type CoordinateSystemType pdf.Name

const (
	// CoordSysGeographic represents a geographic coordinate system (Table 270).
	CoordSysGeographic CoordinateSystemType = "GEOGCS"

	// CoordSysProjected represents a projected coordinate system (Table 271).
	CoordSysProjected CoordinateSystemType = "PROJCS"
)

// CoordinateSystem represents a geographic or projected coordinate system
// dictionary (PDF 2.0, Tables 270-271).
type CoordinateSystem struct {
	// CSType is the coordinate system type ("GEOGCS" or "PROJCS").
	CSType CoordinateSystemType

	// EPSG is the EPSG reference code identifying this coordinate system.
	// Zero means absent.
	EPSG int

	// WKT is the Well Known Text description of this coordinate system.
	// Empty means absent.
	WKT string

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractCoordinateSystem extracts a coordinate system dictionary from a PDF file.
func ExtractCoordinateSystem(c pdf.Cursor, obj pdf.Object, isDirect bool) (*CoordinateSystem, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, pdf.Error("missing coordinate system dictionary")
	}

	cs := &CoordinateSystem{}

	// read Type to determine CSType
	csType, _ := pdf.Optional(c.Name(dict["Type"]))
	switch CoordinateSystemType(csType) {
	case CoordSysGeographic, "":
		cs.CSType = CoordSysGeographic
	case CoordSysProjected:
		cs.CSType = CoordSysProjected
	default:
		return nil, pdf.Errorf("unknown coordinate system type %q", csType)
	}

	// read EPSG (optional)
	epsg, err := pdf.Optional(c.Integer(dict["EPSG"]))
	if err != nil {
		return nil, err
	}
	cs.EPSG = int(epsg)

	// read WKT (optional)
	wkt, err := pdf.Optional(c.String(dict["WKT"]))
	if err != nil {
		return nil, err
	}
	cs.WKT = string(wkt)

	if cs.EPSG == 0 && cs.WKT == "" {
		return nil, pdf.Error("coordinate system has neither EPSG nor WKT")
	}

	cs.SingleUse = isDirect

	return cs, nil
}

// Embed converts the CoordinateSystem into a PDF object.
func (cs *CoordinateSystem) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "coordinate system dictionaries", pdf.V2_0); err != nil {
		return nil, err
	}

	// validate CSType
	switch cs.CSType {
	case CoordSysGeographic, CoordSysProjected:
		// ok
	default:
		return nil, pdf.Errorf("invalid coordinate system type %q", cs.CSType)
	}

	// at least one of EPSG or WKT must be set
	if cs.EPSG == 0 && cs.WKT == "" {
		return nil, pdf.Error("coordinate system must have EPSG or WKT")
	}

	dict := pdf.Dict{
		"Type": pdf.Name(cs.CSType),
	}

	if cs.EPSG != 0 {
		dict["EPSG"] = pdf.Integer(cs.EPSG)
	}

	if cs.WKT != "" {
		dict["WKT"] = pdf.String(cs.WKT)
	}

	if cs.SingleUse {
		return dict, nil
	}

	ref := e.Alloc()
	err := e.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}

	return ref, nil
}
