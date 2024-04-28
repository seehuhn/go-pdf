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

package widths

import (
	"seehuhn.de/go/float"
	"seehuhn.de/go/pdf"
)

// Info contains the FirstChar, LastChar and Widths entries of
// a PDF font dictionary, as well as the MissingWidth entry of the
// FontDescriptor dictionary.
type Info struct {
	FirstChar    pdf.Integer
	LastChar     pdf.Integer
	Widths       pdf.Array
	MissingWidth float64
}

// EncodeSimple encodes the glyph width information for a simple PDF font.
// The slice ww must have length 256 and is indexed by character code.
// Widths values are given in PDF glyph space units.
func EncodeSimple(ww []float64) *Info {
	// find FirstChar and LastChar
	cand := make(map[float64]bool)
	cand[ww[0]] = true
	cand[ww[255]] = true
	bestGain := 0
	FirstChar := 0
	LastChar := 255
	var MissingWidth float64
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
			MissingWidth = w
		}
	}

	Widths := make(pdf.Array, LastChar-FirstChar+1)
	for i := range Widths {
		w := ww[FirstChar+i]
		Widths[i] = pdf.Number(float.Round(w, 2))
	}

	return &Info{
		FirstChar:    pdf.Integer(FirstChar),
		LastChar:     pdf.Integer(LastChar),
		Widths:       Widths,
		MissingWidth: MissingWidth,
	}
}
