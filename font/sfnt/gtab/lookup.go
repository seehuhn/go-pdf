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

package gtab

import "seehuhn.de/go/pdf/font"

// Lookups represents the information from a "GSUB" or "GPOS" table of a font.
type Lookups []*OldLookupTable

// OldLookupTable represents a lookup table inside a "GSUB" or "GPOS" table of a
// font.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-table
type OldLookupTable struct {
	Subtables []LookupSubtable
	Filter    KeepGlyphFn

	markFilteringSet uint16 // TODO(voss): use this or remove this?
	rtl              bool   // TODO(voss): use this or remove this?
}

// LookupSubtable represents a subtable of a "GSUB" or "GPOS" lookup table.
type LookupSubtable interface {
	// Apply attempts to apply the subtable at the given position.
	// If returns the new glyphs and the new position.  If the subtable
	// cannot be applied, the unchanged glyphs and a negative position
	// are returned
	Apply(filter KeepGlyphFn, glyphs []font.Glyph, pos int) ([]font.Glyph, int)
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

func (l *OldLookupTable) applySubtables(glyphs []font.Glyph, pos int) ([]font.Glyph, int) {
	for _, subtable := range l.Subtables {
		glyphs, next := subtable.Apply(l.Filter, glyphs, pos)
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

func (l *lookupNotImplemented) Apply(filter KeepGlyphFn, glyphs []font.Glyph, pos int) ([]font.Glyph, int) {
	return glyphs, -1
}
