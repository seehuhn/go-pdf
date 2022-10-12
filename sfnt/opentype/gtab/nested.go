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
	"seehuhn.de/go/pdf/sfnt/glyph"
	"seehuhn.de/go/pdf/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/sfnt/parser"
)

// SeqLookup describes the actions for contextual and chained contextual
// lookups.
type SeqLookup struct {
	SequenceIndex   uint16
	LookupListIndex LookupIndex
}

// SeqLookups describes the actions of nested lookups.
type SeqLookups []SeqLookup

func readNested(p *parser.Parser, seqLookupCount int) (SeqLookups, error) {
	res := make(SeqLookups, seqLookupCount)
	for i := range res {
		buf, err := p.ReadBytes(4)
		if err != nil {
			return nil, err
		}
		res[i].SequenceIndex = uint16(buf[0])<<8 | uint16(buf[1])
		res[i].LookupListIndex = LookupIndex(buf[2])<<8 | LookupIndex(buf[3])
	}
	return res, nil
}

// SeqContext1 is used for GSUB type 5 format 1 subtables and GPOS type 7 format 1 subtables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#sequence-context-format-1-simple-glyph-contexts
type SeqContext1 struct {
	Cov   coverage.Table
	Rules [][]*SeqRule // indexed by coverage index
}

// SeqRule describes a rule in a SeqContext1 subtable.
type SeqRule struct {
	Input   []glyph.ID // excludes the first input glyph, since this is in Cov
	Actions SeqLookups
}

func readSeqContext1(p *parser.Parser, subtablePos int64) (Subtable, error) {
	coverageOffset, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	seqRuleSetOffsets, err := p.ReadUint16Slice()
	if err != nil {
		return nil, err
	}

	cov, err := coverage.Read(p, subtablePos+int64(coverageOffset))
	if err != nil {
		return nil, err
	}
	if len(cov) > len(seqRuleSetOffsets) {
		cov.Prune(len(seqRuleSetOffsets))
	} else {
		seqRuleSetOffsets = seqRuleSetOffsets[:len(cov)]
	}

	res := &SeqContext1{
		Cov:   cov,
		Rules: make([][]*SeqRule, len(seqRuleSetOffsets)),
	}

	for i, seqRuleSetOffset := range seqRuleSetOffsets {
		if seqRuleSetOffset == 0 {
			continue
		}

		base := subtablePos + int64(seqRuleSetOffset)
		err = p.SeekPos(base)
		if err != nil {
			return nil, err
		}

		seqRuleOffsets, err := p.ReadUint16Slice()
		if err != nil {
			return nil, err
		}
		res.Rules[i] = make([]*SeqRule, len(seqRuleOffsets))
		for j, seqRuleOffset := range seqRuleOffsets {
			err = p.SeekPos(base + int64(seqRuleOffset))
			if err != nil {
				return nil, err
			}

			buf, err := p.ReadBytes(4)
			if err != nil {
				return nil, err
			}
			glyphCount := int(buf[0])<<8 | int(buf[1])
			if glyphCount == 0 {
				return nil, &parser.InvalidFontError{
					SubSystem: "sfnt/opentype/gtab",
					Reason:    "invalid glyph count in SeqContext1",
				}
			}
			seqLookupCount := int(buf[2])<<8 | int(buf[3])
			inputSequence := make([]glyph.ID, glyphCount-1)
			for k := range inputSequence {
				xk, err := p.ReadUint16()
				if err != nil {
					return nil, err
				}
				inputSequence[k] = glyph.ID(xk)
			}
			actions, err := readNested(p, seqLookupCount)
			if err != nil {
				return nil, err
			}
			res.Rules[i][j] = &SeqRule{
				Input:   inputSequence,
				Actions: actions,
			}
		}
	}

	return res, nil
}

// Apply implements the Subtable interface.
func (l *SeqContext1) Apply(keep keepGlyphFn, seq []glyph.Info, a, b int) *Match {
	gid := seq[a].Gid
	rulesIdx, ok := l.Cov[gid]
	if !ok {
		return nil
	}
	rules := l.Rules[rulesIdx]

	var matchPos []int
ruleLoop:
	for _, rule := range rules {
		p := a
		matchPos = append(matchPos[:0], p)
		glyphsNeeded := len(rule.Input)
		for _, gid := range rule.Input {
			glyphsNeeded--
			p++
			for p+glyphsNeeded < b && !keep(seq[p].Gid) {
				p++
			}
			if p+glyphsNeeded >= b || seq[p].Gid != gid {
				continue ruleLoop
			}
			matchPos = append(matchPos, p)
		}

		p++
		for p < b && !keep(seq[p].Gid) {
			p++
		}

		return &Match{
			InputPos: matchPos,
			Actions:  rule.Actions,
			Next:     p,
		}
	}

	return nil
}

// EncodeLen implements the Subtable interface.
func (l *SeqContext1) EncodeLen() int {
	total := 6 + 2*len(l.Rules)
	for _, rules := range l.Rules {
		if rules == nil {
			continue
		}
		total += 2 + 2*len(rules)
		for _, rule := range rules {
			total += 4 + 2*len(rule.Input) + 4*len(rule.Actions)
		}
	}
	total += l.Cov.EncodeLen()
	return total
}

