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

package gtab

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

type LookupIndex uint16

type LookupListInfo []Lookup

type LookupInfo struct {
	LookupType       uint16
	LookupFlag       uint16
	MarkFilteringSet uint16
}

// Lookup represents a subtable of a "GSUB" or "GPOS" lookup table.
type Lookup interface {
	// Apply attempts to apply the subtable at the given position.
	// If returns the new glyphs and the new position.  If the subtable
	// cannot be applied, the unchanged glyphs and a negative position
	// are returned
	Apply(glyphs []font.Glyph, pos int) ([]font.Glyph, int)

	Encode() []byte
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-list-table
func readLookupList(p *parser.Parser, pos int64) (LookupListInfo, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	lookupCount, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}

	lookupOffsets := make([]uint16, lookupCount)
	for i := range lookupOffsets {
		lookupOffsets[i], err = p.ReadUInt16()
		if err != nil {
			return nil, err
		}
	}

	res := make([]Lookup, lookupCount)

	var subtableOffsets []uint16
	for i, offs := range lookupOffsets {
		lookupTablePos := pos + int64(offs)
		err := p.SeekPos(lookupTablePos)
		if err != nil {
			return nil, err
		}
		buf, err := p.ReadBlob(6)
		if err != nil {
			return nil, err
		}
		lookupType := uint16(buf[0])<<8 | uint16(buf[1])
		lookupFlag := uint16(buf[2])<<8 | uint16(buf[3])
		subTableCount := uint16(buf[4])<<8 | uint16(buf[5])
		subtableOffsets = subtableOffsets[:0]
		for j := 0; j < int(subTableCount); j++ {
			subtableOffset, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			}
			subtableOffsets = append(subtableOffsets, subtableOffset)
		}
		var markFilteringSet uint16
		if lookupFlag&0x0010 != 0 {
			markFilteringSet, err = p.ReadUInt16()
			if err != nil {
				return nil, err
			}
		}

		lookupInfo := &LookupInfo{
			LookupType:       lookupType,
			LookupFlag:       lookupFlag,
			MarkFilteringSet: markFilteringSet,
		}

		_ = i
		_ = lookupInfo
		panic("not implemented")
	}
	return res, nil
}
