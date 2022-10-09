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

import "seehuhn.de/go/pdf/font/glyph"

func decodeFormat6(data []byte, code2rune func(c int) rune) (Subtable, error) {
	if code2rune == nil {
		code2rune = unicode
	}

	if len(data) < 10 {
		return nil, errMalformedSubtable
	}
	firstCode := int(data[6])<<8 | int(data[7])
	count := int(data[8])<<8 | int(data[9])

	// some fonts have an excess 0x0000 at the end of the table
	if len(data) == 10+2*count+2 && data[10+2*count] == 0 && data[10+2*count+1] == 0 {
		data = data[:10+2*count]
	}

	if len(data) != 10+2*count {
		return nil, errMalformedSubtable
	}
	data = data[10:]

	res := make(Format4)
	for i := 0; i < count; i++ {
		gid := glyph.ID(data[2*i])<<8 | glyph.ID(data[2*i+1])
		if gid != 0 {
			res[uint16(code2rune(i+firstCode))] = gid
		}
	}
	return res, nil
}