// Encode implements the Subtable interface.
func (l *SeqContext1) Encode() []byte {
	seqRuleSetCount := len(l.Rules)

	total := 6 + 2*seqRuleSetCount
	seqRuleSetOffsets := make([]uint16, seqRuleSetCount)
	for i, rules := range l.Rules {
		if rules == nil {
			continue
		}
		seqRuleSetOffsets[i] = uint16(total)
		total += 2 + 2*len(rules)
		for _, rule := range rules {
			total += 4 + 2*len(rule.Input) + 4*len(rule.Actions)
		}
	}
	coverageOffset := total
	total += l.Cov.EncodeLen()

	buf := make([]byte, 0, total)
	buf = append(buf,
		0, 1, // format
		byte(coverageOffset>>8), byte(coverageOffset),
		byte(seqRuleSetCount>>8), byte(seqRuleSetCount),
	)
	for _, offset := range seqRuleSetOffsets {
		buf = append(buf, byte(offset>>8), byte(offset))
	}
	for _, rules := range l.Rules {
		if rules == nil {
			continue
		}
		seqRuleCount := len(rules)
		buf = append(buf,
			byte(seqRuleCount>>8), byte(seqRuleCount),
		)
		pos := 2 + 2*seqRuleCount
		for _, rule := range rules {
			buf = append(buf,
				byte(pos>>8), byte(pos),
			)
			pos += 4 + 2*len(rule.Input) + 4*len(rule.Actions)
		}
		for _, rule := range rules {
			glyphCount := len(rule.Input) + 1
			seqLookupCount := len(rule.Actions)
			buf = append(buf,
				byte(glyphCount>>8), byte(glyphCount),
				byte(seqLookupCount>>8), byte(seqLookupCount),
			)
			for _, gid := range rule.Input {
				buf = append(buf, byte(gid>>8), byte(gid))
			}
			for _, action := range rule.Actions {
				buf = append(buf,
					byte(action.SequenceIndex>>8), byte(action.SequenceIndex),
					byte(action.LookupListIndex>>8), byte(action.LookupListIndex),
				)
			}
		}
	}
	buf = append(buf, l.Cov.Encode()...)
	return buf
}

// SeqContext2 is used for GSUB type 5 format 2 subtables and GPOS type 7 format 2 subtables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#sequence-context-format-2-class-based-glyph-contexts
type SeqContext2 struct {
	Cov   coverage.Table
	Input classdef.Table
	Rules [][]*ClassSeqRule // indexed by class index of the first glyph
}

// ClassSeqRule describes a sequence of glyph classes and the actions to
// be performed
type ClassSeqRule struct {
	Input   []uint16 // excludes the first input glyph
	Actions SeqLookups
}

func readSeqContext2(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(4)
	if err != nil {
		return nil, err
	}
	coverageOffset := uint16(buf[0])<<8 | uint16(buf[1])
	classDefOffset := uint16(buf[2])<<8 | uint16(buf[3])
	classSeqRuleSetOffsets, err := p.ReadUint16Slice()
	if err != nil {
		return nil, err
	}

	cov, err := coverage.Read(p, subtablePos+int64(coverageOffset))
	if err != nil {
		return nil, err
	}

	classDef, err := classdef.Read(p, subtablePos+int64(classDefOffset))
	if err != nil {
		return nil, err
	}

	numClasses := classDef.NumClasses()
	if len(classSeqRuleSetOffsets) > numClasses {
		classSeqRuleSetOffsets = classSeqRuleSetOffsets[:numClasses]
	}
	seqRuleSetCount := len(classSeqRuleSetOffsets)

	res := &SeqContext2{
		Cov:   cov,
		Input: classDef,
		Rules: make([][]*ClassSeqRule, seqRuleSetCount),
	}

	total := 8 + 2*seqRuleSetCount
	for i, classSeqRuleSetOffset := range classSeqRuleSetOffsets {
		if classSeqRuleSetOffset == 0 {
			continue
		}
		base := subtablePos + int64(classSeqRuleSetOffset)
		err = p.SeekPos(base)
		if err != nil {
			return nil, err
		}
		seqRuleOffsets, err := p.ReadUint16Slice()
		if err != nil {
			return nil, err
		}
		res.Rules[i] = make([]*ClassSeqRule, len(seqRuleOffsets))
		total += 2 + 2*len(res.Rules[i])
		for j, seqRuleOffset := range seqRuleOffsets {
			err = p.SeekPos(base + int64(seqRuleOffset))
			if err != nil {
				return nil, err
			}
			buf, err := p.ReadBytes(4)
			if err != nil {
				return nil, err
			}
			glyphCount := int(buf[0])<<8 | int(buf[1])
			if glyphCount == 0 {
				return nil, &parser.InvalidFontError{
					SubSystem: "sfnt/opentype/gtab",
					Reason:    "invalid glyph count in SeqContext2",
				}
			}
			seqLookupCount := int(buf[2])<<8 | int(buf[3])
			inputSequence := make([]uint16, glyphCount-1)
			for k := range inputSequence {
				xk, err := p.ReadUint16()
				if err != nil {
					return nil, err
				}
				inputSequence[k] = xk
			}
			actions, err := readNested(p, seqLookupCount)
			if err != nil {
				return nil, err
			}
			res.Rules[i][j] = &ClassSeqRule{
				Input:   inputSequence,
				Actions: actions,
			}
			total += 4 + 2*len(inputSequence) + 4*len(actions)
		}
	}
	total += cov.EncodeLen()

	if total > 0xFFFF {
		return nil, &parser.InvalidFontError{
			SubSystem: "sfnt/opentype/gtab",
			Reason:    "SeqContext2 too large",
		}
	}

	return res, nil
}

// Apply implements the Subtable interface.
func (l *SeqContext2) Apply(keep keepGlyphFn, seq []glyph.Info, a, b int) *Match {
	gid := seq[a].Gid
	_, ok := l.Cov[gid]
	if !ok {
		return nil
	}
	ruleIdx := l.Input[gid]
	rules := l.Rules[ruleIdx]

	var matchPos []int
ruleLoop:
	for _, rule := range rules {
		p := a
		matchPos = append(matchPos[:0], p)
		glyphsNeeded := len(rule.Input)
		for _, cls := range rule.Input {
			glyphsNeeded--
			p++
			for p+glyphsNeeded < b && !keep(seq[p].Gid) {
				p++
			}
			if p+glyphsNeeded >= b || l.Input[seq[p].Gid] != cls {
				continue ruleLoop
			}
			matchPos = append(matchPos, p)
		}

		p++
		for p < b && !keep(seq[p].Gid) {
			p++
		}

		return &Match{
			InputPos: matchPos,
			Actions:  rule.Actions,
			Next:     p,
		}
	}

	return nil
}

