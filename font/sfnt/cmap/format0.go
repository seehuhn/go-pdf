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
func decodeFormat0(data []byte, code2rune func(c int) rune) (Subtable, error) {
	if code2rune == nil {
		code2rune = unicode
	}

	data = data[6:]
	if len(data) != 256 {
		return nil, fmt.Errorf("cmap: format 0: expected 256 bytes, got %d", len(data))
	}

	res := make(Format4)
	for code, gid := range data {
		if gid == 0 {
			continue
		}
		r := code2rune(code)
		res[uint16(r)] = font.GlyphID(gid)
	}
	return res, nil
}
