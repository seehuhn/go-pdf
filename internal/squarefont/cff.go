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
	"math"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf/font"
	pdfcff "seehuhn.de/go/pdf/font/cff"
)

// makeCFF500 creates a CFF font equivalent to 500 UPM.
func makeCFF500() font.Layouter {
	return makeCFFFont(matrix.Matrix{0.002, 0, 0, 0.002, 0, 0})
}

// makeCFF1000 creates a CFF font equivalent to 1000 UPM.
func makeCFF1000() font.Layouter {
	return makeCFFFont(matrix.Matrix{0.001, 0, 0, 0.001, 0, 0})
}

// makeCFF2000 creates a CFF font equivalent to 2000 UPM.
func makeCFF2000() font.Layouter {
	return makeCFFFont(matrix.Matrix{0.0005, 0, 0, 0.0005, 0, 0})
}

// makeCFFAsymmetric creates a CFF font with asymmetric scaling.
func makeCFFAsymmetric() font.Layouter {
	return makeCFFFont(matrix.Matrix{0.002, 0, 0, 0.0005, 0, 0})
}

// makeCFFFont creates a CFF font with the specified FontMatrix.
func makeCFFFont(fontMatrix matrix.Matrix) font.Layouter {
	hScale := 1.0 / 1000.0 / fontMatrix[0]
	vScale := 1.0 / 1000.0 / fontMatrix[3]
	sI10 := funit.Int16(math.Round(10 * vScale))
	outlines := &cff.Outlines{
		Glyphs: []*cff.Glyph{
			createCFFEmptyGlyph(".notdef", NotdefWidth, fontMatrix),
			createCFFEmptyGlyph("space", SpaceWidth, fontMatrix),
			createCFFSquareGlyph("A", SquareWidth, fontMatrix),
		},
		Private: []*type1.PrivateDict{
			{
				BlueValues: []funit.Int16{-1 * sI10, 0, 99 * sI10, 10 * sI10},
				BlueScale:  0.039625,
				BlueShift:  7,
				BlueFuzz:   1,
				StdHW:      20 * vScale,
				StdVW:      20 * hScale,
			},
		},
		FDSelect: func(i glyph.ID) int { return 0 },
		Encoding: []glyph.ID{0, 1, 2}, // Simple encoding for our 3 glyphs
	}

	cmapSubtable := cmap.Format4{' ': 1, 'A': 2}
	cmapTable := cmap.Table{
		{PlatformID: 0, EncodingID: 3}: cmapSubtable.Encode(0),
	}

	font := &sfnt.Font{
		FamilyName:         "SquareFont",
		Ascent:             funit.Int16(Ascent * vScale),
		Descent:            funit.Int16(Descent * vScale),
		LineGap:            funit.Int16((Leading - Ascent + Descent) * vScale),
		UnderlinePosition:  funit.Float64(UnderlinePosition * vScale),
		UnderlineThickness: funit.Float64(UnderlineThickness * vScale),
		CapHeight:          funit.Int16(600 * vScale),
		XHeight:            funit.Int16(400 * vScale),
		Outlines:           outlines,
		Width:              os2.WidthNormal,
		Weight:             os2.WeightMedium,
		IsRegular:          true,
		PermUse:            os2.PermInstall,
		UnitsPerEm:         uint16(math.Round(1 / fontMatrix[3])),
		FontMatrix:         fontMatrix,
		CMapTable:          cmapTable,
	}

	instance, err := pdfcff.NewSimple(font, nil)
	if err != nil {
		panic(err)
	}

	return instance
}

// createCFFEmptyGlyph creates an empty CFF glyph.
func createCFFEmptyGlyph(name string, width float64, fm matrix.Matrix) *cff.Glyph {
	return &cff.Glyph{
		Name:  name,
		Width: width / (1000 * fm[0]),
	}
}

// createCFFSquareGlyph creates a square CFF glyph.
// The coordinates are in design units and will be scaled by FontMatrix.
func createCFFSquareGlyph(name string, width float64, fm matrix.Matrix) *cff.Glyph {
	glyph := cff.NewGlyph(name, width/(1000*fm[0]))
	drawSquare(glyph, fm)
	return glyph
}
