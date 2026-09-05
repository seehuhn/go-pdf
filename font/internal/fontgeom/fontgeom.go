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

// Package fontgeom derives [font.Geometry] values from font programs.
//
// The font backends share this code so that the invariants documented for
// [font.Geometry] hold however a font was embedded.
package fontgeom

import (
	"seehuhn.de/go/geom/rect"
	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// Values used where a font reports no vertical dimensions at all.  They are
// chosen so that the resulting estimates satisfy
// xHeight <= capHeight <= leading.
const (
	defaultLeading   = 1.2
	defaultCapHeight = 0.66
	defaultXHeight   = 0.46
)

// FromSFNT returns the geometry of an OpenType or TrueType font, in text
// space units, together with the font bounding box in PDF glyph space units.
// The bounding box is the union of the glyph extents, and is returned here
// so that the font descriptor needs no second pass over the glyph outlines.
//
// The cap height and x-height need no measuring here: the sfnt reader
// already measures them from the glyphs where the OS/2 table has none.
func FromSFNT(info *sfnt.Font) (*font.Geometry, rect.Rect) {
	q := 1 / float64(info.UnitsPerEm)

	// GlyphBBoxesPDF returns 1000-scale glyph space; convert to text space
	glyphExtents := info.GlyphBBoxesPDF()
	var fontBBox rect.Rect
	for i := range glyphExtents {
		fontBBox.Extend(glyphExtents[i])
		glyphExtents[i].Scale(1.0 / 1000)
	}

	g := &font.Geometry{
		Ascent:             float64(info.Ascent) * q,
		Descent:            float64(info.Descent) * q,
		Leading:            float64(info.Ascent-info.Descent+info.LineGap) * q,
		CapHeight:          float64(info.CapHeight) * q,
		XHeight:            float64(info.XHeight) * q,
		UnderlinePosition:  pdf.Round(float64(info.UnderlinePosition)*q, 6),
		UnderlineThickness: pdf.Round(float64(info.UnderlineThickness)*q, 6),

		GlyphExtents: glyphExtents,
		Widths:       info.WidthsPDF(),
	}
	FillHeights(g)
	return g, fontBBox
}

// FillHeights supplies a leading, cap height and x-height for fonts which
// report none, establishing 0 < XHeight <= CapHeight <= Leading.  Dimensions
// the font does report are left alone, even where they break that ordering.
//
// Callers which can measure a height from the glyph outlines should do so
// before calling this function, since the values invented here are only a
// last resort.
func FillHeights(g *font.Geometry) {
	if g.Leading <= 0 {
		g.Leading = defaultLeading
	}
	if g.CapHeight <= 0 {
		g.CapHeight = min(defaultCapHeight, g.Leading)
	}
	if g.XHeight <= 0 {
		g.XHeight = min(defaultXHeight, g.CapHeight)
	}
}
