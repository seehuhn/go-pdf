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

package pdf

import (
	"math"
)

// Round rounds the float64 value x to the specified number of digits. This
// should be used for all coordinates and dimensions before they are written to
// a PDF file.
func Round(x float64, digits int) float64 {
	// Thanks, Alexander Ertli:
	// https://groups.google.com/g/golang-nuts/c/UfynqG8K1n0/m/PqyZfANPAgAJ
	scale := math.Pow10(digits)
	return math.Round(x*scale) / scale
}

// func Round(x float64, digits int) float64 {
// 	if digits <= 0 {
// 		pow := math.Pow10(-digits)
// 		return math.Round(x/pow) * pow
// 	}
//
// 	format := "%." + strconv.Itoa(digits) + "f"
// 	s := fmt.Sprintf(format, x)
// 	result, _ := strconv.ParseFloat(s, 64)
//
// 	return result
// }
