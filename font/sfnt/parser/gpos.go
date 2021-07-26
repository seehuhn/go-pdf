// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package parser

import (
	"fmt"

	"seehuhn.de/go/pdf/font"
)

// The most common GPOS features seen on my system:
//     6777 "kern"
//     3219 "mark"
//     2464 "mkmk"
//     2301 "cpsp"
//     1352 "size"
//      117 "case"
//       92 "dist"
//       76 "vhal"
//       76 "halt"

// GposInfo represents the information from the "GPOS" table of a font.
type GposInfo []*gposLookup

// ReadGposInfo reads the "GSUB" table of a font, for a given writing script
// and language.
func (p *Parser) ReadGposInfo(script, lang string, extraFeatures ...string) (GposInfo, error) {
	gtab, err := newGTab(p, script, lang)
	if err != nil {
		return nil, err
	}

	includeFeature := map[string]bool{
		"kern": true,
		"mark": true,
		"mkmk": true,
	}
	for _, feature := range extraFeatures {
		includeFeature[feature] = true
	}
	err = gtab.Init("GPOS", includeFeature)
	if err != nil {
		return nil, err
	}

	var res GposInfo
	for _, idx := range gtab.LookupIndices {
		l, err := gtab.getGposLookup(idx)
		if err != nil {
			return nil, err
		}
		res = append(res, l)
	}
	return res, nil
}

// Layout applies the positioning from the selected GPOS lookups to a
// series of glyphs.
func (gpos GposInfo) Layout(glyphs []font.Glyph) {
	for _, l := range gpos {
		l.Layout(glyphs)
	}
}

type gposLookup struct {
	Format uint16 // TODO(voss): remove?

	rtl    bool
	filter filter

	subtables        []gposLookupSubtable
	markFilteringSet uint16
}

func (l *gposLookup) Layout(glyphs []font.Glyph) {
	pos := 0
	for pos < len(glyphs) {
		next := l.Position(glyphs, pos)
		if next > pos {
			pos = next
		} else {
			pos++
		}
	}
}

func (l *gposLookup) Position(glyphs []font.Glyph, pos int) int {
	var next int
	for _, subtable := range l.subtables {
		next = subtable.Position(l.filter, glyphs, pos)
		if next > pos {
			return next
		}
	}
	return pos
}

func (g *gTab) getGposLookup(idx uint16) (*gposLookup, error) {
	if int(idx) >= len(g.lookups) {
		return nil, g.error("lookup index %d out of range", idx)
	}
	base := g.lookupListOffset + int64(g.lookups[idx])

	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-table
	s := &State{}
	s.A = base
	err := g.Exec(s,
		CmdSeek,
		CmdStash,            // lookupType
		CmdStash,            // lookupFlag
		CmdRead16, TypeUInt, // subtableCount
		CmdLoop,
		CmdStash, // subtableOffset
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	data := s.GetStash()
	format := data[0]
	flags := data[1]
	subtables := data[2:]
	var markFilteringSet uint16
	if flags&0x0010 != 0 {
		err = g.Exec(s, CmdRead16, TypeUInt) // markFilteringSet
		if err != nil {
			return nil, err
		}
		markFilteringSet = uint16(s.A)
	}

	lookup := &gposLookup{
		Format:           format,
		rtl:              flags&0x0001 != 0,
		filter:           g.makeFilter(flags),
		markFilteringSet: markFilteringSet,
	}
	for _, offs := range subtables {
		res, err := g.readGposSubtable(s, format, base+int64(offs))
		if err != nil {
			return nil, err
		}

		if res != nil {
			lookup.subtables = append(lookup.subtables, res)
		}
	}

	return lookup, nil
}

func (g *gTab) readGposSubtable(s *State, format uint16, subtablePos int64) (gposLookupSubtable, error) {
	// TODO(voss): is this called more than once for the same subtablePos? -> use caching?
	s.A = subtablePos
	err := g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // (sub-)format
	)
	if err != nil {
		return nil, err
	}
	subFormat := s.A

	// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#table-organization
	// switch 10*format + uint16(subFormat) {
	switch 10*format + uint16(subFormat) {
	case 2_1: // Pair Adjustment Positioning Format 1: Adjustments for Glyph Pairs
		return g.readGpos2_1(s, subtablePos)
	case 4_1: // Mark-to-Base Attachment Positioning Format 1: Mark-to-base Attachment
		return g.readGpos4_1(s, subtablePos)
	case 6_1: // Mark-to-Mark Attachment Positioning Format 1: Mark-to-Mark Attachment
		return g.readGpos6_1(s, subtablePos)
	case 9_1: // Extension Positioning Subtable Format 1
		err = g.Exec(s,
			CmdRead16, TypeUInt, // extensionLookupType
			CmdStoreInto, 0,
			CmdRead32, TypeUInt, // extensionOffset
		)
		if err != nil {
			return nil, err
		}
		if s.R[0] == 9 {
			return nil, g.error("invalid extension lookup")
		}
		return g.readGposSubtable(s, uint16(s.R[0]), subtablePos+s.A)
	}

	// fmt.Println("unsupported GPOS format", format, subFormat)
	return nil, nil
}

