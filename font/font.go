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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/sfnt/funit"
	"seehuhn.de/go/sfnt/glyph"
)

type Geometry struct {
	UnitsPerEm         uint16
	Ascent             funit.Int16
	Descent            funit.Int16 // negative
	BaseLineSkip       funit.Int16
	UnderlinePosition  funit.Int16
	UnderlineThickness funit.Int16

	GlyphExtents []funit.Rect
	Widths       []funit.Int16
}

func (g *Geometry) ToPDF(fontSize float64, x funit.Int) float64 {
	return float64(x) * fontSize / float64(g.UnitsPerEm)
}

func (g *Geometry) ToPDF16(fontSize float64, x funit.Int16) float64 {
	return float64(x) * fontSize / float64(g.UnitsPerEm)
}

type Layouter interface {
	GetGeometry() *Geometry
	Layout(s string, ptSize float64) glyph.Seq
}

// Font represents a font which can be embedded in a PDF file.
type Font interface {
	Layouter
	Embed(w *pdf.Writer, resName pdf.Name) (Embedded, error)
}

// Embedded represents a font embedded in a PDF file.
type Embedded interface {
	Layouter
	AppendEncoded(pdf.String, glyph.ID, []rune) pdf.String

	ResourceName() pdf.Name

	pdf.Resource
}

// NumGlyphs returns the number of glyphs in a font.
func NumGlyphs(font Layouter) int {
	g := font.GetGeometry()
	return len(g.Widths)
}
