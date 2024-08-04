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

package color

import (
	"math"

	"seehuhn.de/go/pdf"
)

func getNames(r pdf.Getter, obj pdf.Object) (pdf.Array, error) {
	a, err := pdf.GetArray(r, obj)
	if err != nil {
		return nil, err
	}
	res := make(pdf.Array, len(a))
	for i, obj := range a {
		name, err := pdf.GetName(r, obj)
		if err != nil {
			return nil, err
		}
		res[i] = name
	}
	return res, nil
}

func toPDF(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}

func isConst(x []float64, value float64) bool {
	for _, xi := range x {
		if math.Abs(xi-value) >= ε {
			return false
		}
	}
	return true
}

func isZero(x []float64) bool {
	return isConst(x, 0)
}

func isValues(x []float64, y ...float64) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if math.Abs(x[i]-y[i]) >= ε {
			return false
		}
	}
	return true
}

func isValidWhitePoint(x []float64) bool {
	return len(x) == 3 &&
		x[0] > 0 &&
		math.Abs(x[1]-1) <= ε &&
		x[2] > 0
}

func isValidBlackPoint(x []float64) bool {
	return len(x) == 3 && x[0] >= 0 && x[1] >= 0 && x[2] >= 0
}

const ε = 1e-6
