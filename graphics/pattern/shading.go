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

package pattern

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/matrix"
)

// NewShadingPattern creates a new shading pattern.
func NewShadingPattern(w pdf.Putter, shading *graphics.Shading, M matrix.Matrix, extGState *graphics.ExtGState, singleUse bool, defaultName pdf.Name) (color.Color, error) {
	dict := pdf.Dict{
		"PatternType": pdf.Integer(2),
		"Shading":     shading.PDFObject(),
	}
	if M != matrix.Identity && M != matrix.Zero {
		dict["Matrix"] = toPDF(M[:])
	}
	if extGState != nil {
		dict["ExtGState"] = extGState.PDFObject()
	}

	var data pdf.Object = dict
	if singleUse {
		ref := w.Alloc()
		err := w.Put(ref, dict)
		if err != nil {
			return nil, err
		}
		data = ref
	}

	pat := &color.PatternColored{
		Res: pdf.Res{
			DefName: defaultName,
			Data:    data,
		},
	}
	return pat, nil
}

func toPDF(x []float64) pdf.Array {
	res := make(pdf.Array, len(x))
	for i, xi := range x {
		res[i] = pdf.Number(xi)
	}
	return res
}
