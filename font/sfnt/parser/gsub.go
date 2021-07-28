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
	"seehuhn.de/go/pdf/font"
)

// The most common GSUB features seen on my system:
//     5630 "liga"
//     4185 "frac"
//     3857 "aalt"
//     3746 "onum"
//     3434 "sups"
//     3010 "lnum"
//     2992 "pnum"
//     2989 "ccmp"
//     2976 "dnom"
//     2962 "numr"

// http://adobe-type-tools.github.io/afdko/OpenTypeFeatureFileSpecification.html#7
// TODO(voss): read this carefully

// GsubInfo represents the information from the "GSUB" table of a font.
type GsubInfo []*gsubLookup

type gsubLookup struct {
	Format uint16 // TODO(voss): remove?

	rtl    bool
	filter filter

	subtables        []gsubLookupSubtable
	markFilteringSet uint16
}

type gsubLookupSubtable interface {
	// Replace attempts to apply a single subtable at the given position.
	// If returns the new glyphs and the new position.  The new position
	// will be larger than pos, if and only if a substitution has been made.
	//
	// TODO(voss): how to signal that a deletion occured?
	Replace(filter filter, glyphs []font.Glyph, pos int) ([]font.Glyph, int)
}

// ReadGsubInfo reads the "GSUB" table of a font, for a given writing script
// and language.
func (p *Parser) ReadGsubInfo(script, lang string, extraFeatures ...string) (GsubInfo, error) {
	gtab, err := newGTab(p, script, lang)
	if err != nil {
		return nil, err
	}

	includeFeature := map[string]bool{
		"ccmp": true,
		"liga": true,
		"clig": true,
	}
	for _, feature := range extraFeatures {
		includeFeature[feature] = true
	}
	err = gtab.Init("GSUB", includeFeature)
	if err != nil {
		return nil, err
	}

	var res GsubInfo
	for _, idx := range gtab.LookupIndices {
		l, err := gtab.getGsubLookup(idx)
		if err != nil {
			return nil, err
		}
		res = append(res, l)
	}
	return res, nil
}

// Substitute applies transformations from the selected GSUB lookups to a
// series of glyphs.
func (gsub GsubInfo) Substitute(glyphs []font.Glyph) []font.Glyph {
	for _, l := range gsub {
		glyphs = l.substitute(glyphs)
	}
	return glyphs
}

func (l *gsubLookup) substitute(glyphs []font.Glyph) []font.Glyph {
	pos := 0
	for pos < len(glyphs) {
		var next int
		glyphs, next = l.substituteOne(glyphs, pos)
		if next > pos {
			pos = next
		} else {
			pos++
		}
	}
	return glyphs
}

func (l *gsubLookup) substituteOne(glyphs []font.Glyph, pos int) ([]font.Glyph, int) {
	for _, subtable := range l.subtables {
		glyphs, next := subtable.Replace(l.filter, glyphs, pos)
		if next > pos {
			return glyphs, next
		}
	}
	return glyphs, pos
}

func (g *gTab) getGsubLookup(idx uint16) (*gsubLookup, error) {
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

	lookup := &gsubLookup{
		Format:           format,
		rtl:              flags&0x0001 != 0,
		filter:           g.makeFilter(flags),
		markFilteringSet: markFilteringSet,
	}
	for _, offs := range subtables {
		res, err := g.readGsubSubtable(s, format, base+int64(offs))
		if err != nil {
			return nil, err
		}

		if res != nil {
			lookup.subtables = append(lookup.subtables, res)
		}
	}

	return lookup, nil
}

