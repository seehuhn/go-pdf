// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

// Package font implements the PDF font handling.
package font

import (
	"math"

	"seehuhn.de/go/postscript/funit"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
)

type Geometry struct {
	UnitsPerEm         uint16
	Ascent             funit.Int16
	Descent            funit.Int16 // negative
	BaseLineSkip       funit.Int16
	UnderlinePosition  funit.Float64
	UnderlineThickness funit.Float64

	GlyphExtents []funit.Rect16
	Widths       []funit.Int16
}

func (g *Geometry) ToPDF(fontSize float64, a funit.Int) float64 {
	return float64(a) * fontSize / float64(g.UnitsPerEm)
}

func (g *Geometry) ToPDF16(fontSize float64, a funit.Int16) float64 {
	return float64(a) * fontSize / float64(g.UnitsPerEm)
}

func (g *Geometry) FromPDF16(fontSize float64, x float64) funit.Int16 {
	return funit.Int16(math.Round(x / fontSize * float64(g.UnitsPerEm)))
}

func (g *Geometry) BoundingBox(fontSize float64, gg glyph.Seq) *pdf.Rectangle {
	var bbox funit.Rect
	var xPos funit.Int
	for _, glyph := range gg {
		b16 := g.GlyphExtents[glyph.Gid]
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

// Font represents a font which can be embedded in a PDF file.
type Font interface {
	Embed(w pdf.Putter, resName pdf.Name) (Embedded, error)
	GetGeometry() *Geometry
	Layout(s string, ptSize float64) glyph.Seq
}

// Embedded represents a font embedded in a PDF file.
type Embedded interface {
	Font
	AppendEncoded(pdf.String, glyph.ID, []rune) pdf.String
	ResourceName() pdf.Name
	pdf.Closer
}

// NumGlyphs returns the number of glyphs in a font.
func NumGlyphs(font Font) int {
	g := font.GetGeometry()
	return len(g.Widths)
}

// GetGID returns the glyph ID and advance width for a given rune.
// A glyph ID of 0 indicates that the rune is not supported by the font.
func GetGID(font Font, r rune) (glyph.ID, funit.Int16) {
	gg := font.Layout(string(r), 10)
	if len(gg) != 1 {
		return 0, 0
	}
	return gg[0].Gid, gg[0].Advance
}
