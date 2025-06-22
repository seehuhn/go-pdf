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

package function

import "seehuhn.de/go/pdf"

// arrayFromFloats converts a slice of float64 to a PDF Array.
func arrayFromFloats(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}

// arrayFromInts converts a slice of int to a PDF Array.
func arrayFromInts(x []int) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Integer(xi)
	}
	return res
}

// floatsFromPDF extracts a slice of float64 from a PDF Array.
func floatsFromPDF(r pdf.Getter, obj pdf.Object) ([]float64, error) {
	a, err := pdf.GetArray(r, obj)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, nil
	}

	res := make([]float64, len(a))
	for i, obj := range a {
		num, err := pdf.GetNumber(r, obj)
		if err != nil {
			return nil, err
		}
		res[i] = float64(num)
	}
	return res, nil
}

// intsFromPDF extracts a slice of int from a PDF Array.
func intsFromPDF(r pdf.Getter, obj pdf.Object) ([]int, error) {
	a, err := pdf.GetArray(r, obj)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, nil
	}

	res := make([]int, len(a))
	for i, obj := range a {
		num, err := pdf.GetInteger(r, obj)
		if err != nil {
			return nil, err
		}
		res[i] = int(num)
	}
	return res, nil
}

// clip clips a value to the given range [min, max].
func clip(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// interpolate performs linear interpolation.
func interpolate(x, xMin, xMax, yMin, yMax float64) float64 {
	if xMax <= xMin {
		return yMin
	}
	return yMin + (x-xMin)*(yMax-yMin)/(xMax-xMin)
}