type gposLookupSubtable interface {
	Position(filter, []font.Glyph, int) int
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#pair-adjustment-positioning-format-1-adjustments-for-glyph-pairs
type gpos2_1 struct {
	cov    coverage
	adjust []map[font.GlyphID]*pairAdjust
}

type pairAdjust struct {
	first, second *valueRecord
}

func (g *gTab) readGpos2_1(s *State, subtablePos int64) (*gpos2_1, error) {
	err := g.Exec(s,
		CmdStash,            // coverageOffset
		CmdStash,            // valueFormat1
		CmdStash,            // valueFormat2
		CmdRead16, TypeUInt, // pairSetCount
		CmdLoop,
		CmdStash, // pairSetOffsets[i]
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	data := s.GetStash()
	coverageOffset := data[0]
	valueFormat1 := data[1]
	valueFormat2 := data[2]
	pairSetOffsets := data[3:]

	coverage, err := g.readCoverageTable(subtablePos + int64(coverageOffset))
	if err != nil {
		return nil, err
	}
	if len(coverage) != len(pairSetOffsets) {
		return nil, g.error("malformed GPOS 2.1 table")
	}

	res := &gpos2_1{
		cov:    coverage,
		adjust: make([]map[font.GlyphID]*pairAdjust, len(pairSetOffsets)),
	}
	for i, offs := range pairSetOffsets {
		pairMap := make(map[font.GlyphID]*pairAdjust)
		s.A = subtablePos + int64(offs)
		err := g.Exec(s,
			CmdSeek,
			CmdRead16, TypeUInt, // pairValueCount
		)
		if err != nil {
			return nil, err
		}
		pairValueCount := int(s.A)
		for i := 0; i < pairValueCount; i++ {
			secondGlyph, err := g.ReadUInt16()
			if err != nil {
				return nil, err
			}
			vr1, err := g.readValueRecord(valueFormat1)
			if err != nil {
				return nil, err
			}
			vr2, err := g.readValueRecord(valueFormat2)
			if err != nil {
				return nil, err
			}
			pairMap[font.GlyphID(secondGlyph)] = &pairAdjust{
				first:  vr1,
				second: vr2,
			}
		}
		res.adjust[i] = pairMap
	}

	return res, nil
}

func (l *gpos2_1) Position(filter filter, seq []font.Glyph, pos int) int {
	panic("not implemented")
}

type gpos4_1 struct {
	Mark map[font.GlyphID]*markRecord // the attaching mark
	Base map[font.GlyphID][]anchor    // the base glyph being attached to
}

// Mark-to-Base Attachment Positioning Format 1: Mark-to-base Attachment Point
func (g *gTab) readGpos4_1(s *State, subtablePos int64) (*gpos4_1, error) {
	err := g.Exec(s,
		CmdStash,            // markCoverageOffset
		CmdStash,            // baseCoverageOffset
		CmdRead16, TypeUInt, // markClassCount
		CmdStoreInto, 0,
		CmdStash, // markArrayOffset
		CmdStash, // baseArrayOffset
	)
	if err != nil {
		return nil, err
	}
	data := s.GetStash()
	markCoverageOffset := data[0]
	baseCoverageOffset := data[1]
	markClassCount := int(s.R[0])
	markArrayOffset := data[2]
	baseArrayOffset := data[3]

	markArray, err := g.readMarkArrayTable(subtablePos + int64(markArrayOffset))
	if err != nil {
		return nil, err
	}

	baseArrayPos := subtablePos + int64(baseArrayOffset)
	s.A = baseArrayPos
	err = g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // baseCount
		CmdMult, 0,
		CmdLoop,
		CmdStash, // baseRecords[i].baseAnchorOffsets[j]
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	data = s.GetStash()
	baseAnchors := make([]anchor, len(data))
	for i, offs := range data {
		err = g.readAnchor(baseArrayPos+int64(offs), &baseAnchors[i])
		if err != nil {
			return nil, err
		}
	}

	markCoverage, err := g.readCoverageTable(subtablePos + int64(markCoverageOffset))
	if err != nil {
		return nil, err
	}
	baseCoverage, err := g.readCoverageTable(subtablePos + int64(baseCoverageOffset))
	if err != nil {
		return nil, err
	}

	res := &gpos4_1{
		Mark: map[font.GlyphID]*markRecord{},
		Base: map[font.GlyphID][]anchor{},
	}
	for gid, idx := range baseCoverage {
		a := idx * markClassCount
		b := a + markClassCount
		if b > len(baseAnchors) {
			return nil, g.error("malformed GPOS 4.1 table")
		}
		res.Base[gid] = baseAnchors[a:b]
	}
	for gid, idx := range markCoverage {
		if idx >= len(markArray) || markArray[idx].markClass >= uint16(markClassCount) {
			return nil, g.error("malformed GPOS 4.1 table")
		}
		res.Mark[gid] = &markArray[idx]
	}

	return res, nil
}

func (l *gpos4_1) Position(filter filter, seq []font.Glyph, pos int) int {
	if pos == 0 {
		return pos
	}
	mark, ok := l.Mark[seq[pos].Gid]
	if !ok {
		return pos
	}
	base, ok := l.Base[seq[pos-1].Gid]
	if !ok {
		return pos
	}

	baseAnchor := base[mark.markClass]

	seq[pos].XOffset = -seq[pos-1].Advance + int(baseAnchor.X) - int(mark.X)
	seq[pos].YOffset = int(baseAnchor.Y) - int(mark.Y)
	return pos + 1
}

// Mark-to-Mark Attachment Positioning Format 1: Mark-to-base Attachment
type gpos6_1 struct {
	Mark1 map[font.GlyphID]*markRecord // the attaching mark
	Mark2 map[font.GlyphID][]anchor    // the base mark being attached to
}

func (g *gTab) readGpos6_1(s *State, subtablePos int64) (*gpos6_1, error) {
	// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#mark-to-mark-attachment-positioning-format-1-mark-to-mark-attachment
	err := g.Exec(s,
		CmdStash,            // mark1CoverageOffset
		CmdStash,            // mark2CoverageOffset
		CmdRead16, TypeUInt, // markClassCount
		CmdStoreInto, 0,
		CmdStash, // mark1ArrayOffset
		CmdStash, // mark2ArrayOffset
	)
	if err != nil {
		return nil, err
	}
	data := s.GetStash()
	mark1CoverageOffset := data[0]
	mark2CoverageOffset := data[1]
	markClassCount := int(s.R[0])
	mark1ArrayOffset := data[2]
	mark2ArrayOffset := data[3]

	mark1Array, err := g.readMarkArrayTable(subtablePos + int64(mark1ArrayOffset))
	if err != nil {
		return nil, err
	}

	baseArrayPos := subtablePos + int64(mark2ArrayOffset)
	s.A = baseArrayPos
	err = g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // mark2Count
		CmdMult, 0,
		CmdLoop,
		CmdStash, // mark2Records[i].mark2AnchorOffsets[j]
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	data = s.GetStash()
	mark2Anchors := make([]anchor, len(data))
	for i, offs := range data {
		err = g.readAnchor(baseArrayPos+int64(offs), &mark2Anchors[i])
		if err != nil {
			return nil, err
		}
	}

	mark1Coverage, err := g.readCoverageTable(subtablePos + int64(mark1CoverageOffset))
	if err != nil {
		return nil, err
	}
	mark2Coverage, err := g.readCoverageTable(subtablePos + int64(mark2CoverageOffset))
	if err != nil {
		return nil, err
	}

	res := &gpos6_1{
		Mark1: map[font.GlyphID]*markRecord{},
		Mark2: map[font.GlyphID][]anchor{},
	}
	for gid, idx := range mark2Coverage {
		a := idx * markClassCount
		b := a + markClassCount
		if b > len(mark2Anchors) {
			return nil, g.error("malformed GPOS 4.1 table")
		}
		res.Mark2[gid] = mark2Anchors[a:b]
	}
	for gid, idx := range mark1Coverage {
		if idx >= len(mark1Array) || mark1Array[idx].markClass >= uint16(markClassCount) {
			return nil, g.error("malformed GPOS 6.1 table")
		}
		res.Mark1[gid] = &mark1Array[idx]
	}

	return res, nil
}

func (l *gpos6_1) Position(filter filter, seq []font.Glyph, pos int) int {
	// The mark2 glyph that combines with a mark1 glyph is the glyph preceding
	// the mark1 glyph in glyph string order (skipping glyphs according to
	// LookupFlags). The subtable applies precisely when that mark2 glyph is
	// covered by mark2Coverage. To combine the mark glyphs, the placement of
	// the mark1 glyph is adjusted such that the relevant attachment points
	// coincide.

	x, ok := l.Mark1[seq[pos].Gid]
	if !ok || pos == 0 {
		return pos
	}

	prevPos := pos - 1
	for prevPos >= 0 && !filter(seq[prevPos].Gid) {
		prevPos--
	}
	if prevPos < 0 {
		return pos
	}
	prev, ok := l.Mark2[seq[prevPos].Gid]
	if !ok {
		return pos
	}

	_ = x
	_ = prev
	_ = ok
	fmt.Println(seq[prevPos].Gid, seq[pos].Gid)
	panic("not implemented")
}