func (g *gTab) readGsubSubtable(s *State, format uint16, subtablePos int64) (gsubLookupSubtable, error) {
	// TODO(voss): is this called more than once for the same subtablePos? -> use caching?
	s.A = subtablePos
	err := g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // format
	)
	if err != nil {
		return nil, err
	}
	subFormat := s.A

	var res gsubLookupSubtable

	// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#table-organization
	switch 10*format + uint16(subFormat) {
	case 1_1: // Single Substitution Format 1
		err := g.Exec(s,
			CmdRead16, TypeUInt, // coverageOffset
			CmdStoreInto, 0,
			CmdRead16, TypeUInt, // deltaGlyphID (mod 65536)
		)
		if err != nil {
			return nil, err
		}
		delta := uint16(s.A)
		cov, err := g.readCoverageTable(subtablePos + s.R[0])
		if err != nil {
			return nil, err
		}
		res = &gsub1_1{
			cov:   cov,
			delta: delta,
		}

	case 1_2: // Single Substitution Format 2
		err = g.Exec(s,
			CmdRead16, TypeUInt, // coverageOffset
			CmdStoreInto, 0,
			CmdRead16, TypeUInt, // glyphCount
			CmdLoop,
			CmdStash, // substitutefont.GlyphIndex[i]
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		repl := s.GetStash()
		cov, err := g.readCoverageTable(subtablePos + s.R[0])
		if err != nil {
			return nil, err
		}
		err = cov.check(len(repl))
		if err != nil {
			return nil, err
		}
		res = &gsub1_2{
			cov:                cov,
			substituteGlyphIDs: stashToGlyphs(repl),
		}

	case 2_1: // Multiple Substitution Subtable
		err = g.Exec(s,
			CmdRead16, TypeUInt, // coverageOffset
			CmdStoreInto, 0,
			CmdRead16, TypeUInt, // sequenceCount
			CmdLoop,
			CmdStash, // sequenceOffset[i]
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		sequenceOffsets := s.GetStash()
		cov, err := g.readCoverageTable(subtablePos + s.R[0])
		if err != nil {
			return nil, err
		}
		err = cov.check(len(sequenceOffsets))
		if err != nil {
			return nil, err
		}
		repl := make([][]font.GlyphID, len(sequenceOffsets))
		for _, i := range cov {
			s.A = subtablePos + int64(sequenceOffsets[i])
			err = g.Exec(s,
				CmdSeek,
				CmdRead16, TypeUInt, // glyphCount
				CmdLoop,
				CmdStash, // substituteGlyphID[j]
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			repl[i] = stashToGlyphs(s.GetStash())
		}
		res = &gsub2_1{
			cov:  cov,
			repl: repl,
		}

	case 4_1: // Ligature Substitution Subtable
		err = g.Exec(s,
			CmdRead16, TypeUInt, // coverageOffset
			CmdStoreInto, 0,
			CmdRead16, TypeUInt, // ligatureSetCount
			CmdLoop,
			CmdStash, // ligatureSetOffset[i]
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		ligatureSetOffsets := s.GetStash()
		cov, err := g.readCoverageTable(subtablePos + s.R[0])
		if err != nil {
			return nil, err
		}
		xxx := &gsub4_1{
			cov:  cov,
			repl: make([][]ligature, len(ligatureSetOffsets)),
		}
		for _, i := range cov {
			if i >= len(ligatureSetOffsets) {
				return nil, g.error("ligatureSetOffset %d out of range", i)
			}
			ligSetTablePos := subtablePos + int64(ligatureSetOffsets[i])
			s.A = ligSetTablePos
			err = g.Exec(s,
				CmdSeek,
				CmdRead16, TypeUInt, // ligatureCount
				CmdLoop,
				CmdStash, // ligatureOffset[i]
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			ligatureOffsets := s.GetStash()
			for _, o2 := range ligatureOffsets {
				s.A = ligSetTablePos + int64(o2)
				err = g.Exec(s,
					CmdSeek,
					CmdStash,            // ligatureGlyph
					CmdRead16, TypeUInt, // componentCount
					CmdAssertGt, 0,
					CmdDec,
					CmdLoop,
					CmdStash, // componentfont.GlyphIndex[i]
					CmdEndLoop,
				)
				if err != nil {
					return nil, err
				}
				xx := s.GetStash()

				xxx.repl[i] = append(xxx.repl[i], ligature{
					in:  stashToGlyphs(xx[1:]),
					out: font.GlyphID(xx[0]),
				})
			}
		}
		res = xxx

	case 5_2: // Context Substitution: Class-based Glyph Contexts
		err = g.Exec(s,
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

		xxx := &gsub5_2{
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
					CmdLoad, 0,
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
					CmdLoad, 1,
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
					l, err := g.getGsubLookup(seqLookupRecord[1])
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

		res = xxx

	case 6_1: // Chained Contexts Substitution: Simple Glyph Contexts
		err = g.Exec(s,
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

		xxx := &gsub6_1{
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
					l, err := g.getGsubLookup(seqLookupRecord[1])
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
		res = xxx

	case 6_2: // Chained Contexts Substitution: Class-based Glyph Contexts
		err = g.Exec(s,
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

		xxx := &gsub6_2{
			cov:       cov,
			backtrack: backtrack,
			input:     input,
			lookahead: lookahead,
		}

		for _, offs := range chainedClassSeqRuleSetOffsets {
			if offs == 0 {
				xxx.rulesets = append(xxx.rulesets, nil)
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
					l, err := g.getGsubLookup(seqLookupRecord[1])
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
		res = xxx

	case 6_3: // Chained Contexts Substitution: Coverage-based Glyph Contexts
		return g.readGsub6_3(s, subtablePos)

	case 7_1: // Extension Substitution Subtable Format 1
		err = g.Exec(s,
			CmdRead16, TypeUInt, // extensionLookupType
			CmdStoreInto, 0,
			CmdRead32, TypeUInt, // extensionOffset
		)
		if err != nil {
			return nil, err
		}
		if s.R[0] == 7 {
			return nil, g.error("invalid extension lookup")
		}
		return g.readGsubSubtable(s, uint16(s.R[0]), subtablePos+s.A)
	}

	if res == nil {
		return nil, g.error("unsupported lookup type %d.%d\n", format, subFormat)
	}

	return res, nil
}

func stashToGlyphs(x []uint16) []font.GlyphID {
	res := make([]font.GlyphID, len(x))
	for i, gid := range x {
		res[i] = font.GlyphID(gid)
	}
	return res
}

type gsub1_1 struct {
	cov   coverage
	delta uint16
}

func (l *gsub1_1) Replace(filter filter, seq []font.Glyph, i int) ([]font.Glyph, int) {
	gid := seq[i].Gid
	if _, ok := l.cov[gid]; !ok {
		return seq, i
	}
	seq[i].Gid = font.GlyphID(uint16(gid) + l.delta)
	return seq, i + 1
}

type gsub1_2 struct {
	cov                coverage
	substituteGlyphIDs []font.GlyphID
}

func (l *gsub1_2) Replace(filter filter, seq []font.Glyph, i int) ([]font.Glyph, int) {
	glyph := seq[i]
	j, ok := l.cov[glyph.Gid]
	if !ok {
		return seq, i
	}
	seq[i].Gid = l.substituteGlyphIDs[j]
	return seq, i + 1
}

// Multiple Substitution Subtable
type gsub2_1 struct {
	cov  coverage
	repl [][]font.GlyphID
}

func (l *gsub2_1) Replace(filter filter, seq []font.Glyph, i int) ([]font.Glyph, int) {
	k, ok := l.cov[seq[i].Gid]
	if !ok {
		return seq, i
	}
	replGid := l.repl[k]

	n := len(replGid)
	repl := make([]font.Glyph, n)
	for i, gid := range replGid {
		repl[i].Gid = gid
	}
	if n > 0 {
		// TODO(voss): What should we do for n=0?
		repl[i].Chars = seq[i].Chars
	}

	res := append(seq, repl...) // just to allocate enough backing space
	copy(seq[i:], repl)
	copy(seq[i+n:], seq[i+1:])
	return res, i + n
}

// Ligature Substitution Subtable
type gsub4_1 struct {
	cov  coverage // maps first glyphs to repl indices
	repl [][]ligature
}

type ligature struct {
	in  []font.GlyphID // excludes the first input glyph, since this is in cov
	out font.GlyphID
}

func (l *gsub4_1) Replace(filter filter, seq []font.Glyph, i int) ([]font.Glyph, int) {
	ligSetIdx, ok := l.cov[seq[i].Gid]
	if !ok {
		return seq, i
	}
	ligSet := l.repl[ligSetIdx]

ligLoop:
	for j := range ligSet {
		lig := &ligSet[j]
		if i+1+len(lig.in) > len(seq) {
			continue
		}
		for k, gid := range lig.in {
			if !filter(seq[i+1+k].Gid) {
				panic("not implemented")
			}
			if seq[i+1+k].Gid != gid {
				continue ligLoop
			}
		}

		var chars []rune
		for j := i; j < i+len(lig.in); j++ {
			chars = append(chars, seq[j].Chars...)
		}
		seq[i] = font.Glyph{
			Gid:   lig.out,
			Chars: chars,
		}
		seq = append(seq[:i+1], seq[i+1+len(lig.in):]...)
		return seq, i + 1
	}

	return seq, i
}

type gsub5_2 struct {
	cov      coverage
	input    ClassDef
	rulesets []classSeqRuleSet // indexed by the input class, can be nil
}

type classSeqRuleSet []*classSequenceRule

type classSequenceRule struct {
	input   []int
	actions []seqLookup
}

func (l *gsub5_2) Replace(filter filter, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	glyph := seq[pos]
	if _, ok := l.cov[glyph.Gid]; !ok {
		return seq, pos
	}

	class := l.input[glyph.Gid]
	if class >= len(l.rulesets) {
		return seq, pos
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
	return seq, pos
}

type gsub6_1 struct {
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

func (l *gsub6_1) Replace(filter filter, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	ruleNo := l.cov[seq[pos].Gid]
	if ruleNo >= len(l.rulesets) || l.rulesets[ruleNo] == nil {
		return seq, pos
	}

	panic("not implemented")
}

type gsub6_2 struct {
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

func (l *gsub6_2) Replace(filter filter, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	gid := seq[pos].Gid
	if _, ok := l.cov[gid]; !ok {
		return seq, pos
	}

	class := l.input[gid]
	if class >= len(l.rulesets) {
		return seq, pos
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
	return seq, pos
}

// Chained Contexts Substitution Format 3: Coverage-based Glyph Contexts
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#63-chained-contexts-substitution-format-3-coverage-based-glyph-contexts
type gsub6_3 struct {
	backtrack []coverage
	input     []coverage
	lookahead []coverage
	actions   []seqLookup
}

type seqLookup struct {
	pos    int
	nested *gsubLookup
}

func (g *gTab) readGsub6_3(s *State, subtablePos int64) (*gsub6_3, error) {
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

	xxx := &gsub6_3{}

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
		l, err := g.getGsubLookup(seqLookupRecord[1])
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

func (l *gsub6_3) Replace(filter filter, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	next := pos + len(l.input)
	if pos < len(l.backtrack) || next+len(l.lookahead) > len(seq) {
		return seq, pos
	}

	for i, cov := range l.backtrack {
		if !filter(seq[pos-1-i].Gid) {
			panic("not implemented")
		}
		_, ok := cov[seq[pos-1-i].Gid]
		if !ok {
			return seq, pos
		}
	}
	for i, cov := range l.input {
		if !filter(seq[pos+i].Gid) {
			panic("not implemented")
		}
		_, ok := cov[seq[pos+i].Gid]
		if !ok {
			return seq, pos
		}
	}
	for i, cov := range l.lookahead {
		if !filter(seq[next+i].Gid) {
			panic("not implemented")
		}
		_, ok := cov[seq[next+i].Gid]
		if !ok {
			return seq, pos
		}
	}

	seq = applyActions(l.actions, pos, seq)
	return seq, next
}

func applyActions(actions []seqLookup, pos int, seq []font.Glyph) []font.Glyph {
	origLen := len(seq)
	for _, action := range actions {
		seq, _ = action.nested.substituteOne(seq, pos+action.pos)
		if len(seq) != origLen {
			// TODO(voss): how to interpret action.pos in case a prior action
			// changes the length of seq?
			panic("not implemented: nested lookup changed sequence length")
		}
	}
	return seq
}
