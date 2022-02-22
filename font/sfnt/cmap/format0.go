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

import (
	"fmt"

	"seehuhn.de/go/pdf/font"
)

// https://docs.microsoft.com/en-us/typography/opentype/spec/cmap#format-0-byte-encoding-table

type format0 struct {
	glyphIDArray [256]uint8
}

func decodeFormat0(data []byte) (Subtable, error) {
	data = data[6:]
	if len(data) != 256 {
		return nil, fmt.Errorf("cmap: format 0: expected 256 bytes, got %d", len(data))
	}
	res := &format0{}
	copy(res.glyphIDArray[:], data)
	return res, nil
}

func (cmap *format0) Lookup(code uint32) font.GlyphID {
	if code < 256 {
		return font.GlyphID(cmap.glyphIDArray[code])
	}
	return 0
}

func (cmap *format0) Encode(language uint16) []byte {
	return append([]byte{0, 0, 1, 6, byte(language >> 8), byte(language)},
		cmap.glyphIDArray[:]...)
}

func (cmap *format0) CodeRange() (low, high uint32) {
	for i, c := range cmap.glyphIDArray {
		if c == 0 {
			continue
		}
		if low == 0 {
			low = uint32(i)
		}
		high = uint32(i)
	}
	return
}
