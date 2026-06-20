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
	"fmt"

	"seehuhn.de/go/pdf"
)

// Extract extracts a PDF Measure Dictionary from a PDF file.
func Extract(c pdf.Cursor, obj pdf.Object, isDirect bool) (Measure, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing measure dictionary")
	}

	// Get subtype with default "RL"
	subtype, _ := pdf.Optional(c.Name(dict["Subtype"]))
	if subtype == "" {
		subtype = "RL"
	}

	switch subtype {
	case "RL":
		return extractRectilinearMeasure(c, dict, isDirect)
	case "GEO":
		return extractGeospatialMeasure(c, dict, isDirect)
	default:
		// Unknown subtype - default to RL for permissive reading
		return extractRectilinearMeasure(c, dict, isDirect)
	}
}

// extractRectilinearMeasure extracts a RectilinearMeasure from a PDF dictionary.
func extractRectilinearMeasure(c pdf.Cursor, dict pdf.Dict, isDirect bool) (*RectilinearMeasure, error) {
	singleUse := isDirect

	rm := &RectilinearMeasure{}

	// Extract X axis first (needed for autogenerating ScaleRatio if missing)
	xArray, err := c.Array(dict["X"])
	if err != nil {
		return nil, err
	}
	rm.XAxis, err = extractNumberFormatArray(c, xArray)
	if err != nil {
		return nil, err
	}
	if len(rm.XAxis) == 0 {
		rm.XAxis = []*NumberFormat{{Unit: "pt", ConversionFactor: 1, Precision: 100, DecimalSeparator: "."}}
	}

	// Extract Y axis - if missing, leave as nil
	if dict["Y"] != nil {
		yArray, err := c.Array(dict["Y"])
		if err != nil {
			return nil, err
		}
		rm.YAxis, err = extractNumberFormatArray(c, yArray)
		if err != nil {
			return nil, err
		}
	}
	// Note: YAxis remains nil if not present in PDF

	// Extract ScaleRatio - if missing or empty, autogenerate from X/Y arrays
	scaleRatio, _ := pdf.Optional(c.String(dict["R"]))
	if string(scaleRatio) == "" {
		// autogenerate scale ratio from X (and Y if different)
		if len(rm.XAxis) > 0 {
			xFactor := rm.XAxis[0].ConversionFactor
			xUnit := rm.XAxis[0].Unit

			if len(rm.YAxis) > 0 &&
				(rm.YAxis[0].ConversionFactor != xFactor || rm.YAxis[0].Unit != xUnit) {
				// X and Y differ
				yFactor := rm.YAxis[0].ConversionFactor
				yUnit := rm.YAxis[0].Unit
				scaleRatio = pdf.String(fmt.Sprintf("in X 1 pt = %g %s, in Y 1 pt = %g %s",
					xFactor, xUnit, yFactor, yUnit))
			} else {
				// X and Y same (or Y not present)
				scaleRatio = pdf.String(fmt.Sprintf("1 pt = %g %s", xFactor, xUnit))
			}
		} else {
			// fallback if XAxis is empty or malformed
			scaleRatio = pdf.String("1:1")
		}
	}
	rm.ScaleRatio = string(scaleRatio)

	// Extract Distance
	dArray, err := c.Array(dict["D"])
	if err != nil {
		return nil, err
	}
	rm.Distance, err = extractNumberFormatArray(c, dArray)
	if err != nil {
		return nil, err
	}
	if len(rm.Distance) == 0 {
		rm.Distance = []*NumberFormat{{Unit: "pt", ConversionFactor: 1, Precision: 100, DecimalSeparator: "."}}
	}

	// Extract Area
	aArray, err := c.Array(dict["A"])
	if err != nil {
		return nil, err
	}
	rm.Area, err = extractNumberFormatArray(c, aArray)
	if err != nil {
		return nil, err
	}
	if len(rm.Area) == 0 {
		rm.Area = []*NumberFormat{{Unit: "pt²", ConversionFactor: 1, Precision: 100, DecimalSeparator: "."}}
	}

	// Extract optional fields
	if dict["T"] != nil {
		tArray, err := pdf.Optional(c.Array(dict["T"]))
		if err != nil {
			return nil, err
		} else if tArray != nil {
			rm.Angle, err = extractNumberFormatArray(c, tArray)
			if err != nil {
				return nil, err
			}
		}
	}

	if dict["S"] != nil {
		sArray, err := pdf.Optional(c.Array(dict["S"]))
		if err != nil {
			return nil, err
		} else if sArray != nil {
			rm.Slope, err = extractNumberFormatArray(c, sArray)
			if err != nil {
				return nil, err
			}
		}
	}

	// Extract Origin - default is [0,0]
	if dict["O"] != nil {
		oArray, err := pdf.Optional(c.Array(dict["O"]))
		if err != nil {
			return nil, err
		} else if len(oArray) >= 2 {
			origin0, err := c.Number(oArray[0])
			if err != nil {
				return nil, err
			}
			origin1, err := c.Number(oArray[1])
			if err != nil {
				return nil, err
			}
			rm.Origin = [2]float64{origin0, origin1}
		}
	}

	// Extract CYX if present (and Y was present)
	if dict["Y"] != nil && dict["CYX"] != nil {
		cyx, err := pdf.Optional(c.Number(dict["CYX"]))
		if err != nil {
			return nil, err
		}
		rm.CYX = cyx
	}

	rm.SingleUse = singleUse

	return rm, nil
}

