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

package function

import "seehuhn.de/go/pdf"

func toPDF(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}

func fromPDF(r pdf.Getter, obj pdf.Object) ([]float64, error) {
	a, err := pdf.GetArray(r, obj)
	if err != nil {
		return nil, err
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