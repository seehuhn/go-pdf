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

// spacePatternColored is used for colored tiling patterns and shading patterns.
type spacePatternColored struct{}

func (s spacePatternColored) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused
	if err := pdf.CheckVersion(rm.Out, "Pattern color space", pdf.V1_2); err != nil {
		return nil, zero, err
	}
	return pdf.Name("Pattern"), zero, nil
}

// ColorSpaceFamily implements the [Space] interface.
func (s spacePatternColored) ColorSpaceFamily() pdf.Name {
	return FamilyPattern
}

// Default returns a pattern which causes nothing to be drawn.
// This implements the [Space] interface.
func (s spacePatternColored) Default() Color {
	return colorColoredPattern{Pat: nil}
}

// defaultValues implements the [Space] interface.
func (s spacePatternColored) defaultValues() []float64 {
	return nil
}

// NewColoredPattern returns a new colored pattern as a PDF color.
// This can be used with colored tiling patterns and with shading patterns.
func NewColoredPattern(p Pattern) Color {
	if !p.IsColored() {
		panic("pattern is not colored")
	}
	return colorColoredPattern{Pat: p}
}

type colorColoredPattern struct {
	Pat Pattern
}

func (colorColoredPattern) ColorSpace() Space {
	return spacePatternColored{}
}

func (c colorColoredPattern) values() []float64 {
	return nil
}

// == uncolored patterns =====================================================

type spacePatternUncolored struct {
	base Space
}

// ColorSpaceFamily implements the [Space] interface.
func (s spacePatternUncolored) ColorSpaceFamily() pdf.Name {
	return FamilyPattern
}

func (s spacePatternUncolored) Embed(rm *pdf.ResourceManager) (pdf.Object, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "Pattern color space", pdf.V1_2); err != nil {
		return nil, zero, err
	}
	base, _, err := pdf.ResourceManagerEmbed(rm, s.base)
	if err != nil {
		return nil, zero, err
	}

	return pdf.Array{pdf.Name("Pattern"), base}, zero, nil
}

// Default returns a pattern which causes nothing to be drawn.
func (s spacePatternUncolored) Default() Color {
	return &colorUncoloredPattern{Pat: nil, Col: s.base.Default()}
}

// defaultValues implements the [Space] interface.
func (s spacePatternUncolored) defaultValues() []float64 {
	return s.base.defaultValues()
}

// NewUncoloredPattern returns a new uncolored pattern as a PDF color.
func NewUncoloredPattern(p Pattern, col Color) Color {
	if p.IsColored() {
		panic("pattern is colored")
	}
	return &colorUncoloredPattern{Pat: p, Col: col}
}

type colorUncoloredPattern struct {
	Pat Pattern
	Col Color
}

func (c *colorUncoloredPattern) ColorSpace() Space {
	return spacePatternUncolored{base: c.Col.ColorSpace()}
}

func (c *colorUncoloredPattern) values() []float64 {
	return c.Col.values()
}
