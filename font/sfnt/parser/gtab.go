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
	"sort"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/locale"
)

// gTab represents a GSUB or GPOS table.
type gTab struct {
	*Parser
	script, language string

	gdef *GdefInfo

	lookupIndices    []uint16
	lookupListOffset int64
	lookups          []uint16
	subtableReader   stReaderFn

	coverageCache map[int64]coverage
}

// newGTab wraps a parser with a helper to read GSUB and GPOS tables.
// This modifies p.Funcs!
func newGTab(p *Parser, tableName string, loc *locale.Locale, includeFeature map[string]bool) (*gTab, error) {
	gdef, err := p.ReadGdefTable()
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}

	g := &gTab{
		Parser:   p,
		script:   otfScript[loc.Script],
		language: otfLanguage[loc.Language],

		gdef:          gdef,
		coverageCache: make(map[int64]coverage),
	}
	switch tableName {
	case "GPOS":
		g.subtableReader = g.readGposSubtable
	case "GSUB":
		g.subtableReader = g.readGsubSubtable
	default:
		panic("invalid table type " + tableName)
	}

	err = g.init(tableName, includeFeature)
	if err != nil {
		return nil, err
	}

	return g, nil
}

func (g *gTab) init(tableName string, includeFeature map[string]bool) error {
	err := g.OpenTable(tableName)
	if err != nil {
		return err
	}

	s := &State{}

	err = g.Exec(s,
		CmdStash, // majorVersion
		CmdStash, // minorVersion
		CmdStash, // scriptListOffset
		CmdStash, // featureListOffset
		CmdStash, // lookupListOffset
	)
	if err != nil {
		return err
	}
	data := s.GetStash()
	if data[0] != 1 || data[1] > 1 {
		return g.error("unsupported %s version %d.%d", tableName, data[0], data[1])
	}
	scriptListOffset := int64(data[2])
	featureListOffset := int64(data[3])
	lookupListOffset := int64(data[4])
	var featureVariationsOffset uint16
	if data[1] > 0 {
		featureVariationsOffset, err = g.ReadUInt16()
		if err != nil {
			return err
		}
	}
	if featureVariationsOffset != 0 {
		return g.error("VariationIndex tables not supported")
	}

	// Read the script list and pick a script.
	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-list-table-and-script-record
	s.A = scriptListOffset
	err = g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // scriptCount
	)
	if err != nil {
		return err
	}
	scriptCount := s.A
	var bestScriptOffs int64 = -1
	for i := 0; i < int(scriptCount); i++ {
		err = g.Exec(s,
			CmdRead32, TypeTag, // scriptTag
			CmdRead16, TypeUInt, // scriptOffset
		)
		if err != nil {
			return err
		}
		if s.Tag == g.script || s.Tag == "DFLT" && bestScriptOffs < 0 {
			bestScriptOffs = s.A
		}
	}
	if bestScriptOffs <= 0 {
		// no scripts defined
		return nil
	}

	// Read the language list for the script, and pick a LangSys
	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#script-table-and-language-system-record
	scriptTablePos := int64(scriptListOffset) + bestScriptOffs
	s.A = scriptTablePos
	err = g.Exec(s,
		CmdSeek,
		CmdStash, // defaultLangSysOffset
		CmdStash, // langSysCount
	)
	if err != nil {
		return err
	}
	data = s.GetStash()
	defaultLangSysOffset := data[0]
	langSysCount := int(data[1])
	bestLangSysOffs := int64(defaultLangSysOffset)
	for i := 0; i < langSysCount; i++ {
		err = g.Exec(s,
			CmdRead32, TypeTag, // langSysTag
			CmdRead16, TypeUInt, // langSysOffset
		)
		if err != nil {
			return err
		}
		if s.Tag == g.language {
			bestLangSysOffs = s.A
			break
		}
	}
	if bestLangSysOffs == 0 {
		// no language-specific script behavior is defined
		return nil
	}

	// Read the LangSys table and get the feature indices
	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#language-system-table
	s.A = scriptTablePos + bestLangSysOffs
	err = g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // lookupOrderOffset
		CmdAssertEq, 0,
		CmdStash,            // requiredFeatureIndex
		CmdRead16, TypeUInt, // featureIndexCount
		CmdLoop,
		CmdStash, // featureIndices[i]
		CmdEndLoop,
	)
	if err != nil {
		return err
	}
	featureIndices := s.GetStash()
	requiredFeature := 0
	if featureIndices[0] == 0xFFFF {
		// no required feature
		featureIndices = featureIndices[1:]
		requiredFeature = -1
	}

	// Read the feature list table
	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#feature-list-table
	s.A = featureListOffset
	err = g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // featureCount
	)
	if err != nil {
		return err
	}
	numFeatures := int(s.A)
	var featureOffsets []uint16
	for i, fi := range featureIndices {
		if int(fi) >= numFeatures {
			return g.error("feature index %d out of range", fi)
		}
		s.A = featureListOffset + 2 + 6*int64(fi)
		err = g.Exec(s,
			CmdSeek,
			CmdRead32, TypeTag, // featureTag
			CmdRead16, TypeUInt, // featureOffset
		)
		if err != nil {
			return err
		}
		if includeFeature[s.Tag] || i == requiredFeature {
			featureOffsets = append(featureOffsets, uint16(s.A))
		}
	}

	// read the Feature Tables to find the lookup indices
	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#feature-table
	var lookupIndices []uint16
	for _, offs := range featureOffsets {
		s.A = int64(offs) + featureListOffset
		err = g.Exec(s,
			CmdSeek,             // start of Feature table
			CmdStash,            // featureParamsOffset
			CmdRead16, TypeUInt, // lookupIndexCount
			CmdLoop,
			CmdStash, // lookupIndex
			CmdEndLoop,
		)
		if err != nil {
			return err
		}
		data := s.GetStash()
		if data[0] != 0 {
			return g.error("FeatureParams tables not supported")
		}
		lookupIndices = append(lookupIndices, data[1:]...)
	}

	// sort and remove duplicates
	sort.Slice(lookupIndices, func(i, j int) bool {
		return lookupIndices[i] < lookupIndices[j]
	})
	i := 1
	for i < len(lookupIndices) {
		if lookupIndices[i] == lookupIndices[i-1] {
			lookupIndices = append(lookupIndices[:i], lookupIndices[i+1:]...)
		} else {
			i++
		}
	}
	g.lookupIndices = lookupIndices

	// Since more lookups might be required for nested lookups, we
	// keep the complete list of lookupOffsets.
	s.A = lookupListOffset
	err = g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // lookupCount
		CmdLoop,
		CmdStash, // lookupOffset[i]
		CmdEndLoop,
	)
	if err != nil {
		return err
	}
	g.lookupListOffset = lookupListOffset
	g.lookups = s.GetStash()

	return nil
}