// EncodeLen implements the Subtable interface.
func (l *SeqContext2) EncodeLen() int {
	total := 8 + 2*len(l.Rules)
	total += l.Cov.EncodeLen()
	total += l.Input.AppendLen()
	for _, rules := range l.Rules {
		if rules == nil {
			continue
		}
		total += 2 + 2*len(rules)
		for _, rule := range rules {
			total += 4 + 2*len(rule.Input) + 4*len(rule.Actions)
		}
	}
	return total
}

// Encode implements the Subtable interface.
func (l *SeqContext2) Encode() []byte {
	seqRuleSetCount := len(l.Rules)
	total := 8 + 2*seqRuleSetCount
	seqRuleSetOffsets := make([]uint16, seqRuleSetCount)
	for i, rules := range l.Rules {
		if rules == nil {
			continue
		}
		seqRuleSetOffsets[i] = uint16(total)
		total += 2 + 2*len(rules)
		for _, rule := range rules {
			total += 4 + 2*len(rule.Input) + 4*len(rule.Actions)
		}
	}
	coverageOffset := total
	total += l.Cov.EncodeLen()
	classDefOffset := total
	total += l.Input.AppendLen()

	if classDefOffset > 0xFFFF {
		panic("classDefOffset too large")
	}

	buf := make([]byte, 0, total)
	buf = append(buf,
		0, 2, // format
		byte(coverageOffset>>8), byte(coverageOffset),
		byte(classDefOffset>>8), byte(classDefOffset),
		byte(seqRuleSetCount>>8), byte(seqRuleSetCount),
	)
	for _, offset := range seqRuleSetOffsets {
		buf = append(buf, byte(offset>>8), byte(offset))
	}
	for _, rules := range l.Rules {
		if rules == nil {
			continue
		}
		seqRuleCount := len(rules)
		buf = append(buf,
			byte(seqRuleCount>>8), byte(seqRuleCount),
		)
		pos := 2 + 2*seqRuleCount
		for _, rule := range rules {
			buf = append(buf,
				byte(pos>>8), byte(pos),
			)
			pos += 4 + 2*len(rule.Input) + 4*len(rule.Actions)
		}
		for _, rule := range rules {
			glyphCount := len(rule.Input) + 1
			seqLookupCount := len(rule.Actions)
			buf = append(buf,
				byte(glyphCount>>8), byte(glyphCount),
				byte(seqLookupCount>>8), byte(seqLookupCount),
			)
			for _, gid := range rule.Input {
				buf = append(buf, byte(gid>>8), byte(gid))
			}
			for _, action := range rule.Actions {
				buf = append(buf,
					byte(action.SequenceIndex>>8), byte(action.SequenceIndex),
					byte(action.LookupListIndex>>8), byte(action.LookupListIndex),
				)
			}
		}
	}
	buf = append(buf, l.Cov.Encode()...)
	buf = l.Input.Append(buf)
	return buf
}

// SeqContext3 is used for GSUB type 5 format 3 and GPOS type 7 format 3 subtables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#sequence-context-format-3-coverage-based-glyph-contexts
type SeqContext3 struct {
	Input   []coverage.Table
	Actions SeqLookups
}

