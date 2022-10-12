// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package coverage

import (
	"fmt"
	"sort"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/pdf/sfnt/glyph"
	"seehuhn.de/go/pdf/sfnt/parser"
)

// Set is a coverage table, but with the coverage indices omitted.
type Set map[glyph.ID]bool

// Glyphs returned the glyphs covered by the Set, in order of increasing glyph ID.
func (set Set) Glyphs() []glyph.ID {
	glyphs := maps.Keys(set)
	sort.Slice(glyphs, func(i, j int) bool { return glyphs[i] < glyphs[j] })
	return glyphs
}

// ToTable converts the Set to a Coverage table.
func (set Set) ToTable() Table {
	glyphs := set.Glyphs()
	table := make(Table)
	for i, gid := range glyphs {
		table[gid] = i
	}
	return table
}

// ReadSet reads a coverage table from a parser.
// This function allows for some duplicate glyphs to be included.  This is
// forbidden by the spec, but occurs in some widely used fonts.
func ReadSet(p *parser.Parser, pos int64) (Set, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	format, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}

	table := make(Set)

	switch format {
	case 1: // Coverage Format 1
		glyphCount, err := p.ReadUint16()
		if err != nil {
			return nil, err
		}
		for i := 0; i < int(glyphCount); i++ {
			gid, err := p.ReadUint16()
			if err != nil {
				return nil, err
			}
			table[glyph.ID(gid)] = true
		}

	case 2: // Coverage Format 2
		rangeCount, err := p.ReadUint16()
		if err != nil {
			return nil, err
		}
		pos := 0
		prev := -1
		for i := 0; i < int(rangeCount); i++ {
			buf, err := p.ReadBytes(6)
			if err != nil {
				return nil, err
			}
			startGlyphID := int(buf[0])<<8 | int(buf[1])
			endGlyphID := int(buf[2])<<8 | int(buf[3])
			startCoverageIndex := int(buf[4])<<8 | int(buf[5])
			if startCoverageIndex != pos ||
				startGlyphID < prev ||
				endGlyphID < startGlyphID {
				// Some fonts list individual glyphs twice.  To cover most of
				// these cases, we allow startGlyphID to be equal to prev.
				return nil, &parser.InvalidFontError{
					SubSystem: "sfnt/opentype/coverage",
					Reason:    "invalid coverage table (format 2)",
				}
			}
			for gid := startGlyphID; gid <= endGlyphID; gid++ {
				table[glyph.ID(gid)] = true
				pos++
			}
			prev = endGlyphID
		}

	default:
		return nil, &parser.NotSupportedError{
			SubSystem: "sfnt/opentype/coverage",
			Feature:   fmt.Sprintf("coverage format %d", format),
		}
	}

	return table, nil
}
