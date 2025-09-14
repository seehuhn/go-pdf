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

import (
	"math"

	"seehuhn.de/go/pdf"
)

func isFinite(x float64) bool {
	return !math.IsInf(x, 0) && !math.IsNaN(x)
}

// isPair checks if the given values x and y are finite.
func isPair(x, y float64) bool {
	return !math.IsInf(x, 0) && !math.IsInf(y, 0) && !math.IsNaN(x) && !math.IsNaN(y)
}

// isRange checks if the given values x and y are finite and satisfy x <= y.
func isRange(x, y float64) bool {
	return !math.IsInf(x, 0) && !math.IsInf(y, 0) && x <= y
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

// arrayFromFloats converts a slice of float64 to a PDF Array.
func arrayFromFloats(x []float64) pdf.Array {
	if x == nil {
		return nil
	}
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}

// arrayFromInts converts a slice of int to a PDF Array.
func arrayFromInts(x []int) pdf.Array {
	if x == nil {
		return nil
	}
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Integer(xi)
	}
	return res
}

// floatEpsilon is the tolerance for comparing floating point values.
const floatEpsilon = 1e-9

// floatSlicesEqual compares two float64 slices for equality with a given epsilon tolerance.
func floatSlicesEqual(a, b []float64, eps float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > eps {
			return false
		}
	}
	return true
}

// intSlicesEqual compares two int slices for equality.
func intSlicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Equal compares two PDF functions for equality.
func Equal(a, b pdf.Function) bool {
	if a == nil || b == nil {
		return a == b
	}

	if a.FunctionType() != b.FunctionType() {
		return false
	}

	switch fa := a.(type) {
	case *Type0:
		return fa.Equal(b.(*Type0))
	case *Type2:
		return fa.Equal(b.(*Type2))
	case *Type3:
		return fa.Equal(b.(*Type3))
	case *Type4:
		return fa.Equal(b.(*Type4))
	default:
		return false
	}
}
