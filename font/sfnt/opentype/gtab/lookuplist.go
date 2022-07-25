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
	"sort"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/locale"
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

type chunkCode uint32

const (
	chunkHeader chunkCode = iota << 28
	chunkTable
	chunkSubtable
	chunkExtReplace

	chunkTypeMask     chunkCode = 0b1111_00000000000000_00000000000000
	chunkTableMask    chunkCode = 0b0000_11111111111111_00000000000000
	chunkSubtableMask chunkCode = 0b0000_00000000000000_11111111111111
)

func (info LookupList) encode(extLookupType uint16) []byte {
	if info == nil {
		return nil
	}

	lookupCount := len(info)
	if lookupCount >= 1<<14 {
		panic("too many lookup tables")
	}

	// Make a list of all chunks which need to be written.
	var chunks []layoutChunk
	chunks = append(chunks, layoutChunk{
		code: chunkHeader,
		size: 2 + 2*uint32(lookupCount),
	})
	for i, l := range info {
		lookupHeaderLen := 6 + 2*len(l.Subtables)
		if l.Meta.LookupFlag&LookupUseMarkFilteringSet != 0 {
			lookupHeaderLen += 2
		}
		tCode := chunkCode(i) << 14
		chunks = append(chunks, layoutChunk{
			code: chunkTable | tCode,
			size: uint32(lookupHeaderLen),
		})
		if len(l.Subtables) >= 1<<14 {
			panic("too many subtables")
		}
		for j, subtable := range l.Subtables {
			sCode := chunkCode(j)
			chunks = append(chunks, layoutChunk{
				code: chunkSubtable | tCode | sCode,
				size: uint32(subtable.EncodeLen()),
			})
		}
	}

	// If needed, reorder the chunks or introduce extension records.
	isTooLarge := false
	var total uint32
	for i := range chunks {
		code := chunks[i].code
		if code&chunkTypeMask == chunkTable && total > 0xFFFF {
			isTooLarge = true
			break
		}
		total += chunks[i].size
	}
	if isTooLarge {
		chunks = info.tryReorder(chunks)
	}

	// Layout the chunks.
	chunkPos := make(map[chunkCode]uint32, len(chunks))
	total = 0
	for i := range chunks {
		code := chunks[i].code
		chunkPos[code] = total
		total += chunks[i].size
	}

	// Construct the lookup table in memory.
	buf := make([]byte, 0, total)
	for k := range chunks {
		code := chunks[k].code
		if chunkPos[code] != uint32(len(buf)) { // TODO(voss): remove?
			panic("internal error")
		}
		switch code & chunkTypeMask {
		case chunkHeader:
			buf = append(buf, byte(lookupCount>>8), byte(lookupCount))
			for i := range info {
				tCode := chunkCode(i) << 14
				lookupOffset := chunkPos[chunkTable|tCode]
				buf = append(buf, byte(lookupOffset>>8), byte(lookupOffset))
			}
		case chunkTable:
			tCode := code & chunkTableMask
			i := int(tCode) >> 14
			li := info[i]
			subTableCount := len(li.Subtables)
			lookupType := li.Meta.LookupType
			if _, replaced := chunkPos[chunkExtReplace|tCode]; replaced {
				// fix the lookup type in case of replaced subtables
				lookupType = extLookupType
			}
			buf = append(buf,
				byte(lookupType>>8), byte(lookupType),
				byte(li.Meta.LookupFlag>>8), byte(li.Meta.LookupFlag),
				byte(subTableCount>>8), byte(subTableCount),
			)
			base := chunkPos[code]
			for j := range li.Subtables {
				sCode := chunkCode(j)
				subtablePos, replaced := chunkPos[chunkExtReplace|tCode|sCode]
				if !replaced {
					subtablePos = chunkPos[chunkSubtable|tCode|sCode]
				}
				subtableOffset := subtablePos - base
				buf = append(buf, byte(subtableOffset>>8), byte(subtableOffset))
			}
			if li.Meta.LookupFlag&LookupUseMarkFilteringSet != 0 {
				buf = append(buf,
					byte(li.Meta.MarkFilteringSet>>8), byte(li.Meta.MarkFilteringSet),
				)
			}
		case chunkExtReplace:
			tCode := code & chunkTableMask
			sCode := code & chunkSubtableMask
			lookup := info[tCode>>14]
			pos := chunkPos[code]
			extPos := chunkPos[chunkSubtable|tCode|sCode]
			subtable := &extensionSubtable{
				ExtensionLookupType: lookup.Meta.LookupType,
				ExtensionOffset:     int64(extPos - pos),
			}
			buf = append(buf, subtable.Encode()...)
		case chunkSubtable:
			i := code & chunkTableMask >> 14
			j := code & chunkSubtableMask
			subtable := info[i].Subtables[j]
			buf = append(buf, subtable.Encode()...)
		}
	}
	return buf
}

type layoutChunk struct {
	code chunkCode
	size uint32
}

