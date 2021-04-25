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
	"strconv"
	"strings"

	"seehuhn.de/go/pdf/font"
)

// http://adobe-type-tools.github.io/afdko/OpenTypeFeatureFileSpecification.html#7
// TODO(voss): read this carefully

type gsubLookup []gsubLookupSubtable

func (l gsubLookup) Substitute(glyphs []font.GlyphID) []font.GlyphID {
	pos := 0
glyphLoop:
	for pos < len(glyphs) {
		var next int
		for _, subtable := range l {
			next, glyphs = subtable.Replace(pos, glyphs)
			if next > pos {
				pos = next
				continue glyphLoop
			}
		}
		pos++
	}
	return glyphs
}

func (g *gTab) getGsubLookup(idx uint16, pfx string) (gsubLookup, error) {
	if int(idx) >= len(g.lookups) {
		return nil, g.error("lookup index %d out of range", idx)
	}
	base := g.lookupListOffset + int64(g.lookups[idx])

	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-table
	s := &State{}
	s.A = base
	err := g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // lookupType
		CmdStash,
		CmdRead16, TypeUInt, // lookupFlag
		CmdStash,
		CmdRead16, TypeUInt, // subtableCount
		CmdLoop,
		CmdRead16, TypeUInt, // subtableOffset
		CmdStash,
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	data := s.GetStash()
	format := data[0]
	flags := data[1]
	subtables := data[2:]
	markFilteringSet := -1
	if flags&0x0010 != 0 {
		err = g.Exec(s, CmdRead16, TypeUInt) // markFilteringSet
		if err != nil {
			return nil, err
		}
		markFilteringSet = int(s.A)
	}
	_ = markFilteringSet // TODO(voss): use this correctly

	var lookup gsubLookup
	for _, inc := range subtables {
		subtablePos := base + int64(inc)

		res, err := g.readGsubSubtable(s, flags, format, subtablePos, pfx)
		if err != nil {
			return nil, err
		}

		lookup = append(lookup, res)
	}

	return lookup, nil
}

