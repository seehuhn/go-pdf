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
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
)

// SeqLookup describes the actions for contextual and chained contextual
// lookups.
type SeqLookup struct {
	SequenceIndex   uint16
	LookupListIndex LookupIndex
}

// Nested describes the actions of nested lookups.
type Nested []SeqLookup

func readNested(p *parser.Parser, seqLookupCount int) (Nested, error) {
	res := make(Nested, seqLookupCount)
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

func (actions Nested) substituteLocations(matchPos []int) Nested {
	res := make(Nested, 0, len(actions))
	for _, action := range actions {
		idx := int(action.SequenceIndex)
		if idx >= len(matchPos) {
			continue
		}
		res = append(res, SeqLookup{
			SequenceIndex:   uint16(matchPos[idx]),
			LookupListIndex: action.LookupListIndex,
		})
	}
	return res
}

// SeqContext1 is used for GSUB type 5 format 1 subtables and GPOS type 7 format 1 subtables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#sequence-context-format-1-simple-glyph-contexts
type SeqContext1 struct {
	Cov   coverage.Table
	Rules [][]*SeqRule // indexed by coverage index
}

// SeqRule describes a sequence of glyphs and the actions to be performed
type SeqRule struct {
	Input   []font.GlyphID // excludes the first input glyph, since this is in Cov
	Actions Nested
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
				return nil, &font.InvalidFontError{
					SubSystem: "sfnt/opentype/gtab",
					Reason:    "invalid glyph count in SeqContext1",
				}
			}
			seqLookupCount := int(buf[2])<<8 | int(buf[3])
			inputSequence := make([]font.GlyphID, glyphCount-1)
			for k := range inputSequence {
				xk, err := p.ReadUint16()
				if err != nil {
					return nil, err
				}
				inputSequence[k] = font.GlyphID(xk)
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
func (l *SeqContext1) Apply(keep KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	if !keep(seq[i].Gid) {
		return seq, -1, nil
	}
	rulesIdx, ok := l.Cov[seq[i].Gid]
	if !ok {
		return seq, -1, nil
	}
	rules := l.Rules[rulesIdx]

	var matchPos []int
ruleLoop:
	for _, rule := range rules {
		p := i
		matchPos = append(matchPos[:0], p)
		glyphsNeeded := len(rule.Input)
		for _, gid := range rule.Input {
			glyphsNeeded--
			p++
			for p+glyphsNeeded < len(seq) && !keep(seq[p].Gid) {
				p++
			}
			if p+glyphsNeeded >= len(seq) || seq[p].Gid != gid {
				continue ruleLoop
			}
			matchPos = append(matchPos, p)
		}

		actions := rule.Actions.substituteLocations(matchPos)
		return seq, p + 1, actions
	}

	return seq, -1, nil
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
	for i, rules := range l.Rules {
		if rules == nil {
			continue
		}
		if len(buf) != int(seqRuleSetOffsets[i]) {
			panic("internal error") // TODO(voss): remove
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
	Cov     coverage.Table
	Classes classdef.Table
	Rules   [][]*ClassSequenceRule // indexed by class index of the first glyph
}

// ClassSequenceRule describes a sequence of glyph classes and the actions to
// be performed
type ClassSequenceRule struct {
	Input   []uint16 // excludes the first input glyph
	Actions Nested
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

	res := &SeqContext2{
		Cov:     cov,
		Classes: classDef,
		Rules:   make([][]*ClassSequenceRule, len(classSeqRuleSetOffsets)),
	}

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
		res.Rules[i] = make([]*ClassSequenceRule, len(seqRuleOffsets))
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
				return nil, &font.InvalidFontError{
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
			res.Rules[i][j] = &ClassSequenceRule{
				Input:   inputSequence,
				Actions: actions,
			}
		}
	}

	return res, nil
}

// Apply implements the Subtable interface.
func (l *SeqContext2) Apply(keep KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	if !keep(seq[i].Gid) {
		return seq, -1, nil
	}
	_, ok := l.Cov[seq[i].Gid]
	if !ok {
		return seq, -1, nil
	}
	ruleIdx := l.Classes[seq[i].Gid]
	rules := l.Rules[ruleIdx]

	var matchPos []int
ruleLoop:
	for _, rule := range rules {
		p := i
		matchPos = append(matchPos[:0], p)
		glyphsNeeded := len(rule.Input)
		for _, cls := range rule.Input {
			glyphsNeeded--
			p++
			for p+glyphsNeeded < len(seq) && !keep(seq[p].Gid) {
				p++
			}
			if p+glyphsNeeded >= len(seq) || l.Classes[seq[p].Gid] != cls {
				continue ruleLoop
			}
			matchPos = append(matchPos, p)
		}

		actions := rule.Actions.substituteLocations(matchPos)
		return seq, p + 1, actions
	}

	return seq, -1, nil
}

// EncodeLen implements the Subtable interface.
func (l *SeqContext2) EncodeLen() int {
	total := 8 + 2*len(l.Rules)
	total += l.Cov.EncodeLen()
	total += l.Classes.AppendLen()
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
	total += l.Classes.AppendLen()

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
	for i, rules := range l.Rules {
		if rules == nil {
			continue
		}
		if len(buf) != int(seqRuleSetOffsets[i]) {
			panic("internal error") // TODO(voss): remove
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
	buf = l.Classes.Append(buf)
	return buf
}

// SeqContext3 is used for GSUB type 5 format 3 and GPOS type 7 format 3 subtables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#sequence-context-format-3-coverage-based-glyph-contexts
type SeqContext3 struct {
	InputCov []coverage.Table
	Actions  Nested
}

func readSeqContext3(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(4)
	if err != nil {
		return nil, err
	}
	glyphCount := int(buf[0])<<8 | int(buf[1])
	if glyphCount < 1 {
		return nil, &font.InvalidFontError{
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
		InputCov: cov,
		Actions:  actions,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *SeqContext3) Apply(keep KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	if !l.InputCov[0].Contains(seq[i].Gid) {
		return seq, -1, nil
	}

	p := i
	matchPos := []int{p}
	glyphsNeeded := len(l.InputCov) - 1
	for _, cov := range l.InputCov[1:] {
		glyphsNeeded--
		p++
		for p+glyphsNeeded < len(seq) && !keep(seq[p].Gid) {
			p++
		}
		if p+glyphsNeeded >= len(seq) || !cov.Contains(seq[p].Gid) {
			return seq, -1, nil
		}
		matchPos = append(matchPos, p)
	}

	actions := l.Actions.substituteLocations(matchPos)
	return seq, p + 1, actions
}

// EncodeLen implements the Subtable interface.
func (l *SeqContext3) EncodeLen() int {
	total := 6 + 2*len(l.InputCov) + 4*len(l.Actions)
	for _, cov := range l.InputCov {
		total += cov.EncodeLen()
	}
	return total
}

// Encode implements the Subtable interface.
func (l *SeqContext3) Encode() []byte {
	glyphCount := len(l.InputCov)
	seqLookupCount := len(l.Actions)

	total := 6 + 2*len(l.InputCov) + 4*len(l.Actions)
	coverageOffsets := make([]uint16, glyphCount)
	for i, cov := range l.InputCov {
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
	for i, cov := range l.InputCov {
		if len(buf) != int(coverageOffsets[i]) {
			panic("internal error") // TODO(voss): remove
		}
		buf = append(buf, cov.Encode()...)
	}
	return buf
}

// ChainedSeqContext1 is used for GSUB type 6 format 1 and GPOS type 8 format 1 subtables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#chained-sequence-context-format-1-simple-glyph-contexts
type ChainedSeqContext1 struct {
	Cov   coverage.Table
	Rules [][]*ChainedSeqRule
}

// ChainedSeqRule is used for GSUB type 6 format 1 and GPOS type 8 format 1 subtables.
type ChainedSeqRule struct {
	Backtrack []font.GlyphID
	Input     []font.GlyphID // excludes the first input glyph, since this is in Cov
	Lookahead []font.GlyphID
	Actions   Nested
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

		rules[i] = make([]*ChainedSeqRule, len(chainedSeqRuleOffsets))
		for j, chainedSeqRuleOffset := range chainedSeqRuleOffsets {
			err = p.SeekPos(base + int64(chainedSeqRuleOffset))
			if err != nil {
				return nil, err
			}

			backtrackSequence, err := p.ReadGIDSlice()
			if err != nil {
				return nil, err
			}
			inputGlyphCount, err := p.ReadUint16()
			if err != nil {
				return nil, err
			}
			inputSequence := make([]font.GlyphID, inputGlyphCount-1)
			for k := range inputSequence {
				val, err := p.ReadUint16()
				if err != nil {
					return nil, err
				}
				inputSequence[k] = font.GlyphID(val)
			}
			lookaheadSequence, err := p.ReadGIDSlice()
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
			rules[i][j] = &ChainedSeqRule{
				Backtrack: backtrackSequence,
				Input:     inputSequence,
				Lookahead: lookaheadSequence,
				Actions:   actions,
			}
		}
	}

	res := &ChainedSeqContext1{
		Cov:   cov,
		Rules: rules,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *ChainedSeqContext1) Apply(keep KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	rulesIdx, ok := l.Cov[seq[i].Gid]
	if !ok {
		return seq, -1, nil
	}
	rules := l.Rules[rulesIdx]

	var matchPos []int
ruleLoop:
	for _, rule := range rules {
		glyphsNeeded := len(rule.Backtrack)
		p := i
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

		p = i
		matchPos = append(matchPos[:0], p)
		glyphsNeeded = len(rule.Input) + len(rule.Lookahead)
		for _, gid := range rule.Input {
			glyphsNeeded--
			p++
			for p+glyphsNeeded < len(seq) && !keep(seq[p].Gid) {
				p++
			}
			if p+glyphsNeeded >= len(seq) || seq[p].Gid != gid {
				continue ruleLoop
			}
			matchPos = append(matchPos, p)
		}
		next := p + 1

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

		actions := rule.Actions.substituteLocations(matchPos)
		return seq, next, actions
	}

	return seq, -1, nil
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

	for i, rules := range l.Rules {
		if rules == nil {
			continue
		}
		if len(buf) != int(chainedSeqRuleSetOffsets[i]) {
			panic("internal error") // TODO(voss): remove
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
	Rules     [][]*ChainedClassSeqRule
}

// ChainedClassSeqRule is used for GSUB type 6 format 2 and GPOS type 8 format 2 subtables.
type ChainedClassSeqRule struct {
	Backtrack []uint16
	Input     []uint16 // excludes the first input glyph, since this is in Cov
	Lookahead []uint16
	Actions   Nested
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

	// TODO(voss): this is indexed by class, not coverage index!
	if len(cov) > len(chainedClassSeqRuleSetOffsets) {
		cov.Prune(len(chainedClassSeqRuleSetOffsets))
	} else {
		chainedClassSeqRuleSetOffsets = chainedClassSeqRuleSetOffsets[:len(cov)]
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
func (l *ChainedSeqContext2) Apply(keep KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	rulesIdx, ok := l.Cov[seq[i].Gid]
	if !ok {
		return seq, -1, nil
	}
	rules := l.Rules[rulesIdx]

	var matchPos []int
ruleLoop:
	for _, rule := range rules {
		glyphsNeeded := len(rule.Backtrack)
		p := i
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

		p = i
		matchPos = append(matchPos[:0], p)
		glyphsNeeded = len(rule.Input) + len(rule.Lookahead)
		for _, cls := range rule.Input {
			glyphsNeeded--
			p++
			for p+glyphsNeeded < len(seq) && !keep(seq[p].Gid) {
				p++
			}
			if p+glyphsNeeded >= len(seq) || l.Input[seq[p].Gid] != cls {
				continue ruleLoop
			}
			matchPos = append(matchPos, p)
		}
		next := p + 1

		for _, cls := range rule.Lookahead {
			// TODO(voss): is this right?  What if keep(seq[i].Gid) == false?
			glyphsNeeded--
			p++
			for p+glyphsNeeded < len(seq) && !keep(seq[p].Gid) {
				p++
			}
			if p+glyphsNeeded >= len(seq) || l.Lookahead[seq[p].Gid] != cls {
				continue ruleLoop
			}
		}

		actions := rule.Actions.substituteLocations(matchPos)
		return seq, next, actions
	}

	return seq, -1, nil
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

	for i, rules := range l.Rules {
		if rules == nil {
			continue
		}
		if len(buf) != int(chainedSeqRuleSetOffsets[i]) {
			panic("internal error") // TODO(voss): remove
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

// ChainedSeqContext3 is used for GSUB type 6 and GPOS type 8 format 3 subtables
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#chained-sequence-context-format-3-coverage-based-glyph-contexts
type ChainedSeqContext3 struct {
	BacktrackCov []coverage.Table
	InputCov     []coverage.Table
	LookaheadCov []coverage.Table
	Actions      Nested
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
		return nil, &font.InvalidFontError{
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

	backtrackCov := make([]coverage.Table, len(backtrackCoverageOffsets))
	for i, offset := range backtrackCoverageOffsets {
		backtrackCov[i], err = coverage.Read(p, subtablePos+int64(offset))
		if err != nil {
			return nil, err
		}
	}

	inputCov := make([]coverage.Table, len(inputCoverageOffsets))
	for i, offset := range inputCoverageOffsets {
		inputCov[i], err = coverage.Read(p, subtablePos+int64(offset))
		if err != nil {
			return nil, err
		}
	}

	lookaheadCov := make([]coverage.Table, len(lookaheadCoverageOffsets))
	for i, offset := range lookaheadCoverageOffsets {
		lookaheadCov[i], err = coverage.Read(p, subtablePos+int64(offset))
		if err != nil {
			return nil, err
		}
	}

	res := &ChainedSeqContext3{
		BacktrackCov: backtrackCov,
		InputCov:     inputCov,
		LookaheadCov: lookaheadCov,
		Actions:      actions,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *ChainedSeqContext3) Apply(keep KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	glyphsNeeded := len(l.BacktrackCov)
	p := i
	for _, cov := range l.BacktrackCov {
		glyphsNeeded--
		p--
		for p-glyphsNeeded >= 0 && !keep(seq[p].Gid) {
			p--
		}
		if p-glyphsNeeded < 0 || !cov.Contains(seq[p].Gid) {
			return seq, -1, nil
		}
	}

	matchPos := make([]int, 0, len(l.InputCov))

	p = i
	matchPos = append(matchPos, p)
	glyphsNeeded = len(l.InputCov) + len(l.LookaheadCov)
	for _, cov := range l.InputCov {
		// TODO(voss): is this right?  What if keep(seq[i].Gid) == false?
		glyphsNeeded--
		p++
		for p+glyphsNeeded < len(seq) && !keep(seq[p].Gid) {
			p++
		}
		if p+glyphsNeeded >= len(seq) || !cov.Contains(seq[p].Gid) {
			return seq, -1, nil
		}
		matchPos = append(matchPos, p)
	}
	next := p + 1

	for _, cov := range l.LookaheadCov {
		glyphsNeeded--
		p++
		for p+glyphsNeeded < len(seq) && !keep(seq[p].Gid) {
			p++
		}
		if p+glyphsNeeded >= len(seq) || !cov.Contains(seq[p].Gid) {
			return seq, -1, nil
		}
	}

	actions := l.Actions.substituteLocations(matchPos)

	return seq, next, actions
}

// EncodeLen implements the Subtable interface.
func (l *ChainedSeqContext3) EncodeLen() int {
	total := 10
	total += 2 * len(l.BacktrackCov)
	total += 2 * len(l.InputCov)
	total += 2 * len(l.LookaheadCov)
	total += 4 * len(l.Actions)
	for _, cov := range l.BacktrackCov {
		total += cov.EncodeLen()
	}
	for _, cov := range l.InputCov {
		total += cov.EncodeLen()
	}
	for _, cov := range l.LookaheadCov {
		total += cov.EncodeLen()
	}
	return total
}

// Encode implements the Subtable interface.
func (l *ChainedSeqContext3) Encode() []byte {
	backtrackGlyphCount := len(l.BacktrackCov)
	inputGlyphCount := len(l.InputCov)
	lookaheadGlyphCount := len(l.LookaheadCov)
	seqLookupCount := len(l.Actions)

	total := 10
	total += 2 * len(l.BacktrackCov)
	total += 2 * len(l.InputCov)
	total += 2 * len(l.LookaheadCov)
	total += 4 * len(l.Actions)
	backtrackCoverageOffsets := make([]uint16, backtrackGlyphCount)
	for i, cov := range l.BacktrackCov {
		backtrackCoverageOffsets[i] = uint16(total)
		total += cov.EncodeLen()
	}
	inputCoverageOffsets := make([]uint16, inputGlyphCount)
	for i, cov := range l.InputCov {
		inputCoverageOffsets[i] = uint16(total)
		total += cov.EncodeLen()
	}
	lookaheadCoverageOffsets := make([]uint16, lookaheadGlyphCount)
	for i, cov := range l.LookaheadCov {
		lookaheadCoverageOffsets[i] = uint16(total)
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

	for i, cov := range l.BacktrackCov {
		if len(buf) != int(backtrackCoverageOffsets[i]) {
			panic("internal error") // TODO(voss): remove
		}
		buf = append(buf, cov.Encode()...)
	}
	for i, cov := range l.InputCov {
		if len(buf) != int(inputCoverageOffsets[i]) {
			panic("internal error") // TODO(voss): remove
		}
		buf = append(buf, cov.Encode()...)
	}
	for i, cov := range l.LookaheadCov {
		if len(buf) != int(lookaheadCoverageOffsets[i]) {
			panic("internal error") // TODO(voss): remove
		}
		buf = append(buf, cov.Encode()...)
	}
	return buf
}
