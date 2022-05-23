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
	"sort"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
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

// Subtable represents a subtable of a "GSUB" or "GPOS" lookup table.
type Subtable interface {
	EncodeLen() int

	Encode() []byte

	// Apply attempts to apply the subtable at the given position.
	// If returns the new glyphs and the new position.  If the subtable
	// cannot be applied, the unchanged glyphs and a negative position
	// are returned
	Apply(keep KeepGlyphFn, seq []font.Glyph, a, b int) *Match
}

// Subtables is a slice of Subtable.
type Subtables []Subtable

// Apply tries the subtables one by one and applies the first one that
// matches.  If no subtable matches, the unchanged glyphs and a negative
// position are returned.
func (ss Subtables) Apply(keep KeepGlyphFn, seq []font.Glyph, pos, b int) *Match {
	for _, subtable := range ss {
		match := subtable.Apply(keep, seq, pos, b)
		if match != nil {
			return match
		}
	}
	return nil
}

// subtableReader is a function that can decode a subtable.
// Different functions are required for "GSUB" and "GPOS" tables.
type subtableReader func(*parser.Parser, int64, *LookupMetaInfo) (Subtable, error)

func readLookupList(p *parser.Parser, pos int64, sr subtableReader) (LookupList, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	lookupOffsets, err := p.ReadUint16Slice()
	if err != nil {
		return nil, err
	}

	res := make(LookupList, len(lookupOffsets))

	numLookups := 0
	numSubTables := 0

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
		numLookups++
		numSubTables += int(subTableCount)
		if numLookups+numSubTables > 6000 {
			// The condition ensures that we can always store the lookup
			// data (using extension subtables if necessary), without
			// exceeding the maximum offset size in the lookup list table.
			return nil, &font.InvalidFontError{
				SubSystem: "sfnt/opentype/gtab",
				Reason:    "too many lookup (sub-)tables",
			}
		}
		subtableOffsets = subtableOffsets[:0]
		for j := 0; j < int(subTableCount); j++ {
			subtableOffset, err := p.ReadUint16()
			if err != nil {
				return nil, err
			}
			subtableOffsets = append(subtableOffsets, subtableOffset)
		}
		var markFilteringSet uint16
		if lookupFlag&LookupUseMarkFilteringSet != 0 {
			markFilteringSet, err = p.ReadUint16()
			if err != nil {
				return nil, err
			}
		}

		meta := &LookupMetaInfo{
			LookupType:       lookupType,
			LookupFlag:       lookupFlag,
			MarkFilteringSet: markFilteringSet,
		}

		subtables := make(Subtables, subTableCount)
		for j, subtableOffset := range subtableOffsets {
			subtable, err := sr(p, lookupTablePos+int64(subtableOffset), meta)
			if err != nil {
				return nil, err
			}
			subtables[j] = subtable
		}

		if tp, ok := isExtension(subtables); ok {
			if tp == meta.LookupType {
				return nil, &font.InvalidFontError{
					SubSystem: "sfnt/opentype/gtab",
					Reason:    "invalid extension subtable",
				}
			}
			meta.LookupType = tp
			for j, subtable := range subtables {
				l, ok := subtable.(*extensionSubtable)
				if !ok || l.ExtensionLookupType != tp {
					return nil, &font.InvalidFontError{
						SubSystem: "sfnt/opentype/gtab",
						Reason:    "inconsistent extension subtables",
					}
				}
				pos := lookupTablePos + int64(subtableOffsets[j]) + l.ExtensionOffset
				subtable, err := sr(p, pos, meta)
				if err != nil {
					return nil, err
				}
				subtables[j] = subtable
			}
		}

		res[i] = &LookupTable{
			Meta:      meta,
			Subtables: subtables,
		}
	}
	return res, nil
}

func isExtension(ss Subtables) (uint16, bool) {
	if len(ss) == 0 {
		return 0, false
	}
	l, ok := ss[0].(*extensionSubtable)
	if !ok {
		return 0, false
	}
	return l.ExtensionLookupType, true
}

