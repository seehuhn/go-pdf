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

// Extract extracts a PDF Measure Dictionary from a PDF file.
func Extract(r pdf.Getter, obj pdf.Object) (Measure, error) {
	dict, err := pdf.GetDict(r, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing measure dictionary")
	}

	// Get subtype with default "RL"
	subtype, _ := pdf.Optional(pdf.GetName(r, dict["Subtype"]))
	if subtype == "" {
		subtype = "RL"
	}

	switch subtype {
	case "RL":
		return extractRectilinearMeasure(r, dict)
	case "GEO":
		return extractGeospatialMeasure(r, dict)
	default:
		// Unknown subtype - default to RL for permissive reading
		return extractRectilinearMeasure(r, dict)
	}
}

// extractRectilinearMeasure extracts a RectilinearMeasure from a PDF dictionary.
func extractRectilinearMeasure(r pdf.Getter, dict pdf.Dict) (*RectilinearMeasure, error) {
	rm := &RectilinearMeasure{}

	// Extract required fields
	scaleRatio, err := pdf.GetString(r, dict["R"])
	if err != nil {
		return nil, err
	}
	rm.ScaleRatio = string(scaleRatio)

	// Extract X axis
	xArray, err := pdf.GetArray(r, dict["X"])
	if err != nil {
		return nil, err
	}
	rm.XAxis, err = extractNumberFormatArray(r, xArray)
	if err != nil {
		return nil, err
	}

	// Extract Y axis - if missing, copy from X
	if dict["Y"] != nil {
		yArray, err := pdf.GetArray(r, dict["Y"])
		if err != nil {
			return nil, err
		}
		rm.YAxis, err = extractNumberFormatArray(r, yArray)
		if err != nil {
			return nil, err
		}
	} else {
		// Y missing - copy from X
		rm.YAxis = rm.XAxis
		// When Y is copied from X, set CYX = 1.0 if CYX not present
		if dict["CYX"] == nil {
			rm.CYX = 1.0
		}
	}

	// Extract Distance
	dArray, err := pdf.GetArray(r, dict["D"])
	if err != nil {
		return nil, err
	}
	rm.Distance, err = extractNumberFormatArray(r, dArray)
	if err != nil {
		return nil, err
	}

	// Extract Area
	aArray, err := pdf.GetArray(r, dict["A"])
	if err != nil {
		return nil, err
	}
	rm.Area, err = extractNumberFormatArray(r, aArray)
	if err != nil {
		return nil, err
	}

	// Extract optional fields
	if dict["T"] != nil {
		tArray, err := pdf.Optional(pdf.GetArray(r, dict["T"]))
		if err != nil {
			return nil, err
		} else if tArray != nil {
			rm.Angle, err = extractNumberFormatArray(r, tArray)
			if err != nil {
				return nil, err
			}
		}
	}

	if dict["S"] != nil {
		sArray, err := pdf.Optional(pdf.GetArray(r, dict["S"]))
		if err != nil {
			return nil, err
		} else if sArray != nil {
			rm.Slope, err = extractNumberFormatArray(r, sArray)
			if err != nil {
				return nil, err
			}
		}
	}

	// Extract Origin - default is [0,0]
	if dict["O"] != nil {
		oArray, err := pdf.Optional(pdf.GetArray(r, dict["O"]))
		if err != nil {
			return nil, err
		} else if oArray != nil && len(oArray) >= 2 {
			origin0, err := pdf.GetNumber(r, oArray[0])
			if err != nil {
				return nil, err
			}
			origin1, err := pdf.GetNumber(r, oArray[1])
			if err != nil {
				return nil, err
			}
			rm.Origin = [2]float64{float64(origin0), float64(origin1)}
		}
	}

	// Extract CYX if present (and Y was present)
	if dict["Y"] != nil && dict["CYX"] != nil {
		cyx, err := pdf.Optional(pdf.GetNumber(r, dict["CYX"]))
		if err != nil {
			return nil, err
		}
		rm.CYX = float64(cyx)
	}

	return rm, nil
}

// extractNumberFormatArray extracts an array of NumberFormat objects.
func extractNumberFormatArray(r pdf.Getter, arr pdf.Array) ([]*NumberFormat, error) {
	formats := make([]*NumberFormat, len(arr))
	for i, obj := range arr {
		nf, err := ExtractNumberFormat(r, obj)
		if err != nil {
			return nil, err
		}
		formats[i] = nf
	}
	return formats, nil
}

// extractGeospatialMeasure is a placeholder for geospatial measures.
func extractGeospatialMeasure(r pdf.Getter, dict pdf.Dict) (Measure, error) {
	panic("geospatial measures not yet implemented")
}
