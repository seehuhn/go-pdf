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

package cmap

import "seehuhn.de/go/pdf/font"

// Subtable represents a decoded cmap subtable.
type Subtable interface {
	Lookup(r rune) font.GlyphID

	// Encode returns the binary form of the subtable.
	Encode(language uint16) []byte

	// CodeRange returns the smallest and largest code point in the subtable.
	CodeRange() (low, high rune)
}

// From the font files on my laptop, I extracted all cmap subtables
// and removed duplicates.  The following table is the result.
//
//    count | format |
//   -------+--------+-----------------------------------
//     1668 |    4   | Segment mapping to delta values
//      625 |    6   | Trimmed table mapping
//      554 |   12   | Segmented coverage
//      226 |    0   | Byte encoding table
//       54 |   14   | Unicode Variation Sequences
//       47 |    2   | High-byte mapping through table
//        2 |   10   | Trimmed array
//        1 |    8   | mixed 16-bit and 32-bit coverage
//        1 |   13   | Many-to-one range mappings

var decoders = map[uint16]func([]byte, func(int) rune) (Subtable, error){
	0:  decodeFormat0,
	2:  notImplemented,
	4:  decodeFormat4,
	6:  decodeFormat6,
	8:  notImplemented,
	10: notImplemented,
	12: decodeFormat12,
	13: notImplemented,
	14: notImplemented,
}

func notImplemented([]byte, func(int) rune) (Subtable, error) {
	return nil, errUnsupportedCmapFormat
}

func unicode(code int) rune {
	return rune(code)
}
