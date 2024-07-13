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
// Res.Data must be a PDF stream defining a pattern with PatternType 1 (tiling
// pattern) and PaintType 2 (uncolored).  The function
// [seehuhn.de/go/pdf/graphics/pattern.NewTilingUncolored] can be used to
// create a new tiling pattern.
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

func (s spacePatternColored) Embed(w pdf.Putter) (pdf.Resource, error) {
	return s, nil
}

// PDFObject implements the [Space] interface.
func (s spacePatternColored) PDFObject() pdf.Object {
	return pdf.Name("Pattern")
}

// ColorSpaceFamily implements the [Space] interface.
func (s spacePatternColored) ColorSpaceFamily() pdf.Name {
	return "Pattern"
}

// defaultValues implements the [Space] interface.
func (s spacePatternColored) defaultValues() []float64 {
	return nil
}

// PatternColored represents a colored tiling pattern or a shading pattern.
//
// In case of a colored tiling pattern, `Res.Data“ must be a PDF stream
// defining a pattern with PatternType 1 (tiling pattern) and PaintType 1
// (colored).  Use [seehuhn.de/go/pdf/graphics/pattern.NewTilingColored] to
// create this type of pattern.
//
// In case of a shading pattern, `Res.Data“ must be a PDF pattern dictionary
// with PatternType 2 (shading pattern). Use
// [seehuhn.de/go/pdf/graphics/pattern.NewShadingPattern] to create this type
// of pattern.
type PatternColored struct {
	// TODO(voss): do we need to distinguish between free and embedded patterns?
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
	base Space
}

// ColorSpaceFamily implements the [Space] interface.
func (s spacePatternUncolored) ColorSpaceFamily() pdf.Name {
	return "Pattern"
}

// defaultValues implements the [Space] interface.
func (s spacePatternUncolored) defaultValues() []float64 {
	return nil
}

func (s spacePatternUncolored) Embed(w pdf.Putter) (pdf.Resource, error) {
	// TODO(voss): somehow route this through the graphics.ResourceManager???
	e, err := s.base.Embed(w)
	if err != nil {
		return nil, err
	}
	return spacePatternUncoloredEmbedded{base: e}, nil
}

type spacePatternUncoloredEmbedded struct {
	base pdf.Resource
}

func (s spacePatternUncoloredEmbedded) PDFObject() pdf.Object {
	return pdf.Array{
		pdf.Name("Pattern"),
		s.base.PDFObject(),
	}
}

type colorPatternUncolored struct {
	pdf.Res
	col Color
}

// ColorSpace implements the [Color] interface.
func (c colorPatternUncolored) ColorSpace() Space {
	return spacePatternUncolored{base: c.col.ColorSpace()}
}

// values implements the [Color] interface.
func (c colorPatternUncolored) values() []float64 {
	return c.col.values()
}
