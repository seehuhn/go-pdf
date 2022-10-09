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

// Package kern has code for reading and writing the "kern" table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/kern
package kern

import (
	"bytes"
	"fmt"
	"math/bits"
	"sort"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/funit"
	"seehuhn.de/go/pdf/font/glyph"
	"seehuhn.de/go/pdf/font/parser"
)

// Info contains information from the "kern" table.
// If the value for a glyph pair is greater than zero, the characters will be moved apart.
// If the value is less than zero, the character will be moved closer together.
// https://docs.microsoft.com/en-us/typography/opentype/spec/kern
type Info map[glyph.Pair]funit.Int16

// Read reads the "kern" table.
func Read(r parser.ReadSeekSizer) (Info, error) {
	p := parser.New("kern", r)

	version, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	if version != 0 {
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/kern",
			Feature:   fmt.Sprintf("\"kern\" table version %d", version),
		}
	}

	nTables, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}

	res := make(Info)

	pos := p.Pos()
	for i := 0; i < int(nTables); i++ {
		err := p.SeekPos(pos)
		if err != nil {
			return nil, err
		}
		buf, err := p.ReadBytes(6)
		if err != nil {
			return nil, err
		}
		subtableVersion := uint16(buf[0])<<8 | uint16(buf[1])
		length := uint16(buf[2])<<8 | uint16(buf[3])
		format := buf[4]
		flags := buf[5]

		if length < 6+8 {
			return nil, &font.InvalidFontError{
				SubSystem: "sfnt/kern",
				Reason:    fmt.Sprintf("invalid kern subtable length %d", length),
			}
		}
		pos += int64(length)

		if subtableVersion != 0 || format != 0 || flags&0b11110101 != 1 {
			continue
		}
		isMinimum := flags&0b00000010 != 0
		isOverride := flags&0b00001000 != 0

		nPairs, err := p.ReadUint16()
		if err != nil {
			return nil, err
		}
		err = p.Discard(6) // skip searchRange, entrySelector and rangeShift
		if err != nil {
			return nil, err
		}
		for j := 0; j < int(nPairs); j++ {
			buf, err := p.ReadBytes(6)
			if err != nil {
				return nil, err
			}
			left := glyph.ID(buf[0])<<8 | glyph.ID(buf[1])
			right := glyph.ID(buf[2])<<8 | glyph.ID(buf[3])
			value := funit.Int16(buf[4])<<8 | funit.Int16(buf[5])
			key := glyph.Pair{Left: left, Right: right}
			if isMinimum {
				if res[key] < value {
					res[key] = value
				}
			} else if isOverride {
				res[key] = value
			} else {
				res[key] += value
			}
		}
	}

	return res, nil
}

// Encode converts the "kern" table to its binary representation.
func (info Info) Encode() []byte {
	nPairs := len(info)
	headerLen := 4
	subHeaderLen := 14
	subTableLen := subHeaderLen + 6*nPairs
	buf := make([]byte, 0, headerLen+subTableLen)

	var entrySelector, searchRange, rangeShift int
	if nPairs > 0 {
		entrySelector = bits.Len(uint(nPairs)) - 1
		searchRange = 6 * (1 << entrySelector)
		rangeShift = 6 * (nPairs - 1<<entrySelector)
	}
	buf = append(buf,
		0, 0, // version
		0, 1, // numTables

		0, 0, // subtable version
		byte(subTableLen>>8), byte(subTableLen),
		0, 1, // coverage

		byte(nPairs>>8), byte(nPairs),
		byte(searchRange>>8), byte(searchRange),
		byte(entrySelector>>8), byte(entrySelector),
		byte(rangeShift>>8), byte(rangeShift),
	)
	for pair, val := range info {
		buf = append(buf,
			byte(pair.Left>>8), byte(pair.Left),
			byte(pair.Right>>8), byte(pair.Right),
			byte(val>>8), byte(val),
		)
	}
	sort.Sort(blocks(buf[headerLen+subHeaderLen:]))

	return buf
}

type blocks []byte

func (a blocks) Len() int { return len(a) / 6 }
func (a blocks) Swap(i, j int) {
	var tmp [6]byte
	copy(tmp[:], a[i*6:])
	copy(a[i*6:], a[j*6:(j+1)*6])
	copy(a[j*6:], tmp[:])
}
func (a blocks) Less(i, j int) bool {
	return bytes.Compare(a[i*6:(i+1)*6], a[j*6:(j+1)*6]) < 0
}
