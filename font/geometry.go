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
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/funit"
	"seehuhn.de/go/sfnt/glyph"
)

// Geometry collects the various dimensions connected to a font and to
// the individual glyphs.
//
// TODO(voss): use PDF coordinates
type Geometry struct {
	UnitsPerEm uint16

	Ascent             float64
	Descent            float64 // negative
	BaseLineDistance   float64
	UnderlinePosition  float64
	UnderlineThickness float64

	GlyphExtents []funit.Rect16 // indexed by GID
	Widths       []funit.Int16  // indexed by GID
}

// GetGeometry returns the geometry of a font.
func (g *Geometry) GetGeometry() *Geometry {
	return g
}

// FontMatrix returns the font matrix for a font.
func (g *Geometry) FontMatrix() []float64 {
	return []float64{1 / float64(g.UnitsPerEm), 0, 0, 1 / float64(g.UnitsPerEm), 0, 0}
}

// ToPDF converts an integer from font design units to PDF units.
func (g *Geometry) ToPDF(fontSize float64, a funit.Int) float64 {
	return float64(a) * fontSize / float64(g.UnitsPerEm)
}

// ToPDF16 converts an int16 from font design units to PDF units.
func (g *Geometry) ToPDF16(fontSize float64, a funit.Int16) float64 {
	return float64(a) * fontSize / float64(g.UnitsPerEm)
}

// FromPDF16 converts from PDF units (given as a float64) to an int16 in
// font design units.
func (g *Geometry) FromPDF16(fontSize float64, x float64) funit.Int16 {
	return funit.Int16(math.Round(x / fontSize * float64(g.UnitsPerEm)))
}

// BoundingBox returns the bounding box of a glyph sequence,
// in PDF units.
func (g *Geometry) BoundingBox(fontSize float64, gg glyph.Seq) *pdf.Rectangle {
	var bbox funit.Rect
	var xPos funit.Int
	for _, glyph := range gg {
		b16 := g.GlyphExtents[glyph.GID]
		b := funit.Rect{
			LLx: funit.Int(b16.LLx+glyph.XOffset) + xPos,
			LLy: funit.Int(b16.LLy + glyph.YOffset),
			URx: funit.Int(b16.URx+glyph.XOffset) + xPos,
			URy: funit.Int(b16.URy + glyph.YOffset),
		}
		bbox.Extend(b)
		xPos += funit.Int(glyph.Advance)
	}

	res := &pdf.Rectangle{
		LLx: g.ToPDF(fontSize, bbox.LLx),
		LLy: g.ToPDF(fontSize, bbox.LLy),
		URx: g.ToPDF(fontSize, bbox.URx),
		URy: g.ToPDF(fontSize, bbox.URy),
	}
	return res
}

// BoundingBoxNew returns the bounding box of a glyph sequence,
// assuming that it is typeset at point (0, 0) using the given font size.
func (g *Geometry) BoundingBoxNew(fontSize float64, gg *GlyphSeq) *pdf.Rectangle {
	res := &pdf.Rectangle{}

	q := fontSize / float64(g.UnitsPerEm)

	xPos := gg.Skip
	for _, glyph := range gg.Seq {
		b16 := g.GlyphExtents[glyph.GID]
		if b16.IsZero() {
			continue
		}

		b := &pdf.Rectangle{
			LLx: b16.LLx.AsFloat(q) + xPos,
			LLy: b16.LLy.AsFloat(q) + glyph.Rise,
			URx: b16.URx.AsFloat(q) + xPos,
			URy: b16.URy.AsFloat(q) + glyph.Rise,
		}
		res.Extend(b)
		xPos += glyph.Advance
	}
	return res
}

// NumGlyphs returns the number of glyphs in a font.
//
// TODO(voss): remove?
func NumGlyphs(font interface{ GetGeometry() *Geometry }) int {
	g := font.GetGeometry()
	return len(g.Widths)
}