func readSeqContext3(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(4)
	if err != nil {
		return nil, err
	}
	glyphCount := int(buf[0])<<8 | int(buf[1])
	if glyphCount < 1 {
		return nil, &parser.InvalidFontError{
			SubSystem: "sfnt/opentype/gtab",
			Reason:    "invalid glyph count in SeqContext3",
		}
	}
	seqLookupCount := int(buf[2])<<8 | int(buf[3])
	coverageOffsets := make([]uint16, glyphCount)
	for i := range coverageOffsets {
		coverageOffsets[i], err = p.ReadUint16()
		if err != nil {
			return nil, err
		}
	}

	actions, err := readNested(p, seqLookupCount)
	if err != nil {
		return nil, err
	}

	cov := make([]coverage.Table, glyphCount)
	for i, offset := range coverageOffsets {
		cov[i], err = coverage.Read(p, subtablePos+int64(offset))
		if err != nil {
			return nil, err
		}
	}

	res := &SeqContext3{
		Input:   cov,
		Actions: actions,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *SeqContext3) Apply(keep keepGlyphFn, seq []glyph.Info, a, b int) *Match {
	gid := seq[a].Gid
	if !l.Input[0].Contains(gid) {
		return nil
	}

	p := a
	matchPos := []int{p}
	glyphsNeeded := len(l.Input) - 1
	for _, cov := range l.Input[1:] {
		glyphsNeeded--
		p++
		for p+glyphsNeeded < b && !keep(seq[p].Gid) {
			p++
		}
		if p+glyphsNeeded >= b || !cov.Contains(seq[p].Gid) {
			return nil
		}
		matchPos = append(matchPos, p)
	}

	p++
	for p < b && !keep(seq[p].Gid) {
		p++
	}

	return &Match{
		InputPos: matchPos,
		Actions:  l.Actions,
		Next:     p,
	}
}

// EncodeLen implements the Subtable interface.
func (l *SeqContext3) EncodeLen() int {
	total := 6 + 2*len(l.Input) + 4*len(l.Actions)
	for _, cov := range l.Input {
		total += cov.EncodeLen()
	}
	return total
}

// Encode implements the Subtable interface.
func (l *SeqContext3) Encode() []byte {
	glyphCount := len(l.Input)
	seqLookupCount := len(l.Actions)

	total := 6 + 2*len(l.Input) + 4*len(l.Actions)
	coverageOffsets := make([]uint16, glyphCount)
	for i, cov := range l.Input {
		coverageOffsets[i] = uint16(total)
		total += cov.EncodeLen()
	}

	buf := make([]byte, 0, total)
	buf = append(buf,
		0, 3, // format
		byte(glyphCount>>8), byte(glyphCount),
		byte(seqLookupCount>>8), byte(seqLookupCount),
	)
	for _, offset := range coverageOffsets {
		buf = append(buf, byte(offset>>8), byte(offset))
	}
	for _, action := range l.Actions {
		buf = append(buf,
			byte(action.SequenceIndex>>8), byte(action.SequenceIndex),
			byte(action.LookupListIndex>>8), byte(action.LookupListIndex),
		)
	}
	for _, cov := range l.Input {
		buf = append(buf, cov.Encode()...)
	}
	return buf
}

// ChainedSeqContext1 is used for GSUB type 6 format 1 and GPOS type 8 format 1 subtables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#chained-sequence-context-format-1-simple-glyph-contexts
type ChainedSeqContext1 struct {
	Cov   coverage.Table
	Rules [][]*ChainedSeqRule // indexed by coverage index
}

// ChainedSeqRule describes the rules in a ChainedSeqContext1.
type ChainedSeqRule struct {
	Backtrack []glyph.ID
	Input     []glyph.ID // excludes the first input glyph, since this is in Cov
	Lookahead []glyph.ID
	Actions   SeqLookups
}

func readChainedSeqContext1(p *parser.Parser, subtablePos int64) (Subtable, error) {
	coverageOffset, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	chainedSeqRuleSetOffsets, err := p.ReadUint16Slice()
	if err != nil {
		return nil, err
	}

	cov, err := coverage.Read(p, subtablePos+int64(coverageOffset))
	if err != nil {
		return nil, err
	}

	if len(cov) > len(chainedSeqRuleSetOffsets) {
		cov.Prune(len(chainedSeqRuleSetOffsets))
	} else {
		chainedSeqRuleSetOffsets = chainedSeqRuleSetOffsets[:len(cov)]
	}

	total := 6 + 2*len(chainedSeqRuleSetOffsets)
	total += cov.EncodeLen()

	rules := make([][]*ChainedSeqRule, len(chainedSeqRuleSetOffsets))
	for i, chainedSeqRuleSetOffset := range chainedSeqRuleSetOffsets {
		if chainedSeqRuleSetOffset == 0 {
			continue
		}

		base := subtablePos + int64(chainedSeqRuleSetOffset)
		err = p.SeekPos(base)
		if err != nil {
			return nil, err
		}

		chainedSeqRuleOffsets, err := p.ReadUint16Slice()
		if err != nil {
			return nil, err
		}

		if total > 0xFFFF {
			return nil, &parser.InvalidFontError{
				SubSystem: "sfnt/opentype/gtab",
				Reason:    "ChainedSeqContext1 too large",
			}
		}
		ruleSetSize := 2 + 2*len(chainedSeqRuleOffsets)

		rules[i] = make([]*ChainedSeqRule, len(chainedSeqRuleOffsets))
		for j, chainedSeqRuleOffset := range chainedSeqRuleOffsets {
			err = p.SeekPos(base + int64(chainedSeqRuleOffset))
			if err != nil {
				return nil, err
			}

			backtrackSequence, err := readGIDSlice(p)
			if err != nil {
				return nil, err
			}
			inputGlyphCount, err := p.ReadUint16()
			if err != nil {
				return nil, err
			}
			inputSequence := make([]glyph.ID, inputGlyphCount-1)
			for k := range inputSequence {
				val, err := p.ReadUint16()
				if err != nil {
					return nil, err
				}
				inputSequence[k] = glyph.ID(val)
			}
			lookaheadSequence, err := readGIDSlice(p)
			if err != nil {
				return nil, err
			}
			seqLookupCount, err := p.ReadUint16()
			if err != nil {
				return nil, err
			}
			actions, err := readNested(p, int(seqLookupCount))
			if err != nil {
				return nil, err
			}

			if ruleSetSize > 0xFFFF {
				return nil, &parser.InvalidFontError{
					SubSystem: "sfnt/opentype/gtab",
					Reason:    "ChainedSeqContext1 ruleset too large",
				}
			}
			ruleSetSize += 2 + 2*len(backtrackSequence)
			ruleSetSize += 2 + 2*len(inputSequence)
			ruleSetSize += 2 + 2*len(lookaheadSequence)
			ruleSetSize += 2 + 4*len(actions)

			rules[i][j] = &ChainedSeqRule{
				Backtrack: backtrackSequence,
				Input:     inputSequence,
				Lookahead: lookaheadSequence,
				Actions:   actions,
			}
		}
		total += ruleSetSize
	}

	res := &ChainedSeqContext1{
		Cov:   cov,
		Rules: rules,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *ChainedSeqContext1) Apply(keep keepGlyphFn, seq []glyph.Info, a, b int) *Match {
	gid := seq[a].Gid
	rulesIdx, ok := l.Cov[gid]
	if !ok {
		return nil
	}
	rules := l.Rules[rulesIdx]

	var matchPos []int
ruleLoop:
	for _, rule := range rules {
		p := a
		glyphsNeeded := len(rule.Backtrack)
		for _, gid := range rule.Backtrack {
			glyphsNeeded--
			p--
			for p-glyphsNeeded >= 0 && !keep(seq[p].Gid) {
				p--
			}
			if p-glyphsNeeded < 0 || seq[p].Gid != gid {
				continue ruleLoop
			}
		}

		p = a
		matchPos = append(matchPos[:0], p)
		glyphsNeeded = len(rule.Input)
		for _, gid := range rule.Input {
			glyphsNeeded--
			p++
			for p+glyphsNeeded < b && !keep(seq[p].Gid) {
				p++
			}
			if p+glyphsNeeded >= b || seq[p].Gid != gid {
				continue ruleLoop
			}
			matchPos = append(matchPos, p)
		}
		next := p

		glyphsNeeded = len(rule.Lookahead)
		for _, gid := range rule.Lookahead {
			glyphsNeeded--
			p++
			for p+glyphsNeeded < len(seq) && !keep(seq[p].Gid) {
				p++
			}
			if p+glyphsNeeded >= len(seq) || seq[p].Gid != gid {
				continue ruleLoop
			}
		}

		next++
		for next < b && !keep(seq[next].Gid) {
			next++
		}

		return &Match{
			InputPos: matchPos,
			Actions:  rule.Actions,
			Next:     next,
		}
	}

	return nil
}

// EncodeLen implements the Subtable interface.
func (l *ChainedSeqContext1) EncodeLen() int {
	total := 6 + 2*len(l.Rules)
	total += l.Cov.EncodeLen()
	for _, rules := range l.Rules {
		if rules == nil {
			continue
		}
		total += 2 + 2*len(rules)
		for _, rule := range rules {
			total += 2 + 2*len(rule.Backtrack)
			total += 2 + 2*len(rule.Input)
			total += 2 + 2*len(rule.Lookahead)
			total += 2 + 4*len(rule.Actions)
		}
	}
	return total
}

// Encode implements the Subtable interface.
func (l *ChainedSeqContext1) Encode() []byte {
	chainedSeqRuleSetCount := len(l.Rules)
	total := 6 + 2*len(l.Rules)
	coverageOffset := total
	total += l.Cov.EncodeLen()
	chainedSeqRuleSetOffsets := make([]uint16, chainedSeqRuleSetCount)
	for i, rules := range l.Rules {
		if rules == nil {
			continue
		}
		chainedSeqRuleSetOffsets[i] = uint16(total)
		total += 2 + 2*len(rules)
		for _, rule := range rules {
			total += 2 + 2*len(rule.Backtrack)
			total += 2 + 2*len(rule.Input)
			total += 2 + 2*len(rule.Lookahead)
			total += 2 + 4*len(rule.Actions)
		}
	}

	buf := make([]byte, 0, total)
	buf = append(buf,
		0, 1, // format
		byte(coverageOffset>>8), byte(coverageOffset),
		byte(chainedSeqRuleSetCount>>8), byte(chainedSeqRuleSetCount),
	)
	for _, offset := range chainedSeqRuleSetOffsets {
		buf = append(buf, byte(offset>>8), byte(offset))
	}

	buf = append(buf, l.Cov.Encode()...)

	for _, rules := range l.Rules {
		if rules == nil {
			continue
		}
		chainedSeqRuleCount := len(rules)
		buf = append(buf,
			byte(chainedSeqRuleCount>>8), byte(chainedSeqRuleCount),
		)

		pos := 2 + 2*chainedSeqRuleCount
		for _, rule := range rules {
			buf = append(buf,
				byte(pos>>8), byte(pos),
			)
			pos += 2 + 2*len(rule.Backtrack)
			pos += 2 + 2*len(rule.Input)
			pos += 2 + 2*len(rule.Lookahead)
			pos += 2 + 4*len(rule.Actions)
		}
		for _, rule := range rules {
			backtrackGlyphCount := len(rule.Backtrack)
			buf = append(buf,
				byte(backtrackGlyphCount>>8), byte(backtrackGlyphCount),
			)
			for _, gid := range rule.Backtrack {
				buf = append(buf, byte(gid>>8), byte(gid))
			}
			inputGlyphCount := len(rule.Input) + 1
			buf = append(buf,
				byte(inputGlyphCount>>8), byte(inputGlyphCount),
			)
			for _, gid := range rule.Input {
				buf = append(buf, byte(gid>>8), byte(gid))
			}
			lookaheadGlyphCount := len(rule.Lookahead)
			buf = append(buf,
				byte(lookaheadGlyphCount>>8), byte(lookaheadGlyphCount),
			)
			for _, gid := range rule.Lookahead {
				buf = append(buf, byte(gid>>8), byte(gid))
			}
			seqLookupCount := len(rule.Actions)
			buf = append(buf,
				byte(seqLookupCount>>8), byte(seqLookupCount),
			)
			for _, a := range rule.Actions {
				buf = append(buf,
					byte(a.SequenceIndex>>8), byte(a.SequenceIndex),
					byte(a.LookupListIndex>>8), byte(a.LookupListIndex),
				)
			}
		}
	}
	return buf
}

// ChainedSeqContext2 is used for GSUB type 6 format 2 and GPOS type 8 format 2 subtables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#chained-sequence-context-format-2-class-based-glyph-contexts
type ChainedSeqContext2 struct {
	Cov       coverage.Table
	Backtrack classdef.Table
	Input     classdef.Table
	Lookahead classdef.Table
	Rules     [][]*ChainedClassSeqRule // indexed by input glyph class
}

// ChainedClassSeqRule is used to represent the rules in a ChainedSeqContext2.
// It describes a sequence of nested lookups together with the context where
// they apply.  The Backtrack, Input and Lookahead sequences are given
// as lists of glyph classes, as defined by the corresponding class definition
// tables in the ChainedSeqContext2 structure.
type ChainedClassSeqRule struct {
	Backtrack []uint16
	Input     []uint16 // excludes the first input glyph, since this is in Cov
	Lookahead []uint16
	Actions   SeqLookups
}

func readChainedSeqContext2(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(8)
	if err != nil {
		return nil, err
	}
	covOffset := uint16(buf[0])<<8 | uint16(buf[1])
	backtrackClassDefOffset := uint16(buf[2])<<8 | uint16(buf[3])
	inputClassDefOffset := uint16(buf[4])<<8 | uint16(buf[5])
	lookaheadClassDefOffset := uint16(buf[6])<<8 | uint16(buf[7])

	chainedClassSeqRuleSetOffsets, err := p.ReadUint16Slice()
	if err != nil {
		return nil, err
	}

	cov, err := coverage.Read(p, subtablePos+int64(covOffset))
	if err != nil {
		return nil, err
	}
	backtrackClassDef, err := classdef.Read(p, subtablePos+int64(backtrackClassDefOffset))
	if err != nil {
		return nil, err
	}
	inputClassDef, err := classdef.Read(p, subtablePos+int64(inputClassDefOffset))
	if err != nil {
		return nil, err
	}
	lookaheadClassDef, err := classdef.Read(p, subtablePos+int64(lookaheadClassDefOffset))
	if err != nil {
		return nil, err
	}

	numClasses := inputClassDef.NumClasses()
	if numClasses < len(chainedClassSeqRuleSetOffsets) {
		chainedClassSeqRuleSetOffsets = chainedClassSeqRuleSetOffsets[:numClasses]
	}

	rules := make([][]*ChainedClassSeqRule, len(chainedClassSeqRuleSetOffsets))
	for i, chainedClassSeqRuleSetOffset := range chainedClassSeqRuleSetOffsets {
		if chainedClassSeqRuleSetOffset == 0 {
			continue
		}

		base := subtablePos + int64(chainedClassSeqRuleSetOffset)
		err = p.SeekPos(base)
		if err != nil {
			return nil, err
		}

		chainedClassSeqRuleOffsets, err := p.ReadUint16Slice()
		if err != nil {
			return nil, err
		}

		rules[i] = make([]*ChainedClassSeqRule, len(chainedClassSeqRuleOffsets))
		for j, chainedClassSeqRuleOffset := range chainedClassSeqRuleOffsets {
			err = p.SeekPos(base + int64(chainedClassSeqRuleOffset))
			if err != nil {
				return nil, err
			}

			backtrackSequence, err := p.ReadUint16Slice()
			if err != nil {
				return nil, err
			}
			inputGlyphCount, err := p.ReadUint16()
			if err != nil {
				return nil, err
			}
			inputSequence := make([]uint16, inputGlyphCount-1)
			for k := range inputSequence {
				inputSequence[k], err = p.ReadUint16()
				if err != nil {
					return nil, err
				}
			}
			lookaheadSequence, err := p.ReadUint16Slice()
			if err != nil {
				return nil, err
			}
			seqLookupCount, err := p.ReadUint16()
			if err != nil {
				return nil, err
			}
			actions, err := readNested(p, int(seqLookupCount))
			if err != nil {
				return nil, err
			}

			rules[i][j] = &ChainedClassSeqRule{
				Backtrack: backtrackSequence,
				Input:     inputSequence,
				Lookahead: lookaheadSequence,
				Actions:   actions,
			}
		}
	}

	total := 12 + 2*len(rules)
	total += cov.EncodeLen()
	total += backtrackClassDef.AppendLen()
	total += inputClassDef.AppendLen()
	total += lookaheadClassDef.AppendLen()
	for _, rr := range rules {
		if rr == nil {
			continue
		}
		if total > 0xFFFF {
			return nil, &parser.InvalidFontError{
				SubSystem: "sfnt/opentype/gtab",
				Reason:    "ChainedSeqContext2 too large",
			}
		}

		pos := 2 + 2*len(rr)
		for _, rule := range rr {
			if pos > 0xFFFF {
				return nil, &parser.InvalidFontError{
					SubSystem: "sfnt/opentype/gtab",
					Reason:    "ChainedSeqContext2 too large",
				}
			}
			pos += 2 + 2*len(rule.Backtrack)
			pos += 2 + 2*len(rule.Input)
			pos += 2 + 2*len(rule.Lookahead)
			pos += 2 + 4*len(rule.Actions)
		}
		total += pos
	}

	res := &ChainedSeqContext2{
		Cov:       cov,
		Backtrack: backtrackClassDef,
		Input:     inputClassDef,
		Lookahead: lookaheadClassDef,
		Rules:     rules,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *ChainedSeqContext2) Apply(keep keepGlyphFn, seq []glyph.Info, a, b int) *Match {
	gid := seq[a].Gid
	_, ok := l.Cov[gid]
	if !ok {
		return nil
	}
	rulesIdx := l.Input[gid]
	if int(rulesIdx) >= len(l.Rules) {
		return nil
	}
	rules := l.Rules[rulesIdx]

	var matchPos []int
ruleLoop:
	for _, rule := range rules {
		p := a
		glyphsNeeded := len(rule.Backtrack)
		for _, cls := range rule.Backtrack {
			glyphsNeeded--
			p--
			for p-glyphsNeeded >= 0 && !keep(seq[p].Gid) {
				p--
			}
			if p-glyphsNeeded < 0 || l.Backtrack[seq[p].Gid] != cls {
				continue ruleLoop
			}
		}

		p = a
		matchPos = append(matchPos[:0], p)
		glyphsNeeded = len(rule.Input)
		for _, cls := range rule.Input {
			glyphsNeeded--
			p++
			for p+glyphsNeeded < b && !keep(seq[p].Gid) {
				p++
			}
			if p+glyphsNeeded >= b || l.Input[seq[p].Gid] != cls {
				continue ruleLoop
			}
			matchPos = append(matchPos, p)
		}
		next := p

		glyphsNeeded = len(rule.Lookahead)
		for _, cls := range rule.Lookahead {
			glyphsNeeded--
			p++
			for p+glyphsNeeded < len(seq) && !keep(seq[p].Gid) {
				p++
			}
			if p+glyphsNeeded >= len(seq) || l.Lookahead[seq[p].Gid] != cls {
				continue ruleLoop
			}
		}

		next++
		for next < b && !keep(seq[next].Gid) {
			next++
		}

		return &Match{
			InputPos: matchPos,
			Actions:  rule.Actions,
			Next:     next,
		}
	}

	return nil
}

// EncodeLen implements the Subtable interface.
func (l *ChainedSeqContext2) EncodeLen() int {
	total := 12 + 2*len(l.Rules)
	total += l.Cov.EncodeLen()
	total += l.Backtrack.AppendLen()
	total += l.Input.AppendLen()
	total += l.Lookahead.AppendLen()
	for _, rules := range l.Rules {
		if rules == nil {
			continue
		}
		total += 2 + 2*len(rules)
		for _, rule := range rules {
			total += 2 + 2*len(rule.Backtrack)
			total += 2 + 2*len(rule.Input)
			total += 2 + 2*len(rule.Lookahead)
			total += 2 + 4*len(rule.Actions)
		}
	}
	return total
}

// Encode implements the Subtable interface.
func (l *ChainedSeqContext2) Encode() []byte {
	chainedSeqRuleSetCount := len(l.Rules)
	total := 12 + 2*len(l.Rules)
	coverageOffset := total
	total += l.Cov.EncodeLen()
	backtrackOffset := total
	total += l.Backtrack.AppendLen()
	inputOffset := total
	total += l.Input.AppendLen()
	lookaheadOffset := total
	total += l.Lookahead.AppendLen()
	chainedSeqRuleSetOffsets := make([]uint16, chainedSeqRuleSetCount)
	for i, rr := range l.Rules {
		if rr == nil {
			continue
		}
		if total > 0xFFFF {
			panic("ChainedSeqContext2 too large")
		}
		chainedSeqRuleSetOffsets[i] = uint16(total)
		total += 2 + 2*len(rr)
		for _, rule := range rr {
			total += 2 + 2*len(rule.Backtrack)
			total += 2 + 2*len(rule.Input)
			total += 2 + 2*len(rule.Lookahead)
			total += 2 + 4*len(rule.Actions)
		}
	}

	buf := make([]byte, 0, total)
	buf = append(buf,
		0, 2, // format
		byte(coverageOffset>>8), byte(coverageOffset),
		byte(backtrackOffset>>8), byte(backtrackOffset),
		byte(inputOffset>>8), byte(inputOffset),
		byte(lookaheadOffset>>8), byte(lookaheadOffset),
		byte(chainedSeqRuleSetCount>>8), byte(chainedSeqRuleSetCount),
	)
	for _, offset := range chainedSeqRuleSetOffsets {
		buf = append(buf, byte(offset>>8), byte(offset))
	}

	buf = append(buf, l.Cov.Encode()...)
	buf = l.Backtrack.Append(buf)
	buf = l.Input.Append(buf)
	buf = l.Lookahead.Append(buf)

	for _, rr := range l.Rules {
		if rr == nil {
			continue
		}
		chainedSeqRuleCount := len(rr)
		buf = append(buf,
			byte(chainedSeqRuleCount>>8), byte(chainedSeqRuleCount),
		)

		pos := 2 + 2*chainedSeqRuleCount
		for _, rule := range rr {
			if pos > 0xFFFF {
				panic("ChainedSeqContext2 too large")
			}
			buf = append(buf,
				byte(pos>>8), byte(pos),
			)
			pos += 2 + 2*len(rule.Backtrack)
			pos += 2 + 2*len(rule.Input)
			pos += 2 + 2*len(rule.Lookahead)
			pos += 2 + 4*len(rule.Actions)
		}
		for _, rule := range rr {
			backtrackGlyphCount := len(rule.Backtrack)
			buf = append(buf,
				byte(backtrackGlyphCount>>8), byte(backtrackGlyphCount),
			)
			for _, gid := range rule.Backtrack {
				buf = append(buf, byte(gid>>8), byte(gid))
			}
			inputGlyphCount := len(rule.Input) + 1
			buf = append(buf,
				byte(inputGlyphCount>>8), byte(inputGlyphCount),
			)
			for _, gid := range rule.Input {
				buf = append(buf, byte(gid>>8), byte(gid))
			}
			lookaheadGlyphCount := len(rule.Lookahead)
			buf = append(buf,
				byte(lookaheadGlyphCount>>8), byte(lookaheadGlyphCount),
			)
			for _, gid := range rule.Lookahead {
				buf = append(buf, byte(gid>>8), byte(gid))
			}
			seqLookupCount := len(rule.Actions)
			buf = append(buf,
				byte(seqLookupCount>>8), byte(seqLookupCount),
			)
			for _, a := range rule.Actions {
				buf = append(buf,
					byte(a.SequenceIndex>>8), byte(a.SequenceIndex),
					byte(a.LookupListIndex>>8), byte(a.LookupListIndex),
				)
			}
		}
	}
	return buf
}

// ChainedSeqContext3 is used for GSUB type 6 and GPOS type 8 format 3 subtables
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#chained-sequence-context-format-3-coverage-based-glyph-contexts
type ChainedSeqContext3 struct {
	Backtrack []coverage.Set
	Input     []coverage.Set
	Lookahead []coverage.Set
	Actions   SeqLookups
}

func readChainedSeqContext3(p *parser.Parser, subtablePos int64) (Subtable, error) {
	backtrackCoverageOffsets, err := p.ReadUint16Slice()
	if err != nil {
		return nil, err
	}
	inputCoverageOffsets, err := p.ReadUint16Slice()
	if err != nil {
		return nil, err
	}
	lookaheadCoverageOffsets, err := p.ReadUint16Slice()
	if err != nil {
		return nil, err
	}
	if len(inputCoverageOffsets) < 1 {
		return nil, &parser.InvalidFontError{
			SubSystem: "sfnt/opentype/gtab",
			Reason:    "invalid glyph count in ChainedSeqContext3",
		}
	}

	seqLookupCount, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	actions, err := readNested(p, int(seqLookupCount))
	if err != nil {
		return nil, err
	}

	backtrackCov := make([]coverage.Set, len(backtrackCoverageOffsets))
	for i, offset := range backtrackCoverageOffsets {
		backtrackCov[i], err = coverage.ReadSet(p, subtablePos+int64(offset))
		if err != nil {
			return nil, err
		}
	}

	inputCov := make([]coverage.Set, len(inputCoverageOffsets))
	for i, offset := range inputCoverageOffsets {
		inputCov[i], err = coverage.ReadSet(p, subtablePos+int64(offset))
		if err != nil {
			return nil, err
		}
	}

	lookaheadCov := make([]coverage.Set, len(lookaheadCoverageOffsets))
	for i, offset := range lookaheadCoverageOffsets {
		lookaheadCov[i], err = coverage.ReadSet(p, subtablePos+int64(offset))
		if err != nil {
			return nil, err
		}
	}

	res := &ChainedSeqContext3{
		Backtrack: backtrackCov,
		Input:     inputCov,
		Lookahead: lookaheadCov,
		Actions:   actions,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *ChainedSeqContext3) Apply(keep keepGlyphFn, seq []glyph.Info, a, b int) *Match {
	p := a
	glyphsNeeded := len(l.Backtrack)
	for _, cov := range l.Backtrack {
		glyphsNeeded--
		p--
		for p-glyphsNeeded >= 0 && !keep(seq[p].Gid) {
			p--
		}
		if p-glyphsNeeded < 0 || !cov[seq[p].Gid] {
			return nil
		}
	}

	p = a
	matchPos := []int{p}
	glyphsNeeded = len(l.Input)
	for _, cov := range l.Input {
		if p+glyphsNeeded-1 >= b || !cov[seq[p].Gid] {
			return nil
		}
		matchPos = append(matchPos, p)
		glyphsNeeded--
		p++
		for p+glyphsNeeded < b && !keep(seq[p].Gid) {
			p++
		}
	}
	next := p

	glyphsNeeded = len(l.Lookahead)
	for _, cov := range l.Lookahead {
		if p+glyphsNeeded-1 >= len(seq) || !cov[seq[p].Gid] {
			return nil
		}
		glyphsNeeded--
		p++
		for p+glyphsNeeded < len(seq) && !keep(seq[p].Gid) {
			p++
		}
	}

	return &Match{
		InputPos: matchPos,
		Actions:  l.Actions,
		Next:     next,
	}
}

// EncodeLen implements the Subtable interface.
func (l *ChainedSeqContext3) EncodeLen() int {
	total := 10
	total += 2 * len(l.Backtrack)
	total += 2 * len(l.Input)
	total += 2 * len(l.Lookahead)
	total += 4 * len(l.Actions)
	for _, set := range l.Backtrack {
		cov := set.ToTable()
		total += cov.EncodeLen()
	}
	for _, set := range l.Input {
		cov := set.ToTable()
		total += cov.EncodeLen()
	}
	for _, set := range l.Lookahead {
		cov := set.ToTable()
		total += cov.EncodeLen()
	}
	return total
}

// Encode implements the Subtable interface.
func (l *ChainedSeqContext3) Encode() []byte {
	backtrackGlyphCount := len(l.Backtrack)
	inputGlyphCount := len(l.Input)
	lookaheadGlyphCount := len(l.Lookahead)
	seqLookupCount := len(l.Actions)

	total := 10
	total += 2 * len(l.Backtrack)
	total += 2 * len(l.Input)
	total += 2 * len(l.Lookahead)
	total += 4 * len(l.Actions)
	backtrackCoverageOffsets := make([]uint16, backtrackGlyphCount)
	for i, set := range l.Backtrack {
		backtrackCoverageOffsets[i] = uint16(total)
		cov := set.ToTable()
		total += cov.EncodeLen()
	}
	inputCoverageOffsets := make([]uint16, inputGlyphCount)
	for i, set := range l.Input {
		inputCoverageOffsets[i] = uint16(total)
		cov := set.ToTable()
		total += cov.EncodeLen()
	}
	lookaheadCoverageOffsets := make([]uint16, lookaheadGlyphCount)
	for i, set := range l.Lookahead {
		lookaheadCoverageOffsets[i] = uint16(total)
		cov := set.ToTable()
		total += cov.EncodeLen()
	}

	buf := make([]byte, 0, total)
	buf = append(buf,
		0, 3, // format
		byte(backtrackGlyphCount>>8), byte(backtrackGlyphCount),
	)
	for _, offset := range backtrackCoverageOffsets {
		buf = append(buf, byte(offset>>8), byte(offset))
	}
	buf = append(buf,
		byte(inputGlyphCount>>8), byte(inputGlyphCount),
	)
	for _, offset := range inputCoverageOffsets {
		buf = append(buf, byte(offset>>8), byte(offset))
	}
	buf = append(buf,
		byte(lookaheadGlyphCount>>8), byte(lookaheadGlyphCount),
	)
	for _, offset := range lookaheadCoverageOffsets {
		buf = append(buf, byte(offset>>8), byte(offset))
	}

	buf = append(buf,
		byte(seqLookupCount>>8), byte(seqLookupCount),
	)
	for _, action := range l.Actions {
		buf = append(buf,
			byte(action.SequenceIndex>>8), byte(action.SequenceIndex),
			byte(action.LookupListIndex>>8), byte(action.LookupListIndex),
		)
	}

	for _, set := range l.Backtrack {
		cov := set.ToTable()
		buf = append(buf, cov.Encode()...)
	}
	for _, set := range l.Input {
		cov := set.ToTable()
		buf = append(buf, cov.Encode()...)
	}
	for _, set := range l.Lookahead {
		cov := set.ToTable()
		buf = append(buf, cov.Encode()...)
	}
	return buf
}
