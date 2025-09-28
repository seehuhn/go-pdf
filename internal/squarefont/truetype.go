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
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/maxp"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/truetype"
)

// makeTrueType500 creates a TrueType font with 500 units per em.
func makeTrueType500() font.Layouter {
	return makeTrueTypeFont(500)
}

// makeTrueType1000 creates a TrueType font with 1000 units per em.
func makeTrueType1000() font.Layouter {
	return makeTrueTypeFont(1000)
}

// makeTrueType2000 creates a TrueType font with 2000 units per em.
func makeTrueType2000() font.Layouter {
	return makeTrueTypeFont(2000)
}

// makeTrueTypeFont creates a TrueType font with the specified units per em.
func makeTrueTypeFont(unitsPerEm uint16) font.Layouter {
	// Calculate scale factor for coordinates
	scale := float64(unitsPerEm) / 1000.0

	// Create glyph outlines
	glyphs := make(glyf.Glyphs, 3)
	glyphs[0] = nil                      // .notdef
	glyphs[1] = nil                      // space
	glyphs[2] = createSquareGlyph(scale) // A

	// Create widths
	widths := []funit.Int16{
		funit.Int16(NotdefWidth * scale), // .notdef width
		funit.Int16(SpaceWidth * scale),  // space width
		funit.Int16(SquareWidth * scale), // A width
	}

	// Create outlines
	outlines := &glyf.Outlines{
		Glyphs: glyphs,
		Widths: widths,
		Maxp: &maxp.TTFInfo{
			MaxPoints:   4,
			MaxContours: 1,
		},
	}

	cmapSubtable := cmap.Format4{' ': 1, 'A': 2}
	cmapTable := cmap.Table{
		{PlatformID: 0, EncodingID: 3}: cmapSubtable.Encode(0),
	}

	// Create font structure
	font := &sfnt.Font{
		FamilyName:         "SquareFont",
		Ascent:             funit.Int16(Ascent * scale),
		Descent:            funit.Int16(Descent * scale),
		LineGap:            funit.Int16((Leading - Ascent + Descent) * scale),
		UnderlinePosition:  funit.Float64(UnderlinePosition * scale),
		UnderlineThickness: funit.Float64(UnderlineThickness * scale),
		CapHeight:          funit.Int16(600 * scale),
		XHeight:            funit.Int16(400 * scale),
		Outlines:           outlines,
		Width:              os2.WidthNormal,
		Weight:             os2.WeightMedium,
		IsRegular:          true,
		PermUse:            os2.PermInstall,
		UnitsPerEm:         unitsPerEm,
		FontMatrix: matrix.Matrix{
			1 / float64(unitsPerEm), 0, 0, 1 / float64(unitsPerEm), 0, 0,
		},
		CMapTable: cmapTable,
	}

	// Convert to PDF TrueType font
	instance, err := truetype.NewSimple(font, nil)
	if err != nil {
		panic(err)
	}
	return instance
}

// createSquareGlyph creates a filled square glyph.
func createSquareGlyph(scale float64) *glyf.Glyph {
	// Calculate scaled coordinates
	left := funit.Int16(SquareLeft * scale)
	right := funit.Int16(SquareRight * scale)
	bottom := funit.Int16(SquareBottom * scale)
	top := funit.Int16(SquareTop * scale)

	// Create a single contour with four points forming a rectangle
	contour := glyf.Contour{
		{X: left, Y: bottom, OnCurve: true},  // bottom-left
		{X: right, Y: bottom, OnCurve: true}, // bottom-right
		{X: right, Y: top, OnCurve: true},    // top-right
		{X: left, Y: top, OnCurve: true},     // top-left
	}

	info := &glyf.SimpleUnpacked{
		Contours:     []glyf.Contour{contour},
		Instructions: nil,
	}
	glyph := info.AsGlyph()
	return &glyph
}
