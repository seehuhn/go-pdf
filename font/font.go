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
// TODO(voss): make sure that there is a good way to determine the number
// of glyphs in the font?
type Font struct {
	Name pdf.Name
	Ref  *pdf.Reference

	CMap       map[rune]GlyphID
	Substitute func(glyphs []GlyphID) []GlyphID
	Layout     func(glyphs []GlyphID) []GlyphPos
	Enc        func(GlyphID) pdf.String

	GlyphUnits int

	GlyphExtent []Rect
	Width       []int

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
	Glyphs   []GlyphPos
}

// GlyphPos contains layout information for a single glyph in a run
type GlyphPos struct {
	Gid     GlyphID
	XOffset int
	YOffset int
	Advance int
}

// Typeset computes all information required to typeset the given string
// in a PDF file.
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
	var glyphs []GlyphID
	for _, run := range runs {
		pos := len(glyphs)
		for _, r := range run {
			glyphs = append(glyphs, font.CMap[r])
		}
		glyphs = append(glyphs[:pos], font.Substitute(glyphs[pos:])...)
	}

	// apply kerning etc.
	layout := font.Layout(glyphs)

	return &Layout{
		Font:     font,
		FontSize: ptSize,
		Glyphs:   layout,
	}
}

// GlyphPair represents two consecutive glyphs, specified by a pair of
// character codes.  This is used for ligatures and kerning information.
type GlyphPair [2]GlyphID