func (g *gTab) readGsubSubtable(s *State, flags, format uint16, subtablePos int64, pfx string) (gsubLookupSubtable, error) {
	// TODO(voss): is this called more than once for the same subtablePos?6_3 -> use caching?
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
		err = g.Exec(s,
			CmdRead16, TypeUInt, // coverageOffset
			CmdStore, 0,
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
			flags: flags,
			cov:   cov,
			delta: delta,
		}

	case 1_2: // Single Substitution Format 2
		err = g.Exec(s,
			CmdRead16, TypeUInt, // coverageOffset
			CmdStore, 0,
			CmdRead16, TypeUInt, // glyphCount
			CmdLoop,
			CmdRead16, TypeUInt, // substitutefont.GlyphIndex[i]
			CmdStash,
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
			flags:              flags,
			cov:                cov,
			substituteGlyphIDs: stashToGlyphs(repl),
		}

	case 2_1: // Multiple Substitution Subtable
		err = g.Exec(s,
			CmdRead16, TypeUInt, // coverageOffset
			CmdStore, 0,
			CmdRead16, TypeUInt, // sequenceCount
			CmdLoop,
			CmdRead16, TypeUInt, // sequenceOffset[i]
			CmdStash,
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
				CmdRead16, TypeUInt, // substituteGlyphID[j]
				CmdStash,
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			repl[i] = stashToGlyphs(s.GetStash())
		}
		res = &gsub2_1{
			flags: flags,
			cov:   cov,
			repl:  repl,
		}

	case 4_1: // Ligature Substitution Subtable
		err = g.Exec(s,
			CmdRead16, TypeUInt, // coverageOffset
			CmdStore, 0,
			CmdRead16, TypeUInt, // ligatureSetCount
			CmdLoop,
			CmdRead16, TypeUInt, // ligatureSetOffset[i]
			CmdStash,
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
			flags: flags,
			cov:   cov,
			repl:  make([][]ligature, len(ligatureSetOffsets)),
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
				CmdRead16, TypeUInt, // ligatureOffset[i]
				CmdStash,
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
					CmdRead16, TypeUInt, // ligatureGlyph
					CmdStash,
					CmdRead16, TypeUInt, // componentCount
					CmdDec,
					CmdLoop,
					CmdRead16, TypeUInt, // componentfont.GlyphIndex[i]
					CmdStash,
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

	case 6_2: // Chained Contexts Substitution: Class-based Glyph Contexts
		err = g.Exec(s,
			CmdLoadI, 4,
			CmdLoop,
			CmdRead16, TypeUInt, // coverageOffset, backtrackClassDefOffset, inputClassDefOffset, lookaheadClassDefOffset
			CmdStash,
			CmdEndLoop,

			CmdRead16, TypeUInt, // chainedClassSeqRuleSetCount
			CmdLoop,
			CmdRead16, TypeUInt, // chainedClassSeqRuleSetOffset[i]
			CmdStash,
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
		input, err := g.readClassDefTable(subtablePos + int64(inputClassDefOffset))
		lookahead, err := g.readClassDefTable(subtablePos + int64(lookaheadClassDefOffset))

		xxx := &gsub6_2{
			flags:     flags,
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
				CmdRead16, TypeUInt, // chainedClassSeqRuleOffsets[i]
				CmdStash,
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}

			var ruleset seqRuleSet
			chainedClassSeqRuleOffsets := s.GetStash()
			for _, offs := range chainedClassSeqRuleOffsets {
				rule := &seqRule{}

				s.A = chainedClassSequenceRuleSetPos + int64(offs)
				err = g.Exec(s,
					CmdSeek,
					CmdRead16, TypeUInt, // backtrackGlyphCount
					CmdLoop,
					CmdRead16, TypeUInt, // backtrackSequence[i]
					CmdStash,
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
					CmdRead16, TypeUInt, // inputSequence[i]
					CmdStash,
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
					CmdRead16, TypeUInt, // lookaheadSequence[i]
					CmdStash,
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
					CmdRead16, TypeUInt, // seqLookupRecord.sequenceIndex
					CmdStash,
					CmdRead16, TypeUInt, // seqLookupRecord.lookupListIndex
					CmdStash,
					CmdEndLoop,
				)
				if err != nil {
					return nil, err
				}
				seqLookupRecord := s.GetStash()

				for len(seqLookupRecord) > 0 {
					l, err := g.getGsubLookup(seqLookupRecord[1], "-")
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
		err = g.Exec(s,
			CmdRead16, TypeUInt, // backtrackGlyphCount
			CmdLoop,
			CmdRead16, TypeUInt, // backtrackCoverageOffset
			CmdStash,
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		backtrackCoverageOffsets := s.GetStash()

		err = g.Exec(s,
			CmdRead16, TypeUInt, // inputGlyphCount
			CmdLoop,
			CmdRead16, TypeUInt, // inputCoverageOffset
			CmdStash,
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		inputCoverageOffsets := s.GetStash()

		err = g.Exec(s,
			CmdRead16, TypeUInt, // lookaheadGlyphCount
			CmdLoop,
			CmdRead16, TypeUInt, // lookaheadCoverageOffset
			CmdStash,
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		lookaheadCoverageOffsets := s.GetStash()

		err = g.Exec(s,
			CmdRead16, TypeUInt, // seqLookupCount
			CmdLoop,
			CmdRead16, TypeUInt, // seqLookupRecord.sequenceIndex
			CmdStash,
			CmdRead16, TypeUInt, // seqLookupRecord.lookupListIndex
			CmdStash,
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		seqLookupRecord := s.GetStash()

		xxx := &gsub6_3{
			flags: flags,
		}

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
			l, err := g.getGsubLookup(seqLookupRecord[1], "-")
			if err != nil {
				return nil, err
			}
			xxx.actions = append(xxx.actions, seqLookup{
				pos:    int(seqLookupRecord[0]),
				nested: l,
			})
			seqLookupRecord = seqLookupRecord[2:]
		}
		res = xxx
	}

	if res == nil {
		fmt.Printf("%sunsupported lookup type %d.%d\n", pfx, format, subFormat)
	} else if pfx != "-" {
		res.Explain(g, pfx)
	}
	fmt.Println()

	return res, nil
}

func stashToGlyphs(x []uint16) []font.GlyphID {
	res := make([]font.GlyphID, len(x))
	for i, gid := range x {
		res[i] = font.GlyphID(gid)
	}
	return res
}

type gsubLookupSubtable interface {
	Replace(int, []font.GlyphID) (int, []font.GlyphID)
	Explain(g *gTab, pfx string)
}

type gsub1_1 struct {
	flags uint16
	cov   coverage
	delta uint16
}

func (l *gsub1_1) Replace(i int, seq []font.GlyphID) (int, []font.GlyphID) {
	if l.flags != 0 {
		panic("not implemented")
	}
	in := seq[i]
	_, ok := l.cov[in]
	if !ok {
		return i, seq
	}
	seq[i] = font.GlyphID(uint16(seq[i]) + l.delta)
	return i + 1, seq
}

func (l *gsub1_1) Explain(g *gTab, pfx string) {
	fmt.Printf(pfx+"lookup type 1.1, flags=0x%04x\n", l.flags)
	for gid, _ := range l.cov {
		repl := font.GlyphID(uint16(gid) + l.delta)
		fmt.Printf("%s\t%s -> %s\n",
			pfx, g.glyphName(gid), g.glyphName(repl))
	}
}

type gsub1_2 struct {
	flags              uint16
	cov                coverage
	substituteGlyphIDs []font.GlyphID
}

func (l *gsub1_2) Replace(i int, seq []font.GlyphID) (int, []font.GlyphID) {
	if l.flags != 0 {
		panic("not implemented")
	}
	in := seq[i]
	j, ok := l.cov[in]
	if !ok {
		return i, seq
	}
	seq[i] = l.substituteGlyphIDs[j]
	return i + 1, seq
}

func (l *gsub1_2) Explain(g *gTab, pfx string) {
	fmt.Printf(pfx+"lookup type 1.2, flags=0x%04x\n", l.flags)
	for gid, k := range l.cov {
		fmt.Printf("%s\t%s -> %s\n",
			pfx, g.glyphName(gid), g.glyphName(l.substituteGlyphIDs[k]))
	}
}

type gsub2_1 struct {
	flags uint16
	cov   coverage
	repl  [][]font.GlyphID
}

func (l *gsub2_1) Replace(i int, seq []font.GlyphID) (int, []font.GlyphID) {
	if l.flags != 0 {
		panic("not implemented")
	}
	k, ok := l.cov[seq[i]]
	if !ok {
		return i, seq
	}
	repl := l.repl[k]
	n := len(repl)

	res := append(seq, repl...) // just to allocate enough backing space
	copy(seq[i+n:], seq[i:])
	copy(seq[i:], repl)
	return i + n, res
}

func (l *gsub2_1) Explain(g *gTab, pfx string) {
	fmt.Printf(pfx+"lookup type 2.1, flags=0x%04x\n", l.flags)
	for inGid, k := range l.cov {
		var out []string
		for _, outGid := range l.repl[k] {
			out = append(out, g.glyphName(outGid))
		}
		fmt.Printf("%s\t%s -> %s\n",
			pfx, g.glyphName(inGid), strings.Join(out, ""))
	}
}

type gsub4_1 struct {
	flags uint16
	cov   coverage // maps first glyphs to repl indices
	repl  [][]ligature
}

type ligature struct {
	in  []font.GlyphID // excludes the first input glyph, since this is in cov
	out font.GlyphID
}

func (l *gsub4_1) Replace(i int, seq []font.GlyphID) (int, []font.GlyphID) {
	if l.flags != 0 {
		panic("not implemented")
	}
	ligSetIdx, ok := l.cov[seq[i]]
	if !ok {
		return i, seq
	}
	ligSet := l.repl[ligSetIdx]

ligLoop:
	for j := range ligSet {
		lig := &ligSet[j]
		if i+1+len(lig.in) > len(seq) {
			continue
		}
		for k, gid := range lig.in {
			if seq[i+1+k] != gid {
				continue ligLoop
			}
		}

		seq[i] = lig.out
		seq = append(seq[:i+1], seq[i+1+len(lig.in):]...)
		return i + 1, seq
	}

	return i, seq
}

func (l *gsub4_1) Explain(g *gTab, pfx string) {
	fmt.Printf(pfx+"lookup type 4.1, flags=0x%04x\n", l.flags)
	for inGid, k := range l.cov {
		ligSet := l.repl[k]
		for j := range ligSet {
			lig := &ligSet[j]
			in := []string{g.glyphName(inGid)}
			for _, gid := range lig.in {
				in = append(in, g.glyphName(gid))
			}
			fmt.Printf("%s\t%s -> %s\n",
				pfx, strings.Join(in, ""), g.glyphName(lig.out))
		}
	}
}

type gsub6_2 struct {
	flags     uint16
	cov       coverage
	backtrack classDef
	input     classDef
	lookahead classDef
	rulesets  []seqRuleSet // indexed by the input class, can be nil
}

type seqRuleSet []*seqRule

type seqRule struct {
	backtrack []int
	input     []int
	lookahead []int
	actions   []seqLookup
}

func (l *gsub6_2) Replace(pos int, seq []font.GlyphID) (int, []font.GlyphID) {
	if l.flags != 0 {
		panic("not implemented")
	}

	ruleNo := l.cov[seq[pos]]
	if ruleNo >= len(l.rulesets) || l.rulesets[ruleNo] == nil {
		return pos, seq
	}

	panic("not implemented")
}

func (l *gsub6_2) Explain(g *gTab, pfx string) {
	fmt.Printf(pfx+"lookup type 6.2, flags=0x%04x\n", l.flags)
	pfx = pfx + "\t"

	fmt.Println(pfx+"Cov", g.explainCoverage(l.cov))
	fmt.Println(pfx+"Bc", l.backtrack)
	fmt.Println(pfx+"Ic", l.input)
	fmt.Println(pfx+"Lc", l.lookahead)

	for i, ruleset := range l.rulesets {
		for j, rule := range ruleset {
			fmt.Println(pfx+"b"+strconv.Itoa(i)+"."+strconv.Itoa(j),
				rule.backtrack)
			fmt.Println(pfx+"i"+strconv.Itoa(i)+"."+strconv.Itoa(j),
				"cov +", rule.input)
			fmt.Println(pfx+"l"+strconv.Itoa(i)+"."+strconv.Itoa(j),
				rule.lookahead)
			apfx := pfx + "@" + strconv.Itoa(i) + "." + strconv.Itoa(j)
			for _, action := range rule.actions {
				for _, subtable := range action.nested {
					subtable.Explain(g, apfx)
				}
			}
		}
	}
}

type gsub6_3 struct {
	flags     uint16
	backtrack []coverage
	input     []coverage
	lookahead []coverage
	actions   []seqLookup
}

type seqLookup struct {
	pos    int
	nested gsubLookup
}

func (l *gsub6_3) Replace(pos int, seq []font.GlyphID) (int, []font.GlyphID) {
	if l.flags != 0 {
		panic("not implemented")
	}

	if pos < len(l.backtrack) || pos+len(l.input)+len(l.lookahead) >= len(seq) {
		return pos, seq
	}

	for i, cov := range l.backtrack {
		_, ok := cov[seq[pos-1-i]]
		if !ok {
			return pos, seq
		}
	}
	for i, cov := range l.input {
		_, ok := cov[seq[pos+i]]
		if !ok {
			return pos, seq
		}
	}
	for i, cov := range l.lookahead {
		_, ok := cov[seq[pos+len(l.input)+i]]
		if !ok {
			return pos, seq
		}
	}

	origLen := len(seq)
	for _, action := range l.actions {
		// TODO(voss): how to interpret action.pos in case a prior action
		// changes the length of seq?
		aPos := pos + action.pos
		origAPos := aPos
		subtables := action.nested
		for _, subtable := range subtables {
			aPos, seq = subtable.Replace(aPos, seq)
			if aPos != origAPos {
				// as soon as one subtable matches, we are done
				break
			}
		}
		if len(seq) != origLen {
			panic("not implemented: nested lookup changed sequence length")
		}
	}

	return pos + len(l.input), seq
}

func (l *gsub6_3) Explain(g *gTab, pfx string) {
	fmt.Printf(pfx+"lookup type 6.3, flags=0x%04x\n", l.flags)
	pfx = pfx + "\t"
	for i := len(l.backtrack) - 1; i >= 0; i-- {
		fmt.Println(pfx+"B", g.explainCoverage(l.backtrack[i]))
	}
	for _, cov := range l.input {
		fmt.Println(pfx+"I", g.explainCoverage(cov))
	}
	for _, cov := range l.lookahead {
		fmt.Println(pfx+"L", g.explainCoverage(cov))
	}
	for _, action := range l.actions {
		for _, subtable := range action.nested {
			subtable.Explain(g, pfx+"@"+strconv.Itoa(action.pos))
		}
	}
}