func (info LookupList) encode() []byte {
	if info == nil {
		return nil
	}

	lookupCount := len(info)

	var chunks []layoutChunk
	chunks = append(chunks, layoutChunk{
		size: 2 + 2*uint32(lookupCount),
	})
	for i, l := range info {
		lookupHeaderLen := 6 + 2*len(l.Subtables)
		if l.Meta.LookupFlag&LookupUseMarkFilteringSet != 0 {
			lookupHeaderLen += 2
		}
		lCode := (uint32(i) & 0x3FFF) << 14
		chunks = append(chunks, layoutChunk{
			code: 1<<28 | lCode,
			size: uint32(lookupHeaderLen),
		})
		for j, subtable := range l.Subtables {
			sCode := uint32(j) & 0x3FFF
			chunks = append(chunks, layoutChunk{
				code: 2<<28 | lCode | sCode,
				size: uint32(subtable.EncodeLen()),
			})
		}
	}

	chunkPos := make(map[uint32]uint32, len(chunks))
	total := uint32(0)
	isTooLarge := false
	for i := range chunks {
		code := chunks[i].code
		if code>>28 == 1 && total > 0xFFFF {
			isTooLarge = true
			break
		}
		chunkPos[code] = total
		total += chunks[i].size
	}

	if isTooLarge {
		// reorder chunks and use extension records as needed.
		chunks = info.tryReorder(chunks)
	}

	buf := make([]byte, 0, total)
	for k := range chunks {
		code := chunks[k].code
		if chunkPos[code] != uint32(len(buf)) {
			panic("internal error")
		}
		switch code >> 28 {
		case 0: // LookupList table
			buf = append(buf, byte(lookupCount>>8), byte(lookupCount))
			for i := range info {
				lCode := (uint32(i) & 0x3FFF) << 14
				lookupOffset := chunkPos[1<<28|lCode]
				buf = append(buf, byte(lookupOffset>>8), byte(lookupOffset))
			}
		case 1: // Lookup table
			lCode := code & 0x0FFFC000
			i := int(lCode >> 14)
			li := info[i]
			subTableCount := len(li.Subtables)
			buf = append(buf,
				byte(li.Meta.LookupType>>8), byte(li.Meta.LookupType),
				byte(li.Meta.LookupFlag>>8), byte(li.Meta.LookupFlag),
				byte(subTableCount>>8), byte(subTableCount),
			)
			base := chunkPos[code]
			for j := range li.Subtables {
				sCode := uint32(j) & 0x3FFF
				subtablePos := chunkPos[2<<28|lCode|sCode]
				subtableOffset := subtablePos - base
				buf = append(buf, byte(subtableOffset>>8), byte(subtableOffset))
			}
			if li.Meta.LookupFlag&LookupUseMarkFilteringSet != 0 {
				buf = append(buf,
					byte(li.Meta.MarkFilteringSet>>8), byte(li.Meta.MarkFilteringSet),
				)
			}
		case 2: // lookup subtable
			i := int((code >> 14) & 0x3FFF)
			j := int(code & 0x3FFF)
			subtable := info[i].Subtables[j]
			buf = append(buf, subtable.Encode()...)
		}
	}
	return buf
}

type layoutChunk struct {
	code uint32
	size uint32
}

func (info LookupList) tryReorder(chunks []layoutChunk) []layoutChunk {
	lookupSize := make(map[uint32]uint32)

	type lInfo struct {
		lCode uint32
		size  uint32
	}
	var lookups []lInfo

	total := uint32(0)
	for i := range chunks {
		total += chunks[i].size
		code := chunks[i].code
		if code == 0 {
			continue
		}
		lCode := (code >> 14) & 0x3FFF
		lookupSize[lCode] += chunks[i].size
		if code>>28 == 1 {
			lookups = append(lookups, lInfo{lCode: lCode})
		}
	}

	for i := range lookups {
		lookups[i].size = lookupSize[lookups[i].lCode]
	}
	sort.Slice(lookups, func(i, j int) bool {
		if lookups[i].size != lookups[j].size {
			return lookups[i].size > lookups[j].size
		}
		return lookups[i].lCode > lookups[j].lCode
	})

	replace := make(map[uint32]bool)
	largestLookup := lookups[0].lCode
	largestOffset := total - lookups[0].size
	pos := 1
	for largestOffset > 0xFFFF {
		l := info[lookups[pos].lCode]
		lookupHeaderLen := 6 + 2*len(l.Subtables)
		if l.Meta.LookupFlag&LookupUseMarkFilteringSet != 0 {
			lookupHeaderLen += 2
		}

		oldSize := lookups[pos].size
		newSize := uint32(lookupHeaderLen) + 8*uint32(len(l.Subtables))
		fmt.Println("*", largestOffset, oldSize, newSize)
		if newSize < oldSize {
			replace[lookups[pos].lCode] = true
			largestOffset -= oldSize - newSize
		}

		pos++
	}

	for i := range info {
		li := uint32(i)
		fmt.Println(i, lookupSize[li], replace[li], li == largestLookup)
	}

	panic("not implemented") // TODO(voss): finish this
}

// Extension Substitution Subtable Format 1
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#71-extension-substitution-subtable-format-1
type extensionSubtable struct {
	ExtensionLookupType uint16
	ExtensionOffset     int64
}

func readExtensionSubtable(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(6)
	if err != nil {
		return nil, err
	}
	extensionLookupType := uint16(buf[0])<<8 | uint16(buf[1])
	extensionOffset := int64(buf[2])<<24 | int64(buf[3])<<16 | int64(buf[4])<<8 | int64(buf[5])
	res := &extensionSubtable{
		ExtensionLookupType: extensionLookupType,
		ExtensionOffset:     extensionOffset,
	}
	return res, nil
}

func (l *extensionSubtable) Apply(KeepGlyphFn, []font.Glyph, int, int) *Match {
	panic("unreachable")
}

func (l *extensionSubtable) EncodeLen() int {
	return 8
}

func (l *extensionSubtable) Encode() []byte {
	return []byte{
		0, 1, // format
		byte(l.ExtensionLookupType >> 8), byte(l.ExtensionLookupType),
		byte(l.ExtensionOffset >> 24), byte(l.ExtensionOffset >> 16), byte(l.ExtensionOffset >> 8), byte(l.ExtensionOffset),
	}
}
