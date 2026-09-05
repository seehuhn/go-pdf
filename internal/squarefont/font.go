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

package squarefont

import (
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/pdf/font"
)

// Sample represents a test font with a descriptive label and constructor function.
type Sample struct {
	Label    string
	MakeFont func() font.Layouter
}

// All contains all available test fonts in the collection.
var All []*Sample = all
var all = []*Sample{
	{"TrueType-500", makeTrueType500},
	{"TrueType-1000", makeTrueType1000},
	{"TrueType-2000", makeTrueType2000},
	{"CFF-500", makeCFF500},
	{"CFF-1000", makeCFF1000},
	{"CFF-2000", makeCFF2000},
	{"CFF-Asymmetric", makeCFFAsymmetric},
	{"Type1-500", makeType1_500},
	{"Type1-1000", makeType1_1000},
	{"Type1-2000", makeType1_2000},
	{"Type1-Asymmetric", makeType1Asymmetric},
	{"Type3-500", makeType3_500},
	{"Type3-1000", makeType3_1000},
	{"Type3-2000", makeType3_2000},
	{"Type3-Asymmetric", makeType3Asymmetric},
}

// Standard PDF glyph space values that all fonts should produce.
const (
	// Square glyph bounding box coordinates (400×400 square)
	SquareLeft   = 100
	SquareRight  = 500
	SquareBottom = 200
	SquareTop    = 600

	// Font metrics in PDF glyph space units
	Ascent             = 800
	Descent            = -200
	Leading            = 1200
	CapHeight          = 600
	XHeight            = 400
	UnderlinePosition  = -100
	UnderlineThickness = 50
	StemWidth          = 20 // CFF and Type 1 fonts only

	// Glyph widths in PDF glyph space units
	NotdefWidth = 500
	SpaceWidth  = 250
	SquareWidth = 500
)

// squareBox returns the bounding box of the square glyph, in the glyph space
// implied by the given font matrix.
func squareBox(fm matrix.Matrix) rect.Rect {
	return rect.Rect{
		LLx: SquareLeft / (1000 * fm[0]),
		LLy: SquareBottom / (1000 * fm[3]),
		URx: SquareRight / (1000 * fm[0]),
		URy: SquareTop / (1000 * fm[3]),
	}
}

func drawSquare(path interface {
	MoveTo(x, y float64)
	LineTo(x, y float64)
}, fm matrix.Matrix) {
	box := squareBox(fm)

	path.MoveTo(box.LLx, box.LLy)
	path.LineTo(box.URx, box.LLy)
	path.LineTo(box.URx, box.URy)
	path.LineTo(box.LLx, box.URy)
	path.LineTo(box.LLx, box.LLy)
}
