// seehuhn.de/go/pdf - support for reading and writing PDF files
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

package font

import (
	"unicode"

	"seehuhn.de/go/pdf"
)

// Font represents a font embedded in the PDF file.
type Font struct {
	Name pdf.Name
	Ref  *pdf.Reference

	CMap       map[rune]GlyphID
	Substitute func(glyphs []Glyph) []Glyph
	Layout     func(glyphs []Glyph)
	Enc        func(GlyphID) pdf.String

	GlyphUnits int

	GlyphExtent []Rect
	Width       []int // TODO(voss): needed?

	Ascent  float64 // Ascent in glyph coordinate units
	Descent float64 // Descent in glyph coordinate units, as a negative number
	LineGap float64 // TODO(voss): remove?
}

// GlyphID is used to enumerate the glyphs in a font.  The first glyph
// has index 0 and is used to indicate a missing character (usually rendered
// as an empty box).
type GlyphID uint16

// Rect represents a rectangle with integer coordinates.
type Rect struct {
	LLx, LLy, URx, URy int
}

// IsZero returns whether the glyph leaves marks on the page.
func (rect *Rect) IsZero() bool {
	return rect.LLx == 0 && rect.LLy == 0 && rect.URx == 0 && rect.URy == 0
}

// Layout contains the information needed to typeset a run of text.
type Layout struct {
	Font     *Font
	FontSize float64
	Glyphs   []Glyph
}

// Glyph contains layout information for a single glyph in a run
type Glyph struct {
	Gid     GlyphID
	Chars   []rune
	XOffset int
	YOffset int
	Advance int
}

// Typeset computes all glyph and layout information required to typeset a
// string in a PDF file.
func (font *Font) Typeset(s string, ptSize float64) *Layout {
	var runs [][]rune
	var run []rune
	for _, r := range s {
		if unicode.IsGraphic(r) {
			run = append(run, r)
		} else if len(run) > 0 {
			runs = append(runs, run)
			run = nil
		}
	}
	if len(run) > 0 {
		runs = append(runs, run)
	}

	// introduce ligatures, fix mark glyphs etc.
	var glyphs []Glyph
	for _, run := range runs {
		pos := len(glyphs)
		for _, r := range run {
			g := Glyph{
				Gid:   font.CMap[r],
				Chars: []rune{r},
			}
			glyphs = append(glyphs, g)
		}
		glyphs = append(glyphs[:pos], font.Substitute(glyphs[pos:])...)
	}

	// determine glyph positions, apply kerning etc.
	font.Layout(glyphs)

	return &Layout{
		Font:     font,
		FontSize: ptSize,
		Glyphs:   glyphs,
	}
}

// GlyphPair represents two consecutive glyphs, specified by a pair of
// character codes.  This is used for ligatures and kerning information.
type GlyphPair [2]GlyphID