type stReaderFn func(s *State, format uint16, subtablePos int64) (lookupSubtable, error)

// ReadLookups reads the selected lookup tables.
func (g *gTab) ReadLookups() (Lookups, error) {
	var res Lookups
	for _, idx := range g.lookupIndices {
		l, err := g.getGtabLookup(idx)
		if err != nil {
			return nil, err
		}
		res = append(res, l)
	}
	return res, nil
}

func (g *gTab) getGtabLookup(idx uint16) (*lookupTable, error) {
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

	lookup := &lookupTable{
		rtl:              flags&0x0001 != 0,
		filter:           g.makeFilter(flags),
		markFilteringSet: markFilteringSet,
	}
	for _, offs := range subtables {
		res, err := g.subtableReader(s, format, base+int64(offs))
		if err != nil {
			return nil, err
		}

		if res != nil {
			lookup.subtables = append(lookup.subtables, res)
		}
	}

	return lookup, nil
}

// Sequence Context Format 2: class-based glyph contexts
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#sequence-context-format-2-class-based-glyph-contexts
type seqContext2 struct {
	cov      coverage
	input    ClassDef
	rulesets []classSeqRuleSet // indexed by the input class, can be nil
}

type classSeqRuleSet []*classSequenceRule

type classSequenceRule struct {
	input   []int
	actions []seqLookup
}