// extractNumberFormatArray extracts an array of NumberFormat objects.
func extractNumberFormatArray(c pdf.Cursor, arr pdf.Array) ([]*NumberFormat, error) {
	formats := make([]*NumberFormat, len(arr))
	for i, obj := range arr {
		nf, err := pdf.Decode(c, obj, ExtractNumberFormat)
		if err != nil {
			return nil, err
		}
		formats[i] = nf
	}
	return formats, nil
}

// extractGeospatialMeasure extracts a GeospatialMeasure from a PDF dictionary.
func extractGeospatialMeasure(c pdf.Cursor, dict pdf.Dict, isDirect bool) (Measure, error) {
	gm := &GeospatialMeasure{}

	// GCS (required)
	gcs, err := pdf.Decode(c, dict["GCS"], ExtractCoordinateSystem)
	if err != nil {
		return nil, err
	}
	gm.GCS = gcs

	// DCS (optional)
	dcs, err := pdf.DecodeOptional(c, dict["DCS"], ExtractCoordinateSystem)
	if err != nil {
		return nil, err
	}
	gm.DCS = dcs

	// GPTS (required)
	gpts, err := c.FloatArray(dict["GPTS"])
	if err != nil {
		return nil, err
	}
	// truncate to even length (coordinate pairs)
	gpts = gpts[:len(gpts)&^1]
	if len(gpts) == 0 {
		return nil, pdf.Error("missing required GPTS")
	}
	gm.GPTS = gpts

	// LPTS (optional)
	if dict["LPTS"] != nil {
		lpts, err := c.FloatArray(dict["LPTS"])
		if err != nil {
			return nil, err
		}
		// truncate to even length and match GPTS length
		lpts = lpts[:len(lpts)&^1]
		if len(lpts) != len(gpts) {
			lpts = nil
		}
		gm.LPTS = lpts
	}

	// Bounds (optional)
	if dict["Bounds"] != nil {
		bounds, err := c.FloatArray(dict["Bounds"])
		if err != nil {
			return nil, err
		}
		// truncate to even length
		bounds = bounds[:len(bounds)&^1]
		if len(bounds) > 0 {
			gm.Bounds = bounds
		}
	}

	// PDU (optional, all three must be present)
	if dict["PDU"] != nil {
		pduArray, err := pdf.Optional(c.Array(dict["PDU"]))
		if err != nil {
			return nil, err
		}
		if len(pduArray) >= 3 {
			var pdu [3]pdf.Name
			valid := true
			for i := range 3 {
				name, err := pdf.Optional(c.Name(pduArray[i]))
				if err != nil {
					return nil, err
				}
				if name == "" {
					valid = false
					break
				}
				pdu[i] = name
			}
			if valid {
				gm.PDU = pdu
			}
		}
	}

	// PCSM (optional, must be exactly 12 elements)
	if dict["PCSM"] != nil {
		pcsm, err := c.FloatArray(dict["PCSM"])
		if err != nil {
			return nil, err
		}
		if len(pcsm) == 12 {
			gm.PCSM = pcsm
		}
	}

	gm.SingleUse = isDirect

	return gm, nil
}