func (info LookupList) tryReorder(chunks []layoutChunk) []layoutChunk {
	total := uint32(0)
	for i := range chunks {
		total += chunks[i].size
	}

	lookupSize := make(map[chunkCode]uint32)
	var lookups []chunkCode
	for i := range chunks {
		code := chunks[i].code
		tp := code & chunkTypeMask
		tCode := code & chunkTableMask
		if tp == chunkHeader {
			continue
		} else if tp == chunkTable {
			lookups = append(lookups, tCode)
		}
		lookupSize[tCode] += chunks[i].size
	}
	sort.SliceStable(lookups, func(i, j int) bool {
		return lookupSize[lookups[i]] < lookupSize[lookups[j]]
	})

	// Move the largest table to the end and introduce extension subtables
	// as needed.
	biggestLookup := lookups[len(lookups)-1]
	lastPos := total - lookupSize[biggestLookup]
	idx := len(lookups) - 2
	replace := make(map[chunkCode]bool)
	extra := 0
	for lastPos > 0xFFFF && idx >= 0 {
		tCode := lookups[idx]

		oldSize := lookupSize[tCode]
		l := info[tCode>>14]
		lookupHeaderLen := 6 + 2*len(l.Subtables)
		if l.Meta.LookupFlag&LookupUseMarkFilteringSet != 0 {
			lookupHeaderLen += 2
		}
		newSize := uint32(lookupHeaderLen) + 8*uint32(len(l.Subtables))

		if newSize < oldSize {
			replace[tCode] = true
			extra += len(l.Subtables)
			lastPos -= oldSize - newSize
		}

		idx--
	}
	if lastPos > 0xFFFF {
		panic("too much data for lookup list table")
	}

	res := make([]layoutChunk, 0, len(chunks)+extra)
	var moved, ext []layoutChunk
	for _, chunk := range chunks {
		code := chunk.code
		tp := code & chunkTypeMask
		tCode := code & chunkTableMask
		switch {
		case tp == chunkHeader:
			res = append(res, chunk)
		case tCode == biggestLookup:
			moved = append(moved, chunk)
		case replace[tCode]:
			sCode := code & chunkSubtableMask
			if tp == chunkSubtable {
				res = append(res, layoutChunk{
					code: chunkExtReplace | tCode | sCode,
					size: 8,
				})
				ext = append(ext, chunk)
			} else {
				res = append(res, chunk)
			}
		default:
			res = append(res, chunk)
		}
	}
	res = append(res, moved...)
	res = append(res, ext...)

	return res
}

// Extension Substitution/Positioning Subtable Format 1
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#71-extension-substitution-subtable-format-1
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#lookuptype-9-extension-positioning
type extensionSubtable struct {
	ExtensionLookupType uint16
	ExtensionOffset     int64
}

func readExtensionSubtable(p *parser.Parser, _ int64) (Subtable, error) {
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

// FindLookups returns the lookups required to implement the given
// features in the specified locale.
func (info *Info) FindLookups(loc *locale.Locale, includeFeature map[string]bool) []LookupIndex {
	if info == nil || len(info.ScriptList) == 0 {
		return nil
	}

	candidates := []ScriptLang{
		{Script: locale.ScriptUndefined, Lang: locale.LangUndefined},
	}
	if loc.Script != locale.ScriptUndefined {
		candidates = append(candidates,
			ScriptLang{Script: loc.Script, Lang: locale.LangUndefined})
	}
	if loc.Language != locale.LangUndefined {
		candidates = append(candidates,
			ScriptLang{Script: locale.ScriptUndefined, Lang: loc.Language})
	}
	if len(candidates) == 3 { // both are defined
		candidates = append(candidates,
			ScriptLang{Script: loc.Script, Lang: loc.Language})
	}
	var features *Features
	for _, cand := range candidates {
		f, ok := info.ScriptList[cand]
		if ok {
			features = f
			break
		}
	}
	if features == nil {
		return nil
	}

	includeLookup := make(map[LookupIndex]bool)
	numFeatures := FeatureIndex(len(info.FeatureList))
	if features.Required < numFeatures {
		feature := info.FeatureList[features.Required]
		for _, l := range feature.Lookups {
			includeLookup[l] = true
		}
	}
	for _, f := range features.Optional {
		if f >= numFeatures {
			continue
		}
		feature := info.FeatureList[f]
		if !includeFeature[feature.Tag] {
			continue
		}
		for _, l := range feature.Lookups {
			includeLookup[l] = true
		}
	}

	numLookups := LookupIndex(len(info.LookupList))
	var ll []LookupIndex
	for l := range includeLookup {
		if l >= numLookups {
			continue
		}
		ll = append(ll, l)
	}
	sort.Slice(ll, func(i, j int) bool {
		return ll[i] < ll[j]
	})
	return ll
}

// Lookuptypes for extension lookup records.
// This can be used as an argument for the [Info.Encode] method.
const (
	GposExtensionLookupType uint16 = 9
	GsubExtensionLookupType uint16 = 7
)
