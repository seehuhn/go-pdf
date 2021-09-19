// seehuhn.de/go/pdf - a library for reading and writing PDF files
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

package gtab

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/parser"
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

// ReadGposTable reads the "GPOS" table of a font, for a given writing script
// and language.
func (g *GTab) ReadGposTable(extraFeatures ...string) (Lookups, error) {
	includeFeature := map[string]bool{
		"kern": true,
		"mark": true,
		"mkmk": true,
	}
	for _, feature := range extraFeatures {
		includeFeature[feature] = true
	}

	g.classDefCache = make(map[int64]ClassDef)
	g.coverageCache = make(map[int64]coverage)
	g.subtableReader = g.readGposSubtable

	ll, err := g.selectLookups("GPOS", includeFeature)
	if err != nil {
		return nil, err
	}

	return g.readLookups(ll)
}

func (g *GTab) readGposSubtable(s *parser.State, lookupType uint16, subtablePos int64) (LookupSubtable, error) {
	// TODO(voss): is this called more than once for the same subtablePos? -> use caching?
	s.A = subtablePos
	err := g.Exec(s,
		parser.CmdSeek,
		parser.CmdRead16, parser.TypeUInt, // format
	)
	if err != nil {
		return nil, err
	}
	format := s.A

	// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#table-organization
	// switch 10*format + uint16(subFormat) {
	switch 10*lookupType + uint16(format) {
	case 2_1: // Pair Adjustment Positioning Format 1: Adjustments for Glyph Pairs
		return g.readGpos2_1(s, subtablePos)

	case 2_2: // Pair Adjustment Positioning Format 2: Class Pair Adjustment
		return g.readGpos2_2(s, subtablePos)

	case 4_1: // Mark-to-Base Attachment Positioning Format 1: Mark-to-base Attachment
		return g.readGpos4_1(s, subtablePos)

	case 6_1: // Mark-to-Mark Attachment Positioning Format 1: Mark-to-Mark Attachment
		return g.readGpos6_1(s, subtablePos)

	case 7_2: // Context Positioning Subtable Format 2: Class-based Glyph Contexts
		return g.readSeqContext2(s, subtablePos)

	case 8_1: // Chained Contexts Positioning Format 1: Simple Glyph Contexts
		return g.readChained1(s, subtablePos)

	case 8_2: // Chained Contexts Positioning Format 2: Class-based Glyph Contexts
		return g.readChained2(s, subtablePos)

	case 8_3: // Chained Contexts Positioning Format 3: Coverage-based Glyph Contexts
		return g.readChained3(s, subtablePos)

	case 9_1: // Extension Positioning Subtable Format 1
		err = g.Exec(s,
			parser.CmdRead16, parser.TypeUInt, // extensionLookupType
			parser.CmdStoreInto, 0,
			parser.CmdRead32, parser.TypeUInt, // extensionOffset
		)
		if err != nil {
			return nil, err
		}
		if s.R[0] == 9 {
			return nil, g.Error("invalid extension lookup")
		}
		return g.readGposSubtable(s, uint16(s.R[0]), subtablePos+s.A)

	default:
		return &lookupNotImplemented{
			table:      "GPOS",
			lookupType: lookupType,
			format:     uint16(format),
		}, nil
	}
}

// Pair Adjustment Positioning Format 1: Adjustments for Glyph Pairs
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#pair-adjustment-positioning-format-1-adjustments-for-glyph-pairs
type gpos2_1 struct {
	cov    coverage
	adjust []map[font.GlyphID]*pairAdjust
}

type pairAdjust struct {
	first, second *valueRecord
}

