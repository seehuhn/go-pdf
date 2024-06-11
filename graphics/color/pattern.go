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

package color

import (
	"seehuhn.de/go/pdf"
)

// TilingPatternUncolored represents an uncolored PDF tiling pattern.
//
// Res.Data must be a PDF stream defining a pattern with PatternType 1 and
// PaintType 2.
type TilingPatternUncolored struct {
	pdf.Res
}

// New returns a Color which paints the given tiling pattern in the given
// color.
func (p *TilingPatternUncolored) New(col Color) Color {
	return &colorPatternUncolored{
		Res: p.Res,
		col: col,
	}
}

// == colored patterns and shadings ==========================================

// spacePatternColored is used for colored tiling patterns and shading patterns.
type spacePatternColored struct{}

// DefaultName implements the [Space] interface.
func (s spacePatternColored) DefaultName() pdf.Name {
	return ""
}

// PDFObject implements the [Space] interface.
func (s spacePatternColored) PDFObject() pdf.Object {
	return pdf.Name("Pattern")
}

// ColorSpaceFamily implements the [Space] interface.
func (s spacePatternColored) ColorSpaceFamily() pdf.Name {
	return "Pattern"
}

// defaultColor implements the [Space] interface.
func (s spacePatternColored) defaultColor() Color {
	return nil
}

// PatternColored represents a colored tiling pattern or a shading
// pattern.
//
// In case of a colored tiling pattern, `Res.Data“ must be a PDF stream
// defining a pattern with PatternType 1 and PaintType 1.  In case of a shading
// pattern, `Res.Data“ must be a PDF pattern dictionary with PatternType 2.
type PatternColored struct {
	pdf.Res
}

// ColorSpace implements the [Color] interface.
func (c PatternColored) ColorSpace() Space {
	return spacePatternColored{}
}

func (c PatternColored) values() []float64 {
	return nil
}

// == uncolored patterns =====================================================

type spacePatternUncolored struct {
	s Space
}

func (s spacePatternUncolored) DefaultName() pdf.Name {
	return pdf.Name("")
}

func (s spacePatternUncolored) PDFObject() pdf.Object {
	return pdf.Array{
		pdf.Name("Pattern"),
		s.s.PDFObject(),
	}
}

// ColorSpaceFamily implements the [Space] interface.
func (s spacePatternUncolored) ColorSpaceFamily() pdf.Name {
	return "Pattern"
}

// defaultColor implements the [Space] interface.
func (s spacePatternUncolored) defaultColor() Color {
	return nil
}

type colorPatternUncolored struct {
	pdf.Res
	col Color
}

// ColorSpace implements the [Color] interface.
func (c colorPatternUncolored) ColorSpace() Space {
	return spacePatternUncolored{s: c.col.ColorSpace()}
}

// values implements the [Color] interface.
func (c colorPatternUncolored) values() []float64 {
	return c.col.values()
}
