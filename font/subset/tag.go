// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package subset

import (
	"regexp"

	"seehuhn.de/go/sfnt/glyph"
)

const subsetModulus = 26 * 26 * 26 * 26 * 26 * 26

// Tag constructs a 6-letter tag (range AAAAAA to ZZZZZZ) to describe
// a subset of glyphs of a font.  This is used for the /BaseFont entry in PDF
// Font dictionaries and the /FontName entry in FontDescriptor dictionaries.
func Tag(origGid []glyph.ID, origNumGlyphs int) string {
	// mix all the information into a single uint32
	X := uint32(origNumGlyphs)
	for _, gid := range origGid {
		// 11 is the largest integer smaller than `1<<32 / subsetModulus` which
		// is relatively prime to 26.
		X = (X*11 + uint32(gid)) % subsetModulus
	}

	// convert to a string of six capital letters
	var buf [6]byte
	for i := range buf {
		buf[i] = 'A' + byte(X%26)
		X /= 26
	}
	return string(buf[:])
}

func IsValidTag(s string) bool {
	if len(s) != 6 {
		return false
	}

	for _, char := range s {
		if char < 'A' || char > 'Z' {
			return false
		}
	}

	return true
}

var TagRegexp = regexp.MustCompile(`^([A-Z]{6})\+(.*)$`)

func Join(tag, name string) string {
	if tag == "" {
		return name
	}
	return tag + "+" + name
}
