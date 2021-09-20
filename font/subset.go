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

package font

import (
	"sort"
)

// CMapEntry describes the association between a character index and
// a glyph ID.
type CMapEntry struct {
	CID uint16
	GID GlyphID
}

// MakeSubset converts a mapping from a full font to a subsetted font.
// It also returns the list of original glyphs to include in the subset.
func MakeSubset(origMapping []CMapEntry) ([]CMapEntry, []GlyphID) {
	var newMapping []CMapEntry
	for _, m := range origMapping {
		if m.GID != 0 {
			newMapping = append(newMapping, m)
		}
	}
	sort.Slice(newMapping, func(i, j int) bool {
		return newMapping[i].CID < newMapping[j].CID
	})

	newToOrigGid := []GlyphID{0}
	for i, m := range newMapping {
		newGid := GlyphID(i + 1)
		newToOrigGid = append(newToOrigGid, m.GID)
		newMapping[i].GID = newGid
	}

	return newMapping, newToOrigGid
}

const subsetModulus = 26 * 26 * 26 * 26 * 26 * 26

// GetSubsetTag constructs a 6-letter tag (range AAAAAA to ZZZZZZ) to describe
// a subset of glyphs of a font.  This is used for the /BaseFont entry in PDF
// Font dictionaries and the /FontName entry in FontDescriptor dictionaries.
func GetSubsetTag(gg []GlyphID, numGlyphs int) string {
	sort.Slice(gg, func(i, j int) bool { return gg[i] < gg[j] })

	// mix all the information into a single uint32
	X := uint32(numGlyphs)
	for _, g := range gg {
		// 11 is the largest integer smaller than `1<<32 / subsetModulus` which
		// is relatively prime to 26.
		X = (X*11 + uint32(g)) % subsetModulus
	}

	// convert to a string of six capital letters
	var buf [6]byte
	for i := range buf {
		buf[i] = 'A' + byte(X%26)
		X /= 26
	}
	return string(buf[:])
}
