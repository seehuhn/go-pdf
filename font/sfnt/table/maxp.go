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

package table

import (
	"errors"
	"io"
)

type MaxpInfo struct {
	NumGlyphs int
}

// ReadMaxp reads the number of Glyphs from the "maxp" table.
// All other information is ignored.
func ReadMaxp(r io.Reader) (*MaxpInfo, error) {
	var buf [6]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return nil, err
	}

	version := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
	if version != 0x00005000 && version != 0x00010000 {
		return nil, errors.New("sfnt/maxp: unknown version")
	}

	numGlyphs := int(buf[4])<<8 | int(buf[5])
	if numGlyphs == 0 {
		return nil, errors.New("sfnt/maxp: numGlyphs is zero")
	}

	return &MaxpInfo{numGlyphs}, nil
}

// Encode encodes the number of Glyphs in a "maxp" table.
func (info *MaxpInfo) Encode() ([]byte, error) {
	numGlyphs := info.NumGlyphs
	if numGlyphs < 1 || numGlyphs >= 1<<16 {
		return nil, errors.New("sfnt/maxp: numGlyphs out of range")
	}
	return []byte{0x00, 0x00, 0x50, 0x00, byte(numGlyphs >> 8), byte(numGlyphs)}, nil
}
