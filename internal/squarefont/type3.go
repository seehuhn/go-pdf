// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package squarefont

import (
	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

// makeType3_500 creates a Type 3 font equivalent to 500 UPM.
func makeType3_500() font.Layouter {
	return makeType3Font(matrix.Matrix{0.002, 0, 0, 0.002, 0, 0})
}

// makeType3_1000 creates a Type 3 font equivalent to 1000 UPM.
func makeType3_1000() font.Layouter {
	return makeType3Font(matrix.Matrix{0.001, 0, 0, 0.001, 0, 0})
}

// makeType3_2000 creates a Type 3 font equivalent to 2000 UPM.
func makeType3_2000() font.Layouter {
	return makeType3Font(matrix.Matrix{0.0005, 0, 0, 0.0005, 0, 0})
}

// makeType3Asymmetric creates a Type 3 font with asymmetric scaling.
func makeType3Asymmetric() font.Layouter {
	return makeType3Font(matrix.Matrix{0.002, 0, 0, 0.0005, 0, 0})
}

// makeType3Font creates a Type 3 font with the specified FontMatrix.
func makeType3Font(fontMatrix matrix.Matrix) font.Layouter {
	hScale := 1.0 / 1000.0 / fontMatrix[0]
	vScale := 1.0 / 1000.0 / fontMatrix[3]

	F := &type3.Font{
		Glyphs: []*type3.Glyph{
			// glyph 0 replaces .notdef and must not carry a name
			{Content: emptyType3Glyph(NotdefWidth * hScale)},
			{Name: "space", Content: emptyType3Glyph(SpaceWidth * hScale)},
			{Name: "A", Content: squareType3Glyph(SquareWidth*hScale, fontMatrix)},
		},
		FontMatrix:         fontMatrix,
		PostScriptName:     "SquareFont",
		Ascent:             Ascent * vScale,
		Descent:            Descent * vScale,
		Leading:            Leading * vScale,
		CapHeight:          CapHeight * vScale,
		XHeight:            XHeight * vScale,
		UnderlinePosition:  UnderlinePosition * vScale,
		UnderlineThickness: UnderlineThickness * vScale,
	}
	return font.Must(F.New())
}

// emptyType3Glyph creates a Type 3 glyph which draws nothing.
func emptyType3Glyph(width float64) content.Stream {
	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3UncoloredGlyph(width, 0, 0, 0, 0, 0)
	return builder.Must(b.Harvest())
}

// squareType3Glyph creates a Type 3 glyph showing the square.
func squareType3Glyph(width float64, fm matrix.Matrix) content.Stream {
	box := squareBox(fm)

	b := builder.New(content.Glyph, nil, pdf.V2_0)
	b.Type3UncoloredGlyph(width, 0, box.LLx, box.LLy, box.URx, box.URy)
	drawSquare(b, fm)
	b.ClosePath()
	b.Fill()
	return builder.Must(b.Harvest())
}
