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
	"seehuhn.de/go/pdf/font/parser"
)

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

func (g *GTab) readSeqContext2(s *parser.State, subtablePos int64) (*seqContext2, error) {
	err := g.Exec(s,
		parser.CmdStash16, // coverageOffset
		parser.CmdStash16, // classDefOffset

		parser.CmdRead16, parser.TypeUInt, // classSeqRuleSetCount
		parser.CmdLoop,
		parser.CmdStash16, // classSeqRuleSetOffsets[i]
		parser.CmdEndLoop,
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

	res := &seqContext2{
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
			parser.CmdSeek,
			parser.CmdRead16, parser.TypeUInt, // classSeqRuleCount
			parser.CmdLoop,
			parser.CmdStash16, // classSeqRuleOffsets[i]
			parser.CmdEndLoop,
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
				parser.CmdSeek,
				parser.CmdRead16, parser.TypeUInt, // glyphCount
				parser.CmdStoreInto, 0,
				parser.CmdRead16, parser.TypeUInt, // seqLookupCount
				parser.CmdStoreInto, 1,
				parser.CmdLoadFrom, 0,
				parser.CmdAssertGt, 0,
				parser.CmdDec,
				parser.CmdLoop,
				parser.CmdStash16, // inputSequence[i]
				parser.CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			for _, class := range s.GetStash() {
				rule.input = append(rule.input, int(class))
			}

			err = g.Exec(s,
				parser.CmdLoadFrom, 1,
				parser.CmdLoop,
				parser.CmdStash16, // seqLookupRecord.sequenceIndex
				parser.CmdStash16, // seqLookupRecord.lookupListIndex
				parser.CmdEndLoop,
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
		res.rulesets[i] = ruleSet
	}
	return res, nil
}

func (l *seqContext2) Apply(filter KeepGlyphFn, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	glyph := seq[pos]
	if _, ok := l.cov[glyph.Gid]; !ok {
		return seq, -1
	}

	class0 := l.input[glyph.Gid]
	if class0 < 0 || class0 >= len(l.rulesets) {
		return seq, -1
	}
	rules := l.rulesets[class0]

	pp := []int{pos}
rulesetLoop:
	for _, rule := range rules {
		p := pos
		pp = pp[:1]
		for _, classI := range rule.input {
			p = filter.Next(seq, p)
			if p < 0 {
				return seq, -1
			}
			pp = append(pp, p)
			if l.input[seq[p].Gid] != classI {
				continue rulesetLoop
			}
		}

		seq = applyActions(rule.actions, pp, seq)
		return seq, p + 1
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

func (g *GTab) readChained1(s *parser.State, subtablePos int64) (*chainedSeq1, error) {
	err := g.Exec(s,
		parser.CmdRead16, parser.TypeUInt, // coverageOffset
		parser.CmdStoreInto, 0,
		parser.CmdRead16, parser.TypeUInt, // chainedSeqRuleSetCount
		parser.CmdLoop,
		parser.CmdStash16, // chainedSeqRuleSetOffsets[i]
		parser.CmdEndLoop,
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
			parser.CmdSeek,
			parser.CmdRead16, parser.TypeUInt, // chainedSeqRuleCount
			parser.CmdLoop,
			parser.CmdStash16, // chainedSeqRuleOffsets[i]
			parser.CmdEndLoop,
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
				parser.CmdSeek,
				parser.CmdRead16, parser.TypeUInt, // backtrackGlyphCount
				parser.CmdLoop,
				parser.CmdStash16, // backtrackSequence[i]
				parser.CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			rule.backtrack = stashToGlyphs(s.GetStash())

			err = g.Exec(s,
				parser.CmdRead16, parser.TypeUInt, // inputGlyphCount
				parser.CmdAssertGt, 0,
				parser.CmdDec,
				parser.CmdLoop,
				parser.CmdStash16, // inputSequence[i]
				parser.CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			rule.input = stashToGlyphs(s.GetStash())

			err = g.Exec(s,
				parser.CmdRead16, parser.TypeUInt, // lookaheadGlyphCount
				parser.CmdLoop,
				parser.CmdStash16, // lookaheadSequence[i]
				parser.CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			rule.lookahead = stashToGlyphs(s.GetStash())

			err = g.Exec(s,
				parser.CmdRead16, parser.TypeUInt, // seqLookupCount
				parser.CmdLoop,
				parser.CmdStash16, // seqLookupRecord.sequenceIndex
				parser.CmdStash16, // seqLookupRecord.lookupListIndex
				parser.CmdEndLoop,
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

func (l *chainedSeq1) Apply(filter KeepGlyphFn, seq []font.Glyph, pos int) ([]font.Glyph, int) {
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

func (g *GTab) readChained2(s *parser.State, subtablePos int64) (*chainedSeq2, error) {
	err := g.Exec(s,
		parser.CmdStash16, // coverageOffset
		parser.CmdStash16, // backtrackClassDefOffset
		parser.CmdStash16, // inputClassDefOffset
		parser.CmdStash16, // lookaheadClassDefOffset

		parser.CmdRead16, parser.TypeUInt, // chainedClassSeqRuleSetCount
		parser.CmdLoop,
		parser.CmdStash16, // chainedClassSeqRuleSetOffset[i]
		parser.CmdEndLoop,
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
			parser.CmdSeek,
			parser.CmdRead16, parser.TypeUInt, // chainedClassSeqRuleCount
			parser.CmdLoop,
			parser.CmdStash16, // chainedClassSeqRuleOffsets[i]
			parser.CmdEndLoop,
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
				parser.CmdSeek,
				parser.CmdRead16, parser.TypeUInt, // backtrackGlyphCount
				parser.CmdLoop,
				parser.CmdStash16, // backtrackSequence[i]
				parser.CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			backtrackSequence := s.GetStash()
			for _, class := range backtrackSequence {
				rule.backtrack = append(rule.backtrack, int(class))
			}

			err = g.Exec(s,
				parser.CmdRead16, parser.TypeUInt, // inputGlyphCount
				parser.CmdAssertGt, 0,
				parser.CmdDec,
				parser.CmdLoop,
				parser.CmdStash16, // inputSequence[i]
				parser.CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			inputSequence := s.GetStash()
			for _, class := range inputSequence {
				rule.input = append(rule.input, int(class))
			}

			err = g.Exec(s,
				parser.CmdRead16, parser.TypeUInt, // lookaheadGlyphCount
				parser.CmdLoop,
				parser.CmdStash16, // lookaheadSequence[i]
				parser.CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			lookaheadSequence := s.GetStash()
			for _, class := range lookaheadSequence {
				rule.lookahead = append(rule.lookahead, int(class))
			}

			err = g.Exec(s,
				parser.CmdRead16, parser.TypeUInt, // seqLookupCount
				parser.CmdLoop,
				parser.CmdStash16, // seqLookupRecord.sequenceIndex
				parser.CmdStash16, // seqLookupRecord.lookupListIndex
				parser.CmdEndLoop,
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

func (l *chainedSeq2) Apply(filter KeepGlyphFn, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	gid := seq[pos].Gid
	if _, ok := l.cov[gid]; !ok {
		return seq, -1
	}

	class := l.input[gid]
	if class < 0 || class >= len(l.rulesets) {
		return seq, -1
	}
	rules := l.rulesets[class]

	pp := []int{pos}
ruleLoop:
	for _, rule := range rules {
		p := pos
		for _, class := range rule.backtrack {
			p = filter.Prev(seq, p)
			if p < 0 {
				return seq, -1
			}
			if l.backtrack[seq[p].Gid] != class {
				continue ruleLoop
			}
		}
		p = pos
		pp = pp[:1]
		for _, class := range rule.input {
			p = filter.Next(seq, p)
			if p < 0 {
				return seq, -1
			}
			pp = append(pp, p)
			if l.input[seq[p].Gid] != class {
				continue ruleLoop
			}
		}
		next := p + 1
		for _, class := range rule.lookahead {
			p = filter.Next(seq, p)
			if p < 0 {
				return seq, -1
			}
			if l.lookahead[seq[p].Gid] != class {
				continue ruleLoop
			}
		}

		seq = applyActions(rule.actions, pp, seq)
		return seq, next
	}
	return seq, -1
}

// Chained Contexts Substitution Format 3: Coverage-based Glyph Contexts
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#chseqctxt3
type chainedSeq3 struct {
	backtrack []coverage
	input     []coverage
	lookahead []coverage
	actions   []seqLookup
}

func (g *GTab) readChained3(s *parser.State, subtablePos int64) (*chainedSeq3, error) {
	err := g.Exec(s,
		parser.CmdRead16, parser.TypeUInt, // backtrackGlyphCount
		parser.CmdLoop,
		parser.CmdStash16, // backtrackCoverageOffset
		parser.CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	backtrackCoverageOffsets := s.GetStash()

	err = g.Exec(s,
		parser.CmdRead16, parser.TypeUInt, // inputGlyphCount
		parser.CmdLoop,
		parser.CmdStash16, // inputCoverageOffset
		parser.CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	inputCoverageOffsets := s.GetStash()

	err = g.Exec(s,
		parser.CmdRead16, parser.TypeUInt, // lookaheadGlyphCount
		parser.CmdLoop,
		parser.CmdStash16, // lookaheadCoverageOffset
		parser.CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	lookaheadCoverageOffsets := s.GetStash()

	err = g.Exec(s,
		parser.CmdRead16, parser.TypeUInt, // seqLookupCount
		parser.CmdLoop,
		parser.CmdStash16, // seqLookupRecord.sequenceIndex
		parser.CmdStash16, // seqLookupRecord.lookupListIndex
		parser.CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	seqLookupRecord := s.GetStash()

	res := &chainedSeq3{}

	for _, offs := range backtrackCoverageOffsets {
		cover, err := g.readCoverageTable(subtablePos + int64(offs))
		if err != nil {
			return nil, err
		}
		res.backtrack = append(res.backtrack, cover)
	}
	for _, offs := range inputCoverageOffsets {
		cover, err := g.readCoverageTable(subtablePos + int64(offs))
		if err != nil {
			return nil, err
		}
		res.input = append(res.input, cover)
	}
	for _, offs := range lookaheadCoverageOffsets {
		cover, err := g.readCoverageTable(subtablePos + int64(offs))
		if err != nil {
			return nil, err
		}
		res.lookahead = append(res.lookahead, cover)
	}
	for len(seqLookupRecord) > 0 {
		l, err := g.getGtabLookup(seqLookupRecord[1])
		if err != nil {
			return nil, err
		}
		res.actions = append(res.actions, seqLookup{
			pos:    int(seqLookupRecord[0]),
			nested: l,
		})
		seqLookupRecord = seqLookupRecord[2:]
	}
	return res, nil
}

func (l *chainedSeq3) Apply(keep KeepGlyphFn, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	// TODO(voss): remove this fast path?
	if pos < len(l.backtrack) || pos+len(l.input)+len(l.lookahead) > len(seq) {
		return seq, -1
	}

	p := pos
	for _, cov := range l.backtrack {
		p = keep.Prev(seq, p)
		if p < 0 {
			return seq, p
		}
		_, ok := cov[seq[p].Gid]
		if !ok {
			return seq, -1
		}
	}
	p = pos
	pp := make([]int, len(l.input))
	for i, cov := range l.input {
		if i > 0 {
			p = keep.Next(seq, p)
			if p < 0 {
				return seq, -1
			}
		}
		pp[i] = p
		_, ok := cov[seq[p].Gid]
		if !ok {
			return seq, -1
		}
	}
	next := p + 1 // TODO(voss): special consideration in the case that a nested lookup is a GPOS type 2, paired positioning, lookup
	for i, cov := range l.lookahead {
		p = keep.Next(seq, p)
		if p < 0 {
			return seq, -1
		}
		_, ok := cov[seq[next+i].Gid]
		if !ok {
			return seq, -1
		}
	}

	seq = applyActions(l.actions, pp, seq)
	return seq, next
}

type seqLookup struct {
	pos    int
	nested *LookupTable
}

func applyActions(actions []seqLookup, pp []int, seq []font.Glyph) []font.Glyph {
	origLen := len(seq)
	for _, action := range actions {
		if action.pos < 0 || action.pos >= len(pp) {
			continue
		}
		seq, _ = action.nested.applySubtables(seq, pp[action.pos])
		if len(seq) != origLen {
			// TODO(voss): how to interpret action.pos in case a prior action
			// changes the length of seq?
			panic("not implemented: nested lookup changed sequence length")
		}
	}
	return seq
}
