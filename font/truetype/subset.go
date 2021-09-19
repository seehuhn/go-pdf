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

package truetype

import (
	"sort"

	"seehuhn.de/go/pdf/font"
)

const subsetModulus = 26 * 26 * 26 * 26 * 26 * 26

func getSubsetTag(gg []font.GlyphID, numGlyphs uint16) string {
	sort.Slice(gg, func(i, j int) bool { return gg[i] < gg[j] })

	// mix all the information into a single uint32
	X := uint32(numGlyphs)
	for _, g := range gg {
		// 11 is the largest integer smaller than 1<<32 / subsetModulus which
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
