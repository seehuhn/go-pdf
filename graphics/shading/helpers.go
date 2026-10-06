// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package shading

import (
	"fmt"
	"math"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/sample"
)

func toPDF(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}

func isValues(x []float64, y ...float64) bool {
	return slices.Equal(x, y)
}

// domainContains checks if functionDomain contains shadingDomain.
// Both domains are in format [min0, max0, min1, max1, ...] where each pair
// represents the valid range for one input variable.
func domainContains(functionDomain, shadingDomain []float64) bool {
	if len(shadingDomain)%2 != 0 || len(functionDomain)%2 != 0 {
		return false
	}
	// Function and shading must have same number of input dimensions
	if len(functionDomain) != len(shadingDomain) {
		return false
	}

	// Check each dimension pair
	for i := 0; i < len(shadingDomain); i += 2 {
		shadingMin := shadingDomain[i]
		shadingMax := shadingDomain[i+1]
		functionMin := functionDomain[i]
		functionMax := functionDomain[i+1]

		if functionMin > shadingMin || functionMax < shadingMax {
			return false
		}
	}
	return true
}

// decodeMaps returns the maps between codes and values for the X and Y
// coordinates and for each colour value of a mesh shading.
func decodeMaps(decode []float64, bitsPerCoordinate, bitsPerComponent int) (x, y sample.Map, c []sample.Map) {
	x = sample.Map{Bits: bitsPerCoordinate, Min: decode[0], Max: decode[1]}
	y = sample.Map{Bits: bitsPerCoordinate, Min: decode[2], Max: decode[3]}
	for i := 4; i+1 < len(decode); i += 2 {
		c = append(c, sample.Map{Bits: bitsPerComponent, Min: decode[i], Max: decode[i+1]})
	}
	return x, y, c
}

// allFinite reports whether none of the values is infinite or NaN.
func allFinite(xs ...float64) bool {
	for _, x := range xs {
		if math.IsInf(x, 0) || math.IsNaN(x) {
			return false
		}
	}
	return true
}

// checkFunction checks that the function of a shading has the given number
// of inputs and one output per colour component.
func checkFunction(f pdf.Function, inputs, channels int) error {
	m, n := f.Shape()
	if m != inputs {
		return fmt.Errorf("function inputs (%d) must be %d", m, inputs)
	}
	if n != channels {
		return fmt.Errorf("function outputs (%d) must match color space channels (%d)", n, channels)
	}
	return nil
}

// checkMeshDomain checks that the domain of a mesh shading's function
// contains the range of the parametric value given in the Decode array.
func checkMeshDomain(f pdf.Function, decode []float64) error {
	t := []float64{min(decode[4], decode[5]), max(decode[4], decode[5])}
	if d := f.GetDomain(); !domainContains(d, t) {
		return fmt.Errorf("function domain %v must contain decode range %v", d, t)
	}
	return nil
}

// inDecodeRange reports whether x lies in the interval given by the i-th
// pair of the Decode array.  NaN lies in no interval.
func inDecodeRange(decode []float64, i int, x float64) bool {
	a, b := decode[2*i], decode[2*i+1]
	return min(a, b) <= x && x <= max(a, b)
}