func (g *gTab) readSeqContext2(s *State, subtablePos int64) (*seqContext2, error) {
	err := g.Exec(s,
		CmdStash, // coverageOffset
		CmdStash, // classDefOffset

		CmdRead16, TypeUInt, // classSeqRuleSetCount
		CmdLoop,
		CmdStash, // classSeqRuleSetOffsets[i]
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	stash := s.GetStash()
	coverageOffset := stash[0]
	classDefOffset := stash[1]
	classSeqRuleSetOffsets := stash[2:]

	cov, err := g.readCoverageTable(subtablePos + int64(coverageOffset))
	if err != nil {
		return nil, err
	}
	input, err := g.readClassDefTable(subtablePos + int64(classDefOffset))
	if err != nil {
		return nil, err
	}

	xxx := &seqContext2{
		cov:      cov,
		input:    input,
		rulesets: make([]classSeqRuleSet, len(classSeqRuleSetOffsets)),
	}

	for i, offs := range classSeqRuleSetOffsets {
		if offs == 0 {
			continue
		}
		classSeqRuleSetPos := subtablePos + int64(offs)
		s.A = classSeqRuleSetPos
		err = g.Exec(s,
			CmdSeek,
			CmdRead16, TypeUInt, // classSeqRuleCount
			CmdLoop,
			CmdStash, // classSeqRuleOffsets[i]
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		classSeqRuleOffsets := s.GetStash()

		var ruleSet classSeqRuleSet
		for _, ruleOffs := range classSeqRuleOffsets {
			rule := &classSequenceRule{}

			s.A = classSeqRuleSetPos + int64(ruleOffs)
			err = g.Exec(s,
				CmdSeek,
				CmdRead16, TypeUInt, // glyphCount
				CmdStoreInto, 0,
				CmdRead16, TypeUInt, // seqLookupCount
				CmdStoreInto, 1,
				CmdLoadFrom, 0,
				CmdAssertGt, 0,
				CmdDec,
				CmdLoop,
				CmdStash, // inputSequence[i]
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			for _, class := range s.GetStash() {
				rule.input = append(rule.input, int(class))
			}

			err = g.Exec(s,
				CmdLoadFrom, 1,
				CmdLoop,
				CmdStash, // seqLookupRecord.sequenceIndex
				CmdStash, // seqLookupRecord.lookupListIndex
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			seqLookupRecord := s.GetStash()

			for len(seqLookupRecord) > 0 {
				l, err := g.getGtabLookup(seqLookupRecord[1])
				if err != nil {
					return nil, err
				}
				rule.actions = append(rule.actions, seqLookup{
					pos:    int(seqLookupRecord[0]),
					nested: l,
				})
				seqLookupRecord = seqLookupRecord[2:]
			}

			ruleSet = append(ruleSet, rule)
		}
		xxx.rulesets[i] = ruleSet
	}
	return xxx, nil
}

func (l *seqContext2) Apply(filter filter, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	glyph := seq[pos]
	if _, ok := l.cov[glyph.Gid]; !ok {
		return seq, -1
	}

	class := l.input[glyph.Gid]
	if class >= len(l.rulesets) {
		return seq, -1
	}

rulesetLoop:
	for _, rule := range l.rulesets[class] {
		next := pos + 1 + len(rule.input)
		if next > len(seq) {
			continue
		}
		for i, class := range rule.input {
			if !filter(seq[pos+1+i].Gid) {
				panic("not implemented")
			}
			if !filter(seq[pos+1+i].Gid) {
				panic("not implemented")
			}
			if l.input[seq[pos+1+i].Gid] != class {
				continue rulesetLoop
			}
		}

		seq = applyActions(rule.actions, pos, seq)
		return seq, next
	}
	return seq, -1
}

// Chained Sequence Context Format 1: simple glyph contexts
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#chained-sequence-context-format-1-simple-glyph-contexts
type chainedSeq1 struct {
	cov      coverage
	rulesets []chainedSeqRuleSet
}

type chainedSeqRuleSet []*chainedSequenceRule

type chainedSequenceRule struct {
	backtrack []font.GlyphID
	input     []font.GlyphID
	lookahead []font.GlyphID
	actions   []seqLookup
}

func (g *gTab) readChained1(s *State, subtablePos int64) (*chainedSeq1, error) {
	err := g.Exec(s,
		CmdRead16, TypeUInt, // coverageOffset
		CmdStoreInto, 0,
		CmdRead16, TypeUInt, // chainedSeqRuleSetCount
		CmdLoop,
		CmdStash, // chainedSeqRuleSetOffsets[i]
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	chainedSeqRuleSetOffsets := s.GetStash()
	cov, err := g.readCoverageTable(subtablePos + s.R[0])
	if err != nil {
		return nil, err
	}

	xxx := &chainedSeq1{
		cov: cov,
	}
	for _, offs := range chainedSeqRuleSetOffsets {
		if offs == 0 {
			xxx.rulesets = append(xxx.rulesets, nil)
			continue
		}

		chainedSequenceRuleSetPos := subtablePos + int64(offs)
		s.A = chainedSequenceRuleSetPos
		err = g.Exec(s,
			CmdSeek,
			CmdRead16, TypeUInt, // chainedSeqRuleCount
			CmdLoop,
			CmdStash, // chainedSeqRuleOffsets[i]
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}

		var ruleset chainedSeqRuleSet
		chainedSeqRuleOffsets := s.GetStash()
		for _, offs := range chainedSeqRuleOffsets {
			rule := &chainedSequenceRule{}

			s.A = chainedSequenceRuleSetPos + int64(offs)
			err = g.Exec(s,
				CmdSeek,
				CmdRead16, TypeUInt, // backtrackGlyphCount
				CmdLoop,
				CmdStash, // backtrackSequence[i]
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			rule.backtrack = stashToGlyphs(s.GetStash())

			err = g.Exec(s,
				CmdRead16, TypeUInt, // inputGlyphCount
				CmdAssertGt, 0,
				CmdDec,
				CmdLoop,
				CmdStash, // inputSequence[i]
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			rule.input = stashToGlyphs(s.GetStash())

			err = g.Exec(s,
				CmdRead16, TypeUInt, // lookaheadGlyphCount
				CmdLoop,
				CmdStash, // lookaheadSequence[i]
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			rule.lookahead = stashToGlyphs(s.GetStash())

			err = g.Exec(s,
				CmdRead16, TypeUInt, // seqLookupCount
				CmdLoop,
				CmdStash, // seqLookupRecord.sequenceIndex
				CmdStash, // seqLookupRecord.lookupListIndex
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			seqLookupRecord := s.GetStash()

			for len(seqLookupRecord) > 0 {
				l, err := g.getGtabLookup(seqLookupRecord[1])
				if err != nil {
					return nil, err
				}
				rule.actions = append(rule.actions, seqLookup{
					pos:    int(seqLookupRecord[0]),
					nested: l,
				})
				seqLookupRecord = seqLookupRecord[2:]
			}

			ruleset = append(ruleset, rule)
		}
		xxx.rulesets = append(xxx.rulesets, ruleset)
	}
	return xxx, nil
}

func (l *chainedSeq1) Apply(filter filter, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	ruleNo := l.cov[seq[pos].Gid]
	if ruleNo >= len(l.rulesets) || l.rulesets[ruleNo] == nil {
		return seq, -1
	}

	panic("not implemented")
}

// Chained Sequence Context Format 2: class-based glyph contexts
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#chained-sequence-context-format-2-class-based-glyph-contexts
type chainedSeq2 struct {
	cov       coverage
	backtrack ClassDef
	input     ClassDef
	lookahead ClassDef
	rulesets  []chainedClassSeqRuleSet // indexed by the input class, can be nil
}

type chainedClassSeqRuleSet []*chainedClassSeqRule

type chainedClassSeqRule struct {
	backtrack []int
	input     []int
	lookahead []int
	actions   []seqLookup
}

func (g *gTab) readChained2(s *State, subtablePos int64) (*chainedSeq2, error) {
	err := g.Exec(s,
		CmdStash, // coverageOffset
		CmdStash, // backtrackClassDefOffset
		CmdStash, // inputClassDefOffset
		CmdStash, // lookaheadClassDefOffset

		CmdRead16, TypeUInt, // chainedClassSeqRuleSetCount
		CmdLoop,
		CmdStash, // chainedClassSeqRuleSetOffset[i]
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	stash := s.GetStash()
	coverageOffset := stash[0]
	backtrackClassDefOffset := stash[1]
	inputClassDefOffset := stash[2]
	lookaheadClassDefOffset := stash[3]
	chainedClassSeqRuleSetOffsets := stash[4:]

	cov, err := g.readCoverageTable(subtablePos + int64(coverageOffset))
	if err != nil {
		return nil, err
	}
	backtrack, err := g.readClassDefTable(subtablePos + int64(backtrackClassDefOffset))
	if err != nil {
		return nil, err
	}
	input, err := g.readClassDefTable(subtablePos + int64(inputClassDefOffset))
	if err != nil {
		return nil, err
	}
	lookahead, err := g.readClassDefTable(subtablePos + int64(lookaheadClassDefOffset))
	if err != nil {
		return nil, err
	}

	res := &chainedSeq2{
		cov:       cov,
		backtrack: backtrack,
		input:     input,
		lookahead: lookahead,
	}

	for _, offs := range chainedClassSeqRuleSetOffsets {
		if offs == 0 {
			res.rulesets = append(res.rulesets, nil)
			continue
		}

		chainedClassSequenceRuleSetPos := subtablePos + int64(offs)
		s.A = chainedClassSequenceRuleSetPos
		err = g.Exec(s,
			CmdSeek,
			CmdRead16, TypeUInt, // chainedClassSeqRuleCount
			CmdLoop,
			CmdStash, // chainedClassSeqRuleOffsets[i]
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}

		var ruleset chainedClassSeqRuleSet
		chainedClassSeqRuleOffsets := s.GetStash()
		for _, offs := range chainedClassSeqRuleOffsets {
			rule := &chainedClassSeqRule{}

			s.A = chainedClassSequenceRuleSetPos + int64(offs)
			err = g.Exec(s,
				CmdSeek,
				CmdRead16, TypeUInt, // backtrackGlyphCount
				CmdLoop,
				CmdStash, // backtrackSequence[i]
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			backtrackSequence := s.GetStash()
			for _, class := range backtrackSequence {
				rule.backtrack = append(rule.backtrack, int(class))
			}

			err = g.Exec(s,
				CmdRead16, TypeUInt, // inputGlyphCount
				CmdAssertGt, 0,
				CmdDec,
				CmdLoop,
				CmdStash, // inputSequence[i]
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			inputSequence := s.GetStash()
			for _, class := range inputSequence {
				rule.input = append(rule.input, int(class))
			}

			err = g.Exec(s,
				CmdRead16, TypeUInt, // lookaheadGlyphCount
				CmdLoop,
				CmdStash, // lookaheadSequence[i]
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			lookaheadSequence := s.GetStash()
			for _, class := range lookaheadSequence {
				rule.lookahead = append(rule.lookahead, int(class))
			}

			err = g.Exec(s,
				CmdRead16, TypeUInt, // seqLookupCount
				CmdLoop,
				CmdStash, // seqLookupRecord.sequenceIndex
				CmdStash, // seqLookupRecord.lookupListIndex
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			seqLookupRecord := s.GetStash()

			for len(seqLookupRecord) > 0 {
				l, err := g.getGtabLookup(seqLookupRecord[1])
				if err != nil {
					return nil, err
				}
				rule.actions = append(rule.actions, seqLookup{
					pos:    int(seqLookupRecord[0]),
					nested: l,
				})
				seqLookupRecord = seqLookupRecord[2:]
			}

			ruleset = append(ruleset, rule)
		}
		res.rulesets = append(res.rulesets, ruleset)
	}
	return res, nil
}

func (l *chainedSeq2) Apply(filter filter, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	gid := seq[pos].Gid
	if _, ok := l.cov[gid]; !ok {
		return seq, -1
	}

	class := l.input[gid]
	if class >= len(l.rulesets) {
		return seq, -1
	}

ruleLoop:
	for _, rule := range l.rulesets[class] {
		next := pos + 1 + len(rule.input)
		if pos < len(rule.backtrack) || next+len(rule.lookahead) > len(seq) {
			continue
		}

		for i, class := range rule.backtrack {
			if !filter(seq[pos-1-i].Gid) {
				panic("not implemented")
			}
			if l.backtrack[seq[pos-1-i].Gid] != class {
				continue ruleLoop
			}
		}
		for i, class := range rule.input {
			if !filter(seq[pos+1+i].Gid) {
				panic("not implemented")
			}
			if l.input[seq[pos+1+i].Gid] != class {
				continue ruleLoop
			}
		}
		for i, class := range rule.lookahead {
			if !filter(seq[next+i].Gid) {
				panic("not implemented")
			}
			if l.lookahead[seq[next+i].Gid] != class {
				continue ruleLoop
			}
		}

		seq = applyActions(rule.actions, pos, seq)
		return seq, next
	}
	return seq, -1
}

// Chained Contexts Substitution Format 3: Coverage-based Glyph Contexts
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#63-chained-contexts-substitution-format-3-coverage-based-glyph-contexts
type chainedSeq3 struct {
	backtrack []coverage
	input     []coverage
	lookahead []coverage
	actions   []seqLookup
}

type seqLookup struct {
	pos    int
	nested *lookupTable
}

func (g *gTab) readChained3(s *State, subtablePos int64) (*chainedSeq3, error) {
	err := g.Exec(s,
		CmdRead16, TypeUInt, // backtrackGlyphCount
		CmdLoop,
		CmdStash, // backtrackCoverageOffset
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	backtrackCoverageOffsets := s.GetStash()

	err = g.Exec(s,
		CmdRead16, TypeUInt, // inputGlyphCount
		CmdLoop,
		CmdStash, // inputCoverageOffset
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	inputCoverageOffsets := s.GetStash()

	err = g.Exec(s,
		CmdRead16, TypeUInt, // lookaheadGlyphCount
		CmdLoop,
		CmdStash, // lookaheadCoverageOffset
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	lookaheadCoverageOffsets := s.GetStash()

	err = g.Exec(s,
		CmdRead16, TypeUInt, // seqLookupCount
		CmdLoop,
		CmdStash, // seqLookupRecord.sequenceIndex
		CmdStash, // seqLookupRecord.lookupListIndex
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	seqLookupRecord := s.GetStash()

	xxx := &chainedSeq3{}

	for _, offs := range backtrackCoverageOffsets {
		cover, err := g.readCoverageTable(subtablePos + int64(offs))
		if err != nil {
			return nil, err
		}
		xxx.backtrack = append(xxx.backtrack, cover)
	}
	for _, offs := range inputCoverageOffsets {
		cover, err := g.readCoverageTable(subtablePos + int64(offs))
		if err != nil {
			return nil, err
		}
		xxx.input = append(xxx.input, cover)
	}
	for _, offs := range lookaheadCoverageOffsets {
		cover, err := g.readCoverageTable(subtablePos + int64(offs))
		if err != nil {
			return nil, err
		}
		xxx.lookahead = append(xxx.lookahead, cover)
	}
	for len(seqLookupRecord) > 0 {
		l, err := g.getGtabLookup(seqLookupRecord[1])
		if err != nil {
			return nil, err
		}
		xxx.actions = append(xxx.actions, seqLookup{
			pos:    int(seqLookupRecord[0]),
			nested: l,
		})
		seqLookupRecord = seqLookupRecord[2:]
	}
	return xxx, nil
}

func (l *chainedSeq3) Apply(filter filter, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	next := pos + len(l.input)
	if pos < len(l.backtrack) || next+len(l.lookahead) > len(seq) {
		return seq, -1
	}

	for i, cov := range l.backtrack {
		if !filter(seq[pos-1-i].Gid) {
			panic("not implemented")
		}
		_, ok := cov[seq[pos-1-i].Gid]
		if !ok {
			return seq, -1
		}
	}
	for i, cov := range l.input {
		if !filter(seq[pos+i].Gid) {
			panic("not implemented")
		}
		_, ok := cov[seq[pos+i].Gid]
		if !ok {
			return seq, -1
		}
	}
	for i, cov := range l.lookahead {
		if !filter(seq[next+i].Gid) {
			panic("not implemented")
		}
		_, ok := cov[seq[next+i].Gid]
		if !ok {
			return seq, -1
		}
	}

	seq = applyActions(l.actions, pos, seq)
	return seq, next
}

func applyActions(actions []seqLookup, pos int, seq []font.Glyph) []font.Glyph {
	origLen := len(seq)
	for _, action := range actions {
		seq, _ = action.nested.applySubtables(seq, pos+action.pos)
		if len(seq) != origLen {
			// TODO(voss): how to interpret action.pos in case a prior action
			// changes the length of seq?
			panic("not implemented: nested lookup changed sequence length")
		}
	}
	return seq
}