func (g *GTab) readGpos2_1(s *parser.State, subtablePos int64) (*gpos2_1, error) {
	err := g.Exec(s,
		parser.CmdStash,                   // coverageOffset
		parser.CmdStash,                   // valueFormat1
		parser.CmdStash,                   // valueFormat2
		parser.CmdRead16, parser.TypeUInt, // pairSetCount
		parser.CmdLoop,
		parser.CmdStash, // pairSetOffsets[i]
		parser.CmdEndLoop,
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
		return nil, g.Error("malformed GPOS 2.1 table")
	}

	res := &gpos2_1{
		cov:    coverage,
		adjust: make([]map[font.GlyphID]*pairAdjust, len(pairSetOffsets)),
	}
	for i, offs := range pairSetOffsets {
		pairMap := make(map[font.GlyphID]*pairAdjust)
		s.A = subtablePos + int64(offs)
		err := g.Exec(s,
			parser.CmdSeek,
			parser.CmdRead16, parser.TypeUInt, // pairValueCount
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

func (l *gpos2_1) Apply(keep KeepGlyphFn, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	class, ok := l.cov[seq[pos].Gid]
	if !ok || class >= len(l.adjust) {
		return seq, -1
	}
	tab := l.adjust[class]

	next := pos + 1
	for next < len(seq) && !keep(seq[next].Gid) {
		next++
	}
	if next >= len(seq) {
		return seq, -1
	}

	pairAdjust, ok := tab[seq[next].Gid]
	if !ok {
		return seq, -1
	}

	pairAdjust.first.Apply(&seq[pos])
	pairAdjust.second.Apply(&seq[next])

	if pairAdjust.second == nil {
		return seq, next
	}
	return seq, next + 1
}

// Pair Adjustment Positioning Format 2: Class Pair Adjustment
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#pair-adjustment-positioning-format-2-class-pair-adjustment
type gpos2_2 struct {
	cov       coverage
	classDef1 ClassDef
	classDef2 ClassDef
	adjust    [][]pairAdjust
}

func (g *GTab) readGpos2_2(s *parser.State, subtablePos int64) (*gpos2_2, error) {
	err := g.Exec(s,
		parser.CmdStash, // coverageOffset
		parser.CmdStash, // valueFormat1
		parser.CmdStash, // valueFormat2
		parser.CmdStash, // classDef1Offset
		parser.CmdStash, // classDef2Offset
		parser.CmdStash, // class1Count
		parser.CmdStash, // class2Count
	)
	if err != nil {
		return nil, err
	}
	data := s.GetStash()
	coverageOffset := data[0]
	valueFormat1 := data[1]
	valueFormat2 := data[2]
	classDef1Offset := data[3]
	classDef2Offset := data[4]
	class1Count := int(data[5])
	class2Count := int(data[6])

	n := class1Count * class2Count
	aa := make([]pairAdjust, n)
	for i := 0; i < n; i++ {
		vr1, err := g.readValueRecord(valueFormat1)
		if err != nil {
			return nil, err
		}
		vr2, err := g.readValueRecord(valueFormat2)
		if err != nil {
			return nil, err
		}
		aa[i].first = vr1
		aa[i].second = vr2
	}

	cov, err := g.readCoverageTable(subtablePos + int64(coverageOffset))
	if err != nil {
		return nil, err
	}

	classDef1, err := g.readClassDefTable(subtablePos + int64(classDef1Offset))
	if err != nil {
		return nil, err
	}

	classDef2, err := g.readClassDefTable(subtablePos + int64(classDef2Offset))
	if err != nil {
		return nil, err
	}

	res := &gpos2_2{
		cov:       cov,
		classDef1: classDef1,
		classDef2: classDef2,
		adjust:    make([][]pairAdjust, class1Count),
	}
	for i := 0; i < class1Count; i++ {
		a := i * class2Count
		b := (i + 1) * class2Count
		res.adjust[i] = aa[a:b]
	}

	return res, nil
}

func (l *gpos2_2) Apply(keep KeepGlyphFn, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	_, ok := l.cov[seq[pos].Gid]
	if !ok {
		return seq, -1
	}

	next := pos + 1
	for next < len(seq) && !keep(seq[next].Gid) {
		next++
	}
	if next >= len(seq) {
		return seq, -1
	}

	class1 := l.classDef1[seq[pos].Gid]
	class2 := l.classDef2[seq[next].Gid]
	pairAdjust := l.adjust[class1][class2]

	pairAdjust.first.Apply(&seq[pos])
	pairAdjust.second.Apply(&seq[next])

	if pairAdjust.second == nil {
		return seq, next
	}
	return seq, next + 1
}

// Mark-to-Base Attachment Positioning Format 1: Mark-to-base Attachment Point
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#mark-to-base-attachment-positioning-format-1-mark-to-base-attachment-point
type gpos4_1 struct {
	Mark map[font.GlyphID]*markRecord // the attaching mark
	Base map[font.GlyphID][]anchor    // the base glyph being attached to
}

func (g *GTab) readGpos4_1(s *parser.State, subtablePos int64) (*gpos4_1, error) {
	err := g.Exec(s,
		parser.CmdStash,                   // markCoverageOffset
		parser.CmdStash,                   // baseCoverageOffset
		parser.CmdRead16, parser.TypeUInt, // markClassCount
		parser.CmdStoreInto, 0,
		parser.CmdStash, // markArrayOffset
		parser.CmdStash, // baseArrayOffset
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
		parser.CmdSeek,
		parser.CmdRead16, parser.TypeUInt, // baseCount
		parser.CmdMult, 0,
		parser.CmdLoop,
		parser.CmdStash, // baseRecords[i].baseAnchorOffsets[j]
		parser.CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	data = s.GetStash()
	baseAnchors := make([]anchor, len(data))
	for i, offs := range data {
		if offs == 0 {
			continue
		}
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
			return nil, g.Error("malformed GPOS 4.1 table")
		}
		res.Base[gid] = baseAnchors[a:b]
	}
	for gid, idx := range markCoverage {
		if idx >= len(markArray) || markArray[idx].markClass >= uint16(markClassCount) {
			return nil, g.Error("malformed GPOS 4.1 table")
		}
		res.Mark[gid] = &markArray[idx]
	}

	return res, nil
}

func (l *gpos4_1) Apply(filter KeepGlyphFn, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	if pos == 0 {
		return seq, -1
	}
	mark, ok := l.Mark[seq[pos].Gid]
	if !ok {
		return seq, -1
	}
	base, ok := l.Base[seq[pos-1].Gid]
	if !ok {
		return seq, -1
	}

	baseAnchor := base[mark.markClass]

	seq[pos].XOffset = -seq[pos-1].Advance + int(baseAnchor.X) - int(mark.X)
	seq[pos].YOffset = int(baseAnchor.Y) - int(mark.Y)
	return seq, pos + 1
}

// Mark-to-Mark Attachment Positioning Format 1: Mark-to-base Attachment
type gpos6_1 struct {
	Mark1 map[font.GlyphID]*markRecord // the attaching mark
	Mark2 map[font.GlyphID][]anchor    // the base mark being attached to
}

func (g *GTab) readGpos6_1(s *parser.State, subtablePos int64) (*gpos6_1, error) {
	// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#mark-to-mark-attachment-positioning-format-1-mark-to-mark-attachment
	err := g.Exec(s,
		parser.CmdStash,                   // mark1CoverageOffset
		parser.CmdStash,                   // mark2CoverageOffset
		parser.CmdRead16, parser.TypeUInt, // markClassCount
		parser.CmdStoreInto, 0,
		parser.CmdStash, // mark1ArrayOffset
		parser.CmdStash, // mark2ArrayOffset
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
		parser.CmdSeek,
		parser.CmdRead16, parser.TypeUInt, // mark2Count
		parser.CmdMult, 0,
		parser.CmdLoop,
		parser.CmdStash, // mark2Records[i].mark2AnchorOffsets[j]
		parser.CmdEndLoop,
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
			return nil, g.Error("malformed GPOS 4.1 table")
		}
		res.Mark2[gid] = mark2Anchors[a:b]
	}
	for gid, idx := range mark1Coverage {
		if idx >= len(mark1Array) || mark1Array[idx].markClass >= uint16(markClassCount) {
			return nil, g.Error("malformed GPOS 6.1 table")
		}
		res.Mark1[gid] = &mark1Array[idx]
	}

	return res, nil
}

func (l *gpos6_1) Apply(filter KeepGlyphFn, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	// The mark2 glyph that combines with a mark1 glyph is the glyph preceding
	// the mark1 glyph in glyph string order (skipping glyphs according to
	// LookupFlags). The subtable applies precisely when that mark2 glyph is
	// covered by mark2Coverage. To combine the mark glyphs, the placement of
	// the mark1 glyph is adjusted such that the relevant attachment points
	// coincide.

	x, ok := l.Mark1[seq[pos].Gid]
	if !ok {
		return seq, -1
	}

	prevPos := pos - 1
	for prevPos >= 0 && !filter(seq[prevPos].Gid) {
		prevPos--
	}
	if prevPos < 0 {
		return seq, -1
	}
	prev, ok := l.Mark2[seq[prevPos].Gid]
	if !ok {
		return seq, -1
	}

	_ = x
	_ = prev
	_ = ok
	panic("not implemented")
}
