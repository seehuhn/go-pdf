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
	"slices"

	"seehuhn.de/go/pdf"
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
