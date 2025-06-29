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
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf/font"
	pdftype1 "seehuhn.de/go/pdf/font/type1"
)

// makeType1_500 creates a Type1 font equivalent to 500 UPM.
func makeType1_500() font.Layouter {
	return makeType1Font(matrix.Matrix{0.002, 0, 0, 0.002, 0, 0})
}

// makeType1_1000 creates a Type1 font equivalent to 1000 UPM.
func makeType1_1000() font.Layouter {
	return makeType1Font(matrix.Matrix{0.001, 0, 0, 0.001, 0, 0})
}

// makeType1_2000 creates a Type1 font equivalent to 2000 UPM.
func makeType1_2000() font.Layouter {
	return makeType1Font(matrix.Matrix{0.0005, 0, 0, 0.0005, 0, 0})
}

// makeType1Asymmetric creates a Type1 font with asymmetric scaling.
func makeType1Asymmetric() font.Layouter {
	return makeType1Font(matrix.Matrix{0.002, 0, 0, 0.0005, 0, 0})
}

// makeType1Font creates a Type1 font with the specified FontMatrix.
func makeType1Font(fontMatrix matrix.Matrix) font.Layouter {
	hScale := 1 / 1000.0 / fontMatrix[0]
	vScale := 1 / 1000.0 / fontMatrix[3]
	sI10 := funit.Int16(math.Round(10 * vScale))

	encoding := make([]string, 256)
	for i := range encoding {
		switch i {
		case ' ':
			encoding[i] = "space"
		case 'A':
			encoding[i] = "A"
		default:
			encoding[i] = ".notdef"
		}
	}

	F := &type1.Font{
		FontInfo: &type1.FontInfo{
			FontName:           "SquareFont",
			FullName:           "Square Font",
			FamilyName:         "SquareFont",
			Weight:             "Regular",
			Version:            "1.0",
			FontMatrix:         fontMatrix,
			UnderlinePosition:  funit.Float64(UnderlinePosition * vScale),
			UnderlineThickness: funit.Float64(UnderlineThickness * vScale),
			ItalicAngle:        0,
		},
		Outlines: &type1.Outlines{
			Glyphs: map[string]*type1.Glyph{
				".notdef": createType1EmptyGlyph(NotdefWidth, fontMatrix),
				"space":   createType1EmptyGlyph(SpaceWidth, fontMatrix),
				"A":       createType1SquareGlyph(SquareWidth, fontMatrix),
			},
			Private: &type1.PrivateDict{
				BlueValues: []funit.Int16{-1 * sI10, 0, 99 * sI10, 10 * sI10},
				BlueScale:  0.039625,
				BlueShift:  7,
				BlueFuzz:   1,
				StdHW:      20 * vScale,
				StdVW:      20 * hScale,
			},
			Encoding: encoding,
		},
	}

	M := &afm.Metrics{
		Glyphs: map[string]*afm.GlyphInfo{
			".notdef": {WidthX: NotdefWidth},
			"space":   {WidthX: SpaceWidth},
			"A": {
				WidthX: SquareWidth,
				BBox: rect.Rect{
					LLx: SquareLeft,
					LLy: SquareBottom,
					URx: SquareRight,
					URy: SquareTop,
				},
			},
		},
		Encoding:           encoding,
		FontName:           "SquareFont",
		FullName:           "SquareFont",
		Version:            "1.0",
		Ascent:             Ascent,
		Descent:            Descent,
		UnderlinePosition:  UnderlinePosition,
		UnderlineThickness: UnderlineThickness,
	}

	// Convert to PDF Type1 font (no AFM metrics to avoid consistency issues)
	instance, err := pdftype1.New(F, M)
	if err != nil {
		panic(err)
	}
	return instance
}

// createType1EmptyGlyph creates an empty Type1 glyph.
func createType1EmptyGlyph(width float64, fm matrix.Matrix) *type1.Glyph {
	return &type1.Glyph{WidthX: width / (1000 * fm[0])}
}

func createType1SquareGlyph(width float64, fm matrix.Matrix) *type1.Glyph {
	glyph := &type1.Glyph{
		WidthX: width / (1000 * fm[0]),
	}
	drawSquare(glyph, fm)
	glyph.ClosePath()

	return glyph
}
