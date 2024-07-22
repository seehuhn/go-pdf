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

package dummyfont

import (
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt"
	sfntcff "seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf/font"
	pdfcff "seehuhn.de/go/pdf/font/cff"
)

func Must() font.Font {
	F, err := New()
	if err != nil {
		panic(err)
	}
	return F
}

// New creates a simple font for testing purposes.
// The only glyphs available are the space and the capital letter A.
// The font is embedded as a simple CFF font.
func New() (font.Font, error) {
	encoding := make([]glyph.ID, 256)
	encoding[' '] = 1
	encoding['A'] = 2

	in := &sfntcff.Outlines{
		Private:  []*type1.PrivateDict{{}},
		FDSelect: func(gi glyph.ID) int { return 0 },
		Encoding: encoding,
	}

	g := sfntcff.NewGlyph(".notdef", 500)
	in.Glyphs = append(in.Glyphs, g)

	g = sfntcff.NewGlyph("space", 1000)
	in.Glyphs = append(in.Glyphs, g)

	g = sfntcff.NewGlyph("A", 900)
	g.MoveTo(50, 50)
	g.LineTo(850, 50)
	g.LineTo(850, 850)
	g.LineTo(50, 850)
	in.Glyphs = append(in.Glyphs, g)

	fontData := &sfnt.Font{
		FamilyName: "Dummy",
		Width:      os2.WidthNormal,
		Weight:     os2.WeightNormal,
		IsRegular:  true,
		UnitsPerEm: 1000,
		FontMatrix: [6]float64{0.001, 0, 0, 0.001, 0, 0},
		Ascent:     850,
		Descent:    0,
		LineGap:    1000,
		CapHeight:  850,
		Outlines:   in,
	}

	subtable := cmap.Format4{}
	subtable[' '] = 1
	subtable['A'] = 2
	fontData.InstallCMap(subtable)

	return pdfcff.New(fontData, nil)
}
