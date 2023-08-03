// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package font

import (
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/funit"
)

// This file contains helper functions used for embedding simple fonts.

type widthInfo struct {
	FirstChar    pdf.Integer
	LastChar     pdf.Integer
	Widths       pdf.Array
	MissingWidth pdf.Integer
}

func CompressWidths(ww []funit.Int16, unitsPerEm uint16) *widthInfo {
	q := 1000 / float64(unitsPerEm)

	// find FirstChar and LastChar
	cand := make(map[funit.Int16]int)
	cand[ww[0]] = 0
	cand[ww[255]] = 0
	bestGain := 0
	FirstChar := 0
	LastChar := 255
	var MissingWidth pdf.Integer
	for w := range cand {
		b := 255
		for b > 0 && ww[b] == w {
			b--
		}
		a := 0
		for a < b && ww[a] == w {
			a++
		}
		gain := (255 - b + a) * 4
		if w != 0 {
			gain -= 15
		}
		if gain > bestGain {
			bestGain = gain
			FirstChar = a
			LastChar = b
			MissingWidth = pdf.Integer(math.Round(w.AsFloat(q)))
		}
	}

	Widths := make(pdf.Array, LastChar-FirstChar+1)
	for i := range Widths {
		w := ww[FirstChar+i]
		Widths[i] = pdf.Integer(math.Round(w.AsFloat(q)))
	}

	return &widthInfo{
		FirstChar:    pdf.Integer(FirstChar),
		LastChar:     pdf.Integer(LastChar),
		Widths:       Widths,
		MissingWidth: MissingWidth,
	}
}
