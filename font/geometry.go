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

package font

import (
	"seehuhn.de/go/pdf"
)

// Geometry collects the various dimensions connected to a font and to
// the individual glyphs.  All fields are measured in PDF text space units,
// and need to be scaled by the font size.
type Geometry struct {
	Ascent             float64
	Descent            float64 // negative
	BaseLineDistance   float64
	UnderlinePosition  float64
	UnderlineThickness float64

	GlyphExtents []pdf.Rectangle // indexed by GID
	Widths       []float64       // indexed by GID
}

// GetGeometry returns the geometry of a font.
func (g *Geometry) GetGeometry() *Geometry {
	return g
}

// BoundingBox returns the bounding box of a glyph sequence,
// assuming that it is typeset at point (0, 0) using the given font size.
func (g *Geometry) BoundingBox(fontSize float64, gg *GlyphSeq) *pdf.Rectangle {
	res := &pdf.Rectangle{}

	xPos := gg.Skip
	for _, glyph := range gg.Seq {
		bbox := g.GlyphExtents[glyph.GID]
		if bbox.IsZero() {
			continue
		}

		b := &pdf.Rectangle{
			LLx: bbox.LLx*fontSize + xPos,
			LLy: bbox.LLy*fontSize + glyph.Rise,
			URx: bbox.URx*fontSize + xPos,
			URy: bbox.URy*fontSize + glyph.Rise,
		}
		res.Extend(b)
		xPos += glyph.Advance
	}
	return res
}
