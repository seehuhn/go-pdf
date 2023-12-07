// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package float

import (
	"math"
	"strconv"
)

func Format(x float64, digits int) string {
	signPart := ""
	if x < 0 {
		signPart = "-"
		x = -x
	}

	for i := 0; i < digits; i++ {
		x *= 10
	}
	xInt := int64(math.Round(x))
	if xInt == 0 {
		return "0"
	}
	s := strconv.FormatInt(xInt, 10)
	var intPart, fracPart string
	if len(s) > digits {
		intPart = s[:len(s)-digits]
		fracPart = s[len(s)-digits:]
	} else {
		intPart = "0"
		fracPart = s
		for len(fracPart) < digits {
			fracPart = "0" + fracPart
		}
	}
	for fracPart != "" && fracPart[len(fracPart)-1] == '0' {
		fracPart = fracPart[:len(fracPart)-1]
	}
	if fracPart != "" {
		fracPart = "." + fracPart
	}

	if signPart == "" && intPart == "0" && fracPart != "" {
		intPart = ""
	}
	return signPart + intPart + fracPart
}

func Round(x float64, digits int) float64 {
	s := Format(x, digits)
	y, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(err)
	}
	return y
}
