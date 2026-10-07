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
	"errors"
	stdcolor "image/color"

	"seehuhn.de/go/icc"
	"seehuhn.de/go/pdf"
)

// Pattern represents a PDF pattern.
//
// Use the functions in [seehuhn.de/go/pdf/graphics/pattern] to create pattern
// objects.
//
// To extract a pattern from a PDF file, use
// [seehuhn.de/go/pdf/graphics/extract.Pattern].
type Pattern interface {
	// PatternType returns 1 for tiling patterns and 2 for shading patterns.
	PatternType() int

	// PaintType returns 1 for colored patterns and 2 for uncolored patterns.
	PaintType() int

	// Equal reports whether two patterns are equal.
	Equal(other Pattern) bool

	pdf.Embedder
}

// PDF 2.0 sections: 8.6.6.2

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

// ComponentRange panics.  Colored patterns have no numeric components, so
// Channels returns 0 and no component index is valid.
// This implements the [Space] interface.
func (s spacePatternColored) ComponentRange(i int) (lo, hi float64) {
	panic("colored pattern has no color components")
}

// Embed adds the color space to a PDF file.
// This implements the [Space] interface.
func (s spacePatternColored) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "Pattern color space", pdf.V1_2); err != nil {
		return nil, err
	}
	return pdf.Name("Pattern"), nil
}

// Default returns a pattern which causes nothing to be drawn.
// This implements the [Space] interface.
func (s spacePatternColored) Default() Color {
	return colorPatternColored{Pat: nil}
}

// Convert returns the default color (nil pattern) since a color cannot be
// meaningfully converted to a pattern.
// This implements the [stdcolor.Model] interface.
func (s spacePatternColored) Convert(c stdcolor.Color) stdcolor.Color {
	// patterns cannot be derived from other colors
	if cp, ok := c.(colorPatternColored); ok {
		return cp
	}
	return s.Default()
}

// ToXYZ returns a placeholder mid-gray in CIE XYZ tristimulus values
// adapted to the Profile Connection Space white point, since the actual
// color depends on the pattern content.
func (s spacePatternColored) ToXYZ(values []float64, ws *icc.Workspace) (X, Y, Z float64) {
	return srgbToXYZ(0.5, 0.5, 0.5)
}

type colorPatternColored struct {
	Pat Pattern
}

// PatternColored returns a new colored pattern as a PDF color.
// This can be used with colored tiling patterns and with shading patterns.
func PatternColored(p Pattern) Color {
	if p.PaintType() != 1 {
		panic("pattern is not colored")
	}
	return colorPatternColored{Pat: p}
}

func (colorPatternColored) ColorSpace() Space {
	return spacePatternColored{}
}

// ToXYZ returns the colour as CIE XYZ tristimulus values
// adapted to the Profile Connection Space white point.
// For colored patterns, returns a neutral mid-gray since the actual color
// depends on the pattern content which is not available here.
func (colorPatternColored) ToXYZ() (X, Y, Z float64) {
	return srgbToXYZ(0.5, 0.5, 0.5)
}

// RGBA implements the color.Color interface.
// For colored patterns, returns a neutral gray since the actual color
// depends on the pattern content which is not available here.
func (colorPatternColored) RGBA() (r, g, b, a uint32) {
	return 0x8000, 0x8000, 0x8000, 0xffff
}

// == uncolored patterns =====================================================

// PDF 2.0 sections: 8.6.6.2

// SpacePatternUncolored represents the color space for monochrome patterns
// (where the color is specified separately).
//
// The underlying base color space is never itself a Pattern color space.
type SpacePatternUncolored struct {
	Base Space
}

// Family returns /Pattern.
// This implements the [Space] interface.
func (s SpacePatternUncolored) Family() pdf.Name {
	return FamilyPattern
}

// Channels returns the number of color channels in the base color space.
// This implements the [Space] interface.
func (s SpacePatternUncolored) Channels() int {
	return s.Base.Channels()
}

// ComponentRange delegates to the underlying color space.
// This implements the [Space] interface.
func (s SpacePatternUncolored) ComponentRange(i int) (lo, hi float64) {
	return s.Base.ComponentRange(i)
}

// Embed adds the pattern color space to the PDF file.
// This implements the [Space] interface.
func (s SpacePatternUncolored) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "Pattern color space", pdf.V1_2); err != nil {
		return nil, err
	}
	if IsPattern(s.Base) {
		return nil, errPatternBase
	}
	base, err := rm.Embed(s.Base)
	if err != nil {
		return nil, err
	}

	return pdf.Array{pdf.Name("Pattern"), base}, nil
}

var errPatternBase = errors.New("uncolored pattern base cannot be a Pattern color space")

// Default returns a pattern which causes nothing to be drawn.
func (s SpacePatternUncolored) Default() Color {
	return colorPatternUncolored{Pat: nil, Col: s.Base.Default()}
}

// Convert converts the base colour and returns it with a nil pattern.
// This implements the [stdcolor.Model] interface.
func (s SpacePatternUncolored) Convert(c stdcolor.Color) stdcolor.Color {
	if up, ok := c.(colorPatternUncolored); ok {
		// already an uncolored pattern over the same base space
		if SpacesEqual(up.Col.ColorSpace(), s.Base) {
			return up
		}
	}

	// convert the colour to the base space
	baseColor := s.Base.Convert(c)
	if bc, ok := baseColor.(Color); ok {
		return colorPatternUncolored{Pat: nil, Col: bc}
	}
	return s.Default()
}

// ToXYZ converts the base color values to CIE XYZ tristimulus values
// adapted to the Profile Connection Space white point.
func (s SpacePatternUncolored) ToXYZ(values []float64, ws *icc.Workspace) (X, Y, Z float64) {
	return s.Base.ToXYZ(values, ws)
}

type colorPatternUncolored struct {
	Pat Pattern
	Col Color
}

// PatternUncolored returns a new PDF color which draws the given pattern
// using the given color.
//
// PatternUncolored panics if p is a colored pattern (PaintType != 2), or
// if col's color space is itself a Pattern color space (PDF 2.0
// §8.7.3.3 forbids nesting an uncolored pattern over another Pattern
// color space).
func PatternUncolored(p Pattern, col Color) Color {
	if p.PaintType() != 2 {
		panic("pattern is colored")
	}
	if IsPattern(col.ColorSpace()) {
		panic(errPatternBase)
	}
	return colorPatternUncolored{Pat: p, Col: col}
}

func (c colorPatternUncolored) ColorSpace() Space {
	return SpacePatternUncolored{Base: c.Col.ColorSpace()}
}

// ToXYZ returns the colour as CIE XYZ tristimulus values
// adapted to the Profile Connection Space white point.
func (c colorPatternUncolored) ToXYZ() (X, Y, Z float64) {
	return c.Col.ToXYZ()
}

// RGBA implements the color.Color interface.
// Returns the RGBA values of the underlying color.
func (c colorPatternUncolored) RGBA() (r, g, b, a uint32) {
	return c.Col.RGBA()
}
