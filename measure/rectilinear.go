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

// RectilinearMeasure represents a rectilinear coordinate system.
type RectilinearMeasure struct {
	// ScaleRatio expresses the scale ratio of the drawing.
	ScaleRatio string

	// XAxis specifies units for the x-axis.
	XAxis []*NumberFormat

	// YAxis specifies units for the y-axis.
	YAxis []*NumberFormat

	// Distance specifies units for distance measurements.
	Distance []*NumberFormat

	// Area specifies units for area measurements.
	Area []*NumberFormat

	// Angle specifies units for angle measurements (optional).
	Angle []*NumberFormat

	// Slope specifies units for slope measurements (optional).
	Slope []*NumberFormat

	// Origin specifies the origin of the measurement coordinate system.
	Origin [2]float64

	// CYX is the y-to-x axis conversion factor.
	// TODO(voss): Clarify the semantics when Y units match X units.
	// Zero means the value is not present in the PDF.
	CYX float64

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// MeasureType returns the type of measure dictionary.
func (rm *RectilinearMeasure) MeasureType() pdf.Name {
	return "RL"
}

// Embed converts the RectilinearMeasure into a PDF object.
func (rm *RectilinearMeasure) Embed(res *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	// Version check for PDF 1.6+
	if err := pdf.CheckVersion(res.Out, "measure dictionaries", pdf.V1_6); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{}

	// Optional Type field
	if res.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Measure")
	}

	// Subtype
	dict["Subtype"] = pdf.Name("RL")

	// Required fields
	dict["R"] = pdf.String(rm.ScaleRatio)

	// X axis
	xArray, err := embedNumberFormatArray(res, rm.XAxis)
	if err != nil {
		return nil, zero, err
	}
	dict["X"] = xArray

	// Y axis - optimize by omitting if pointer-equal to X
	yAxisOmitted := areNumberFormatArraysEqual(rm.YAxis, rm.XAxis)
	if !yAxisOmitted {
		yArray, err := embedNumberFormatArray(res, rm.YAxis)
		if err != nil {
			return nil, zero, err
		}
		dict["Y"] = yArray
	}

	// Distance
	dArray, err := embedNumberFormatArray(res, rm.Distance)
	if err != nil {
		return nil, zero, err
	}
	dict["D"] = dArray

	// Area
	aArray, err := embedNumberFormatArray(res, rm.Area)
	if err != nil {
		return nil, zero, err
	}
	dict["A"] = aArray

	// Optional fields
	if len(rm.Angle) > 0 {
		tArray, err := embedNumberFormatArray(res, rm.Angle)
		if err != nil {
			return nil, zero, err
		}
		dict["T"] = tArray
	}

	if len(rm.Slope) > 0 {
		sArray, err := embedNumberFormatArray(res, rm.Slope)
		if err != nil {
			return nil, zero, err
		}
		dict["S"] = sArray
	}

	// Origin - only write if not [0,0]
	if rm.Origin[0] != 0 || rm.Origin[1] != 0 {
		dict["O"] = pdf.Array{pdf.Number(rm.Origin[0]), pdf.Number(rm.Origin[1])}
	}

	// CYX - only write if non-zero AND Y axis is present
	if rm.CYX != 0 && !yAxisOmitted {
		dict["CYX"] = pdf.Number(rm.CYX)
	}

	if rm.SingleUse {
		return dict, zero, nil
	}

	ref := res.Out.Alloc()
	err = res.Out.Put(ref, dict)
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// embedNumberFormatArray embeds an array of NumberFormat objects.
func embedNumberFormatArray(res *pdf.ResourceManager, formats []*NumberFormat) (pdf.Array, error) {
	arr := make(pdf.Array, len(formats))
	for i, format := range formats {
		embedded, _, err := format.Embed(res)
		if err != nil {
			return nil, err
		}
		arr[i] = embedded
	}
	return arr, nil
}

// areNumberFormatArraysEqual checks if two arrays contain identical pointers.
func areNumberFormatArraysEqual(a, b []*NumberFormat) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] { // Pointer equality
			return false
		}
	}
	return true
}
