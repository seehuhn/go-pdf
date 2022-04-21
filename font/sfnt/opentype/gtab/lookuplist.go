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

// LookupMetaInfo contains information associated with a lookup but not
// specific to a subtable.
type LookupMetaInfo struct {
	LookupType       uint16
	LookupFlag       LookupFlags
	MarkFilteringSet uint16
}

// LookupFlags contains bits which modify application of a lookup to a glyph string.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookupFlags
type LookupFlags uint16

// Bit values for LookupFlag.
const (
	LookupRightToLeft         LookupFlags = 0x0001
	LookupIgnoreBaseGlyphs    LookupFlags = 0x0002
	LookupIgnoreLigatures     LookupFlags = 0x0004
	LookupIgnoreMarks         LookupFlags = 0x0008
	LookupUseMarkFilteringSet LookupFlags = 0x0010
	LookupMarkAttachTypeMask  LookupFlags = 0xFF00
)

// LookupIndex enumerates lookups.
// It is used as an index into a LookupList.
type LookupIndex uint16

// LookupList contains the information from a Lookup List Table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-list-table
type LookupList []*LookupTable

// LookupTable represents a lookup table inside a "GSUB" or "GPOS" table of a
// font.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-table
type LookupTable struct {
	Meta      *LookupMetaInfo
	Subtables Subtables
}

// EncodeLen returns the number of bytes required to encode the LookupTable.
func (li *LookupTable) EncodeLen() int {
	total := 6
	total += 2 * len(li.Subtables)
	if li.Meta.LookupFlag&0x0010 != 0 {
		total += 2
	}
	for _, subtable := range li.Subtables {
		total += subtable.EncodeLen()
	}
	return total
}

// Subtable represents a subtable of a "GSUB" or "GPOS" lookup table.
type Subtable interface {
	// Apply attempts to apply the subtable at the given position.
	// If returns the new glyphs and the new position.  If the subtable
	// cannot be applied, the unchanged glyphs and a negative position
	// are returned
	Apply(KeepGlyphFn, []font.Glyph, int) ([]font.Glyph, int, Nested)

	EncodeLen() int

	Encode() []byte
}

// Subtables is a slice of Subtable.
type Subtables []Subtable

// Apply tries the subtables one by one and applies the first one that
// matches.  If no subtable matches, the unchanged glyphs and a negative
// position are returned.
func (ss Subtables) Apply(keep KeepGlyphFn, seq []font.Glyph, pos int) ([]font.Glyph, int, Nested) {
	for _, subtable := range ss {
		newSeq, newPos, nested := subtable.Apply(keep, seq, pos)
		if newPos >= 0 {
			return newSeq, newPos, nested
		}
	}
	return seq, -1, nil
}

// subtableReader is a function that can decode a subtable.
// Different functions are required for "GSUB" and "GPOS" tables.
type subtableReader func(*parser.Parser, int64, *LookupMetaInfo) (Subtable, error)

func readLookupList(p *parser.Parser, pos int64, sr subtableReader) (LookupList, error) {
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

	res := make(LookupList, lookupCount)

	var subtableOffsets []uint16
	for i, offs := range lookupOffsets {
		lookupTablePos := pos + int64(offs)
		err := p.SeekPos(lookupTablePos)
		if err != nil {
			return nil, err
		}
		buf, err := p.ReadBytes(6)
		if err != nil {
			return nil, err
		}
		lookupType := uint16(buf[0])<<8 | uint16(buf[1])
		lookupFlag := LookupFlags(buf[2])<<8 | LookupFlags(buf[3])
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
		if lookupFlag&LookupUseMarkFilteringSet != 0 {
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

		subTables := make(Subtables, subTableCount)
		for j, subtableOffset := range subtableOffsets {
			subtable, err := sr(p, lookupTablePos+int64(subtableOffset), meta)
			if err != nil {
				return nil, err
			}
			subTables[j] = subtable
		}

		res[i] = &LookupTable{
			Meta:      meta,
			Subtables: subTables,
		}
	}
	return res, nil
}

func (info LookupList) encode() []byte {
	if info == nil {
		return nil
	}

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

	for _, li := range info {
		subTableCount := len(li.Subtables)
		res = append(res,
			byte(li.Meta.LookupType>>8), byte(li.Meta.LookupType),
			byte(li.Meta.LookupFlag>>8), byte(li.Meta.LookupFlag),
			byte(subTableCount>>8), byte(subTableCount))

		stPos := 6
		stPos += 2 * subTableCount
		if li.Meta.LookupFlag&LookupUseMarkFilteringSet != 0 {
			stPos += 2
		}
		for _, st := range li.Subtables {
			res = append(res, byte(stPos>>8), byte(stPos))
			stPos += st.EncodeLen()
		}
		if li.Meta.LookupFlag&LookupUseMarkFilteringSet != 0 {
			res = append(res,
				byte(li.Meta.MarkFilteringSet>>8), byte(li.Meta.MarkFilteringSet))
		}
		for _, st := range li.Subtables {
			res = append(res, st.Encode()...)
		}
	}
	return res
}
