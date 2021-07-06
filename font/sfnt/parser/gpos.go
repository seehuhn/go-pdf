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

type gposInfo []*gposLookup

// readGposInfo reads the "GSUB" table of a font, for a given writing script
// and language.
func (p *Parser) readGposInfo(script, lang string, extraFeatures ...string) (gposInfo, error) {
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

	var res gposInfo
	for _, idx := range gtab.LookupIndices {
		l, err := gtab.GetGposLookup(idx)
		if err != nil {
			return nil, err
		}
		res = append(res, l)
	}
	return res, nil
}

type gposLookup struct {
	Format uint16 // TODO(voss): remove?
	Flags  uint16

	subtables        []gposLookupSubtable
	markFilteringSet uint16
}

func (g *gTab) GetGposLookup(idx uint16) (*gposLookup, error) {
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
		Flags:            flags,
		markFilteringSet: markFilteringSet,
	}
	for _, offs := range subtables {
		res, err := g.readGposSubtable(s, format, base+int64(offs))
		if err != nil {
			return nil, err
		}

		lookup.subtables = append(lookup.subtables, res)
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
	case 4_1: // Mark-to-Base Attachment Positioning Format 1: Mark-to-base Attachment
		return g.readGpos4_1(s, subtablePos)
	case 6_1: // Mark-to-Mark Attachment Positioning Format 1: Mark-to-Mark Attachment
		return g.readGpos6_1(s, subtablePos)
	case 9_1: // Extension Positioning Subtable Format 1
		err = g.Exec(s,
			CmdRead16, TypeUInt, // extensionLookupType
			CmdStore, 0,
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

	fmt.Println("upsupported format", format, subFormat)
	// return nil, g.error("unsupported lookup type %d.%d\n", format, subFormat)
	return nil, nil
}

type gposLookupSubtable interface {
	Position(uint16, []font.GlyphPos, int) int
}

type gpos4_1 struct {
	Mark map[font.GlyphID]*markRecord
	Base map[font.GlyphID][]anchor
}

// Mark-to-Base Attachment Positioning Format 1: Mark-to-base Attachment Point
func (g *gTab) readGpos4_1(s *State, subtablePos int64) (*gpos4_1, error) {
	err := g.Exec(s,
		CmdStash,            // markCoverageOffset
		CmdStash,            // baseCoverageOffset
		CmdRead16, TypeUInt, // markClassCount
		CmdStore, 0,
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

	xxx := &gpos4_1{
		Mark: map[font.GlyphID]*markRecord{},
		Base: map[font.GlyphID][]anchor{},
	}
	for gid, idx := range baseCoverage {
		a := idx * markClassCount
		b := a + markClassCount
		if b > len(baseAnchors) {
			return nil, g.error("malformed GPOS 4.1 table")
		}
		xxx.Base[gid] = baseAnchors[a:b]
	}
	for gid, idx := range markCoverage {
		if idx >= len(markArray) || markArray[idx].markClass >= uint16(markClassCount) {
			return nil, g.error("malformed GPOS 4.1 table")
		}
		xxx.Mark[gid] = &markArray[idx]
	}
	fmt.Println(xxx.Base)
	fmt.Println(xxx.Mark)

	return xxx, nil
}

func (l *gpos4_1) Position(flags uint16, seq []font.GlyphPos, pos int) int {
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
	err := g.Exec(s,
		CmdStash, // mark1CoverageOffset
		CmdStash, // mark2CoverageOffset
		CmdStash, // markClassCount
		CmdStash, // mark1ArrayOffset
		CmdStash, // mark2ArrayOffset
	)
	if err != nil {
		return nil, err
	}
	data := s.GetStash()
	mark1CoverageOffset := data[0]
	mark2CoverageOffset := data[1]
	markClassCount := data[2]
	mark1ArrayOffset := data[3]
	mark2ArrayOffset := data[4]

	mark1Array, err := g.readMarkArrayTable(subtablePos + int64(mark1ArrayOffset))
	if err != nil {
		return nil, err
	}

	fmt.Println("MA1", mark1Array)

	mark1Coverage, err := g.readCoverageTable(subtablePos + int64(mark1CoverageOffset))
	if err != nil {
		return nil, err
	}
	mark2Coverage, err := g.readCoverageTable(subtablePos + int64(mark2CoverageOffset))
	if err != nil {
		return nil, err
	}

	_ = mark1Coverage
	_ = mark2Coverage
	_ = markClassCount
	_ = mark1Array
	_ = mark2ArrayOffset
	panic("not implemented")
}

func (l *gpos6_1) Position(flags uint16, seq []font.GlyphPos, pos int) int {
	// The mark2 glyph that combines with a mark1 glyph is the glyph preceding
	// the mark1 glyph in glyph string order (skipping glyphs according to
	// LookupFlags). The subtable applies precisely when that mark2 glyph is
	// covered by mark2Coverage. To combine the mark glyphs, the placement of
	// the mark1 glyph is adjusted such that the relevant attachment points
	// coincide.
	panic("not implemented")
}
