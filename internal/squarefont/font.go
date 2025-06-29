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
	UnderlinePosition  = -100
	UnderlineThickness = 50

	// Glyph widths in PDF glyph space units
	NotdefWidth = 500
	SpaceWidth  = 250
	SquareWidth = 500
)

func drawSquare(path interface {
	MoveTo(x, y float64)
	LineTo(x, y float64)
}, fm matrix.Matrix) {
	left := float64(SquareLeft / (1000 * fm[0]))
	right := float64(SquareRight / (1000 * fm[0]))
	bottom := float64(SquareBottom / (1000 * fm[3]))
	top := float64(SquareTop / (1000 * fm[3]))

	path.MoveTo(left, bottom)
	path.LineTo(right, bottom)
	path.LineTo(right, top)
	path.LineTo(left, top)
	path.LineTo(left, bottom)
}
