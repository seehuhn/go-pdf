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
	"unicode"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/sfnt/funit"
	"seehuhn.de/go/sfnt/glyph"
)

// Font represents a font which can be embedded in a PDF file.
type NewFont struct {
	UnitsPerEm         uint16
	Ascent             funit.Int16
	Descent            funit.Int16 // negative
	BaseLineSkip       funit.Int16
	UnderlinePosition  funit.Int16
	UnderlineThickness funit.Int16

	GlyphExtents []funit.Rect
	Widths       []funit.Int16

	// Layout converts a sequence of runes into a sequence of glyphs.  Runes
	// missing from the font are replaced by the glyph for the .notdef
	// character (glyph ID 0).  Glyph substitutions (e.g. from OpenType GSUB
	// tables) and positioning rules (e.g. kerning) are applied.
	Layout func([]rune) glyph.Seq

	ResourceName pdf.Name
	GetDict      func(w *pdf.Writer) (Dict, error)
}

type Dict interface {
	AppendEncoded(pdf.String, glyph.ID, []rune) pdf.String
	Reference() *pdf.Reference
	Write(w *pdf.Writer) error
}

// Font represents a font embedded in the PDF file.
type Font struct {
	InstName pdf.Name
	Ref      *pdf.Reference

	// Layout converts a sequence of runes into a sequence of glyphs.
	// Runes missing from the font are replaced by the glyph for the
	// .notdef character (glyph ID 0).  Glyph substitutions, e.g. from
	// OpenType GSUB tables, are applied.
	Layout func([]rune) glyph.Seq

	// Enc maps a glyph ID to a string that can be used in a PDF file.
	// As a side effect, this function records that the corresponding
	// glyph must be included in our subset of the font.
	Enc func(pdf.String, glyph.ID) pdf.String

	UnitsPerEm         uint16
	Ascent             funit.Int16
	Descent            funit.Int16 // negative
	BaseLineSkip       funit.Int16
	UnderlinePosition  funit.Int16
	UnderlineThickness funit.Int16

	GlyphExtents []funit.Rect
	Widths       []funit.Int16
}

// NumGlyphs returns the number of glyphs in a font.
func (font *Font) NumGlyphs() int {
	return len(font.Widths)
}

func (font *Font) ToPDF(fontSize float64, x funit.Int) float64 {
	return float64(x) * fontSize / float64(font.UnitsPerEm)
}

func (font *Font) ToPDF16(fontSize float64, x funit.Int16) float64 {
	return float64(x) * fontSize / float64(font.UnitsPerEm)
}

// Typeset computes all glyph and layout information required to typeset a
// string in a PDF file.
// TODO(voss): do we need this function?
// TODO(voss): return a structure like boxes.hGlyphs instead?
func (font *Font) Typeset(s string, ptSize float64) glyph.Seq {
	var runs [][]rune
	var run []rune
	for _, r := range s {
		if unicode.IsGraphic(r) || isPrivateRange(r) {
			run = append(run, r)
		} else if len(run) > 0 {
			runs = append(runs, run)
			run = nil
		}
	}
	if len(run) > 0 {
		runs = append(runs, run)
	}

	var glyphs []glyph.Info
	for _, run := range runs {
		seq := font.Layout(run)
		glyphs = append(glyphs, seq...)
	}
	return glyphs
}

func isPrivateRange(r rune) bool {
	return r >= '\uE000' && r <= '\uF8FF' ||
		r >= '\U000F0000' && r <= '\U000FFFFD' ||
		r >= '\U00100000' && r <= '\U0010FFFD'
}
