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

// KernInfo encodes the kerning for a pair of Glyphs.
// If Kern is less than zero, the character will be moved closer together.
type KernInfo struct {
	Left, Right font.GlyphID
	Kern        int16
}

// KerningAsLookup returns data from the "kern" table, converted to the form of
// a GPOS lookup.
func KerningAsLookup(kerning []KernInfo) Lookups {
	cov := make(Coverage)
	nextClass := 0
	var adjust []map[font.GlyphID]*pairAdjust
	for _, k := range kerning {
		if k.Kern == 0 {
			continue
		}

		class, ok := cov[k.Left]
		if !ok {
			class = nextClass
			adjust = append(adjust, map[font.GlyphID]*pairAdjust{})
			nextClass++

			cov[k.Left] = class
		}

		adjust[class][k.Right] = &pairAdjust{
			first: &valueRecord{XAdvance: k.Kern},
		}
	}

	subtable := &gpos2_1{
		cov:    cov,
		adjust: adjust,
	}
	lookup := &LookupTable{
		Subtables: []LookupSubtable{subtable},
		Filter:    useAllGlyphs,
	}
	return Lookups{lookup}
}
