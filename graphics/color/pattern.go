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

// == colored patterns and shadings ==========================================

// spacePatternColored is used for uncolored tiling patterns and shading patterns.
type spacePatternColored struct{}

// DefaultName implements the [ColorSpace] interface.
func (s spacePatternColored) DefaultName() pdf.Name {
	return ""
}

// PDFObject implements the [ColorSpace] interface.
func (s spacePatternColored) PDFObject() pdf.Object {
	return pdf.Name("Pattern")
}

// ColorSpaceFamily implements the [ColorSpace] interface.
func (s spacePatternColored) ColorSpaceFamily() string {
	return "Pattern"
}

// defaultColor implements the [ColorSpace] interface.
func (s spacePatternColored) defaultColor() Color {
	return nil
}

type colorPatternColored struct {
	pattern pdf.Res
}

// NewTilingPatternColored returns a new colored tiling pattern. Ref must be a
// stream defining a pattern with PatternType 1 and PaintType 1.
//
// DefName should normally be empty but can be set to specify a default name
// for refering to the pattern from within content streams.
//
// TODO(voss): make this API more robust
func NewTilingPatternColored(pattern pdf.Res) Color {
	return colorPatternColored{
		pattern: pattern,
	}
}

// NewShadingPattern returns a new shading pattern.  Dict must be a shading
// dictionary with PatternType 2.
//
// DefName should normally be empty but can be set to specify a default name
// for refering to the pattern from within content streams.
//
// TODO(voss): make this API more robust
func NewShadingPattern(dict pdf.Object, defName pdf.Name) Color {
	return colorPatternColored{
		pattern: pdf.Res{
			DefName: defName,
			Data:    dict,
		},
	}
}

func (c colorPatternColored) ColorSpace() Space {
	return spacePatternColored{}
}

func (c colorPatternColored) values() []float64 {
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

// ColorSpaceFamily implements the [ColorSpace] interface.
func (s spacePatternUncolored) ColorSpaceFamily() string {
	return "Pattern"
}

// defaultColor implements the [ColorSpace] interface.
func (s spacePatternUncolored) defaultColor() Color {
	return nil
}

type colorPatternUncolored struct {
	col     Color
	pattern pdf.Res
}

// NewTilingPatternUncolored returns a new uncolored tiling pattern.
//
// pattern.Ref must be a stream defining a pattern with PatternType 1 and
// PaintType 2.
//
// pattern.DefName should normally be empty but can be set to specify a default
// name for refering to the pattern from within content streams.
//
// Color is the color to paint the pattern with.
//
// TODO(voss): make this API more robust
func NewTilingPatternUncolored(pattern pdf.Res, color Color) Color {
	return colorPatternUncolored{
		col:     color,
		pattern: pattern,
	}
}

// ColorSpace implements the [Color] interface.
func (c colorPatternUncolored) ColorSpace() Space {
	return spacePatternUncolored{s: c.col.ColorSpace()}
}

// values implements the [Color] interface.
func (c colorPatternUncolored) values() []float64 {
	return c.col.values()
}

// ===========================================================================
