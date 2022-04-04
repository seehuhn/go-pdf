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
	"fmt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

type LookupIndex uint16

type LookupListInfo []*LookupInfo

type LookupInfo struct {
	Meta      *LookupMetaInfo
	SubTables []Subtable
}

func (li *LookupInfo) EncodeLen() int {
	total := 6
	total += 2 * len(li.SubTables)
	if li.Meta.LookupFlag&0x0010 != 0 {
		total += 2
	}
	for _, subtable := range li.SubTables {
		total += subtable.EncodeLen(li.Meta)
	}
	return total
}

type LookupMetaInfo struct {
	LookupType       uint16
	LookupFlag       uint16
	MarkFilteringSet uint16
}

// Subtable represents a subtable of a "GSUB" or "GPOS" lookup table.
type Subtable interface {
	// Apply attempts to apply the subtable at the given position.
	// If returns the new glyphs and the new position.  If the subtable
	// cannot be applied, the unchanged glyphs and a negative position
	// are returned
	Apply(*LookupMetaInfo, []font.Glyph, int) ([]font.Glyph, int)

	EncodeLen(*LookupMetaInfo) int

	Encode(*LookupMetaInfo) []byte
}

type SubtableReader func(*parser.Parser, int64, *LookupMetaInfo) (Subtable, error)

// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-list-table
func readLookupList(p *parser.Parser, pos int64, sr SubtableReader) (LookupListInfo, error) {
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

	res := make(LookupListInfo, lookupCount)

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

		meta := &LookupMetaInfo{
			LookupType:       lookupType,
			LookupFlag:       lookupFlag,
			MarkFilteringSet: markFilteringSet,
		}

		subTables := make([]Subtable, subTableCount)
		for j, subtableOffset := range subtableOffsets {
			subtable, err := sr(p, lookupTablePos+int64(subtableOffset), meta)
			if err != nil {
				return nil, err
			}
			subTables[j] = subtable
		}

		res[i] = &LookupInfo{
			Meta:      meta,
			SubTables: subTables,
		}
	}
	return res, nil
}

func (info LookupListInfo) encode() []byte {
	lookupCount := len(info)

	lookupOffsets := make([]int, lookupCount)
	pos := 2 + 2*lookupCount
	for i, li := range info {
		lookupOffsets[i] = pos
		pos += li.EncodeLen()
	}

	res := make([]byte, 0, pos)
	res = append(res, byte(lookupCount>>8), byte(lookupCount))
	for i := range info {
		res = append(res, byte(lookupOffsets[i]>>8), byte(lookupOffsets[i]))
	}

	for i, li := range info {
		if len(res) != lookupOffsets[i] { // TODO(voss): remove
			fmt.Println(lookupOffsets)
			fmt.Println(i, len(res), lookupOffsets[i])
			panic("internal error")
		}
		subTableCount := len(li.SubTables)
		res = append(res,
			byte(li.Meta.LookupType>>8), byte(li.Meta.LookupType),
			byte(li.Meta.LookupFlag>>8), byte(li.Meta.LookupFlag),
			byte(subTableCount>>8), byte(subTableCount))

		stPos := 6
		stPos += 2 * subTableCount
		if li.Meta.LookupFlag&0x0010 != 0 {
			stPos += 2
		}
		for _, st := range li.SubTables {
			res = append(res, byte(stPos>>8), byte(stPos))
			stPos += st.EncodeLen(li.Meta)
		}
		if li.Meta.LookupFlag&0x0010 != 0 {
			res = append(res,
				byte(li.Meta.MarkFilteringSet>>8), byte(li.Meta.MarkFilteringSet))
		}
		for _, st := range li.SubTables {
			res = append(res, st.Encode(li.Meta)...)
		}
	}
	return res
}
