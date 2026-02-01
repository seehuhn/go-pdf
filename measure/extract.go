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
func Extract(x *pdf.Extractor, obj pdf.Object) (Measure, error) {
	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing measure dictionary")
	}

	// Get subtype with default "RL"
	subtype, _ := pdf.Optional(x.GetName(dict["Subtype"]))
	if subtype == "" {
		subtype = "RL"
	}

	switch subtype {
	case "RL":
		return extractRectilinearMeasure(x, dict)
	case "GEO":
		return extractGeospatialMeasure(x, dict)
	default:
		// Unknown subtype - default to RL for permissive reading
		return extractRectilinearMeasure(x, dict)
	}
}

// extractRectilinearMeasure extracts a RectilinearMeasure from a PDF dictionary.
func extractRectilinearMeasure(x *pdf.Extractor, dict pdf.Dict) (*RectilinearMeasure, error) {
	rm := &RectilinearMeasure{}

	// Extract X axis first (needed for autogenerating ScaleRatio if missing)
	xArray, err := x.GetArray(dict["X"])
	if err != nil {
		return nil, err
	}
	rm.XAxis, err = extractNumberFormatArray(x, xArray)
	if err != nil {
		return nil, err
	}
	if len(rm.XAxis) == 0 {
		rm.XAxis = []*NumberFormat{{Unit: "pt", ConversionFactor: 1, Precision: 100}}
	}

	// Extract Y axis - if missing, leave as nil
	if dict["Y"] != nil {
		yArray, err := x.GetArray(dict["Y"])
		if err != nil {
			return nil, err
		}
		rm.YAxis, err = extractNumberFormatArray(x, yArray)
		if err != nil {
			return nil, err
		}
	}
	// Note: YAxis remains nil if not present in PDF

	// Extract ScaleRatio - if missing or empty, autogenerate from X/Y arrays
	scaleRatio, _ := pdf.Optional(x.GetString(dict["R"]))
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
	dArray, err := x.GetArray(dict["D"])
	if err != nil {
		return nil, err
	}
	rm.Distance, err = extractNumberFormatArray(x, dArray)
	if err != nil {
		return nil, err
	}
	if len(rm.Distance) == 0 {
		rm.Distance = []*NumberFormat{{Unit: "pt", ConversionFactor: 1, Precision: 100}}
	}

	// Extract Area
	aArray, err := x.GetArray(dict["A"])
	if err != nil {
		return nil, err
	}
	rm.Area, err = extractNumberFormatArray(x, aArray)
	if err != nil {
		return nil, err
	}
	if len(rm.Area) == 0 {
		rm.Area = []*NumberFormat{{Unit: "pt²", ConversionFactor: 1, Precision: 100}}
	}

	// Extract optional fields
	if dict["T"] != nil {
		tArray, err := pdf.Optional(x.GetArray(dict["T"]))
		if err != nil {
			return nil, err
		} else if tArray != nil {
			rm.Angle, err = extractNumberFormatArray(x, tArray)
			if err != nil {
				return nil, err
			}
		}
	}

	if dict["S"] != nil {
		sArray, err := pdf.Optional(x.GetArray(dict["S"]))
		if err != nil {
			return nil, err
		} else if sArray != nil {
			rm.Slope, err = extractNumberFormatArray(x, sArray)
			if err != nil {
				return nil, err
			}
		}
	}

	// Extract Origin - default is [0,0]
	if dict["O"] != nil {
		oArray, err := pdf.Optional(x.GetArray(dict["O"]))
		if err != nil {
			return nil, err
		} else if len(oArray) >= 2 {
			origin0, err := x.GetNumber(oArray[0])
			if err != nil {
				return nil, err
			}
			origin1, err := x.GetNumber(oArray[1])
			if err != nil {
				return nil, err
			}
			rm.Origin = [2]float64{float64(origin0), float64(origin1)}
		}
	}

	// Extract CYX if present (and Y was present)
	if dict["Y"] != nil && dict["CYX"] != nil {
		cyx, err := pdf.Optional(x.GetNumber(dict["CYX"]))
		if err != nil {
			return nil, err
		}
		rm.CYX = float64(cyx)
	}

	return rm, nil
}

// extractNumberFormatArray extracts an array of NumberFormat objects.
func extractNumberFormatArray(x *pdf.Extractor, arr pdf.Array) ([]*NumberFormat, error) {
	formats := make([]*NumberFormat, len(arr))
	for i, obj := range arr {
		nf, err := ExtractNumberFormat(x, obj)
		if err != nil {
			return nil, err
		}
		formats[i] = nf
	}
	return formats, nil
}

// extractGeospatialMeasure is a placeholder for geospatial measures.
func extractGeospatialMeasure(x *pdf.Extractor, dict pdf.Dict) (Measure, error) {
	panic("geospatial measures not yet implemented")
}
