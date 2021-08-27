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

package parser

import "seehuhn.de/go/pdf/font"

// Lookups represents the information from a "GSUB" or "GPOS" table of a font.
type Lookups []*lookupTable

type lookupTable struct {
	subtables        []lookupSubtable
	filter           keepGlyphFn
	markFilteringSet uint16
	rtl              bool
}

type lookupSubtable interface {
	// Apply attempts to apply a single subtable at the given position.
	// If returns the new glyphs and the new position.  If the subtable
	// cannot be applied, the unchanged glyphs and a negative position
	// are returned
	Apply(filter keepGlyphFn, glyphs []font.Glyph, pos int) ([]font.Glyph, int)
}

// ApplyAll applies transformations from the selected lookup tables to a
// series of glyphs.
func (gtab Lookups) ApplyAll(glyphs []font.Glyph) []font.Glyph {
	for _, l := range gtab {
		pos := 0
		for pos < len(glyphs) {
			glyphs, pos = l.applySubtables(glyphs, pos)
		}
	}
	return glyphs
}

func (l *lookupTable) applySubtables(glyphs []font.Glyph, pos int) ([]font.Glyph, int) {
	for _, subtable := range l.subtables {
		glyphs, next := subtable.Apply(l.filter, glyphs, pos)
		if next >= 0 {
			return glyphs, next
		}
	}
	return glyphs, pos + 1
}

type lookupNotImplemented struct {
	table              string
	lookupType, format uint16
}

func (l *lookupNotImplemented) Apply(filter keepGlyphFn, glyphs []font.Glyph, pos int) ([]font.Glyph, int) {
	return glyphs, -1
}
