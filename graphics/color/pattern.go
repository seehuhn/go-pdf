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

// Pattern represents a PDF pattern.
//
// Use the functions in [seehuhn.de/go/pdf/graphics/pattern] to create pattern
// objects.
type Pattern interface {
	// PatternType returns 1 for tiling patterns and 2 for shading patterns.
	PatternType() int

	// PaintType returns 1 for colored patterns and 2 for uncolored patterns.
	PaintType() int

	pdf.Embedder[pdf.Unused]
}

// == colored patterns and shadings ==========================================

// spacePatternColored is used for colored tiling patterns and shading patterns.
type spacePatternColored struct{}

// Family returns /Pattern.
// This implements the [Space] interface.
func (s spacePatternColored) Family() pdf.Name {
	return FamilyPattern
}

// Channels returns 0, to indicate that no color values are needed for
// colored patterns.
// This implements the [Space] interface.
func (s spacePatternColored) Channels() int {
	return 0
}

// Embed adds the color space to a PDF file.
// This implements the [Space] interface.
func (s spacePatternColored) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	if err := pdf.CheckVersion(rm.Out, "Pattern color space", pdf.V1_2); err != nil {
		return nil, zero, err
	}
	return pdf.Name("Pattern"), zero, nil
}

// Default returns a pattern which causes nothing to be drawn.
// This implements the [Space] interface.
func (s spacePatternColored) Default() Color {
	return colorColoredPattern{Pat: nil}
}

type colorColoredPattern struct {
	Pat Pattern
}

// PatternColored returns a new colored pattern as a PDF color.
// This can be used with colored tiling patterns and with shading patterns.
func PatternColored(p Pattern) Color {
	if p.PaintType() != 1 {
		panic("pattern is not colored")
	}
	return colorColoredPattern{Pat: p}
}

func (colorColoredPattern) ColorSpace() Space {
	return spacePatternColored{}
}

// == uncolored patterns =====================================================

// spacePatternUncolored represents the color space for uncolored patterns
// (where the color is specified separately).
type spacePatternUncolored struct {
	base Space
}

// Family returns /Pattern.
// This implements the [Space] interface.
func (s spacePatternUncolored) Family() pdf.Name {
	return FamilyPattern
}

// Channels returns the number of color channels in the base color space.
// This implements the [Space] interface.
func (s spacePatternUncolored) Channels() int {
	return s.base.Channels()
}

// Embed adds the pattern color space to the PDF file.
// This implements the [Space] interface.
func (s spacePatternUncolored) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
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
	return colorUncoloredPattern{Pat: nil, Col: s.base.Default()}
}

type colorUncoloredPattern struct {
	Pat Pattern
	Col Color
}

// PatternUncolored returns a new PDF color which draws the given pattern
// using the given color.
func PatternUncolored(p Pattern, col Color) Color {
	if p.PaintType() != 2 {
		panic("pattern is colored")
	}
	return colorUncoloredPattern{Pat: p, Col: col}
}

func (c colorUncoloredPattern) ColorSpace() Space {
	return spacePatternUncolored{base: c.Col.ColorSpace()}
}
