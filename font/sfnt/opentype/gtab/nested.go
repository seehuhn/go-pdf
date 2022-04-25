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

// SeqContext1 is used for GSUB type 5 format 1 subtables and GPOS type 7 format 1 subtables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#sequence-context-format-1-simple-glyph-contexts
type SeqContext1 struct {
	Cov   coverage.Table
	Rules [][]*SeqRule
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

	cov, err := coverage.ReadTable(p, subtablePos+int64(coverageOffset))
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
			actions := make(Nested, seqLookupCount)
			for k := range actions {
				buf, err := p.ReadBytes(4)
				if err != nil {
					return nil, err
				}
				actions[k].SequenceIndex = uint16(buf[0])<<8 | uint16(buf[1])
				actions[k].LookupListIndex = LookupIndex(buf[2])<<8 | LookupIndex(buf[3])
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

		actions := make(Nested, len(rule.Actions))
		for j, a := range rule.Actions {
			idx := int(a.SequenceIndex)
			if idx < len(matchPos) {
				actions[j].SequenceIndex = uint16(matchPos[idx])
				actions[j].LookupListIndex = a.LookupListIndex
			}
		}

		return seq, p + 1, actions
	}

	return seq, -1, nil
}

// EncodeLen implements the Subtable interface.
func (l *SeqContext1) EncodeLen() int {
	total := 6 + 2*len(l.Rules)
	for _, rule := range l.Rules {
		total += 2 + 2*len(rule)
		for _, r := range rule {
			total += 4 + 2*len(r.Input) + 4*len(r.Actions)
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
	for i, rule := range l.Rules {
		seqRuleSetOffsets[i] = uint16(total)
		total += 2 + 2*len(rule)
		for _, r := range rule {
			total += 4 + 2*len(r.Input) + 4*len(r.Actions)
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
	for i, rule := range l.Rules {
		if len(buf) != int(seqRuleSetOffsets[i]) {
			panic("internal error") // TODO(voss): remove
		}
		seqRuleCount := len(rule)
		buf = append(buf,
			byte(seqRuleCount>>8), byte(seqRuleCount),
		)
		pos := 2 + 2*seqRuleCount
		for _, r := range rule {
			buf = append(buf,
				byte(pos>>8), byte(pos),
			)
			pos += 4 + 2*len(r.Input) + 4*len(r.Actions)
		}
		for _, r := range rule {
			glyphCount := len(r.Input) + 1
			seqLookupCount := len(r.Actions)
			buf = append(buf,
				byte(glyphCount>>8), byte(glyphCount),
				byte(seqLookupCount>>8), byte(seqLookupCount),
			)
			for _, gid := range r.Input {
				buf = append(buf, byte(gid>>8), byte(gid))
			}
			for _, action := range r.Actions {
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
	Rules   [][]*ClassSequenceRule
}

// ClassSequenceRule describes a sequence of glyph classes and the actions to
// be performed
type ClassSequenceRule struct {
	Input   []uint16 // excludes the first input glyph, since this is in Cov
	Actions Nested
}

func readSeqContext2(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(4)
	if err != nil {
		return nil, err
	}
	coverageOffset := uint16(buf[0])<<8 | uint16(buf[1])
	classDefOffset := uint16(buf[2])<<8 | uint16(buf[3])
	seqRuleSetOffsets, err := p.ReadUint16Slice()
	if err != nil {
		return nil, err
	}

	cov, err := coverage.ReadTable(p, subtablePos+int64(coverageOffset))
	if err != nil {
		return nil, err
	}
	if len(cov) > len(seqRuleSetOffsets) {
		cov.Prune(len(seqRuleSetOffsets))
	} else {
		seqRuleSetOffsets = seqRuleSetOffsets[:len(cov)]
	}

	classDef, err := classdef.ReadTable(p, subtablePos+int64(classDefOffset))
	if err != nil {
		return nil, err
	}

	res := &SeqContext2{
		Cov:     cov,
		Classes: classDef,
		Rules:   make([][]*ClassSequenceRule, len(seqRuleSetOffsets)),
	}

	for i, seqRuleSetOffset := range seqRuleSetOffsets {
		base := subtablePos + int64(seqRuleSetOffset)
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
			actions := make(Nested, seqLookupCount)
			for k := range actions {
				buf, err := p.ReadBytes(4)
				if err != nil {
					return nil, err
				}
				actions[k].SequenceIndex = uint16(buf[0])<<8 | uint16(buf[1])
				actions[k].LookupListIndex = LookupIndex(buf[2])<<8 | LookupIndex(buf[3])
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
	ruleIdx, ok := l.Cov[seq[i].Gid]
	if !ok {
		return seq, -1, nil
	}
	rule := l.Rules[ruleIdx]

	var matchPos []int
ruleLoop:
	for _, r := range rule {
		p := i
		matchPos = append(matchPos[:0], p)
		glyphsNeeded := len(r.Input)
		for _, cls := range r.Input {
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

		actions := make(Nested, len(r.Actions))
		for j, a := range r.Actions {
			idx := int(a.SequenceIndex)
			if idx < len(matchPos) {
				actions[j].SequenceIndex = uint16(matchPos[idx])
				actions[j].LookupListIndex = a.LookupListIndex
			}
		}

		return seq, p + 1, actions
	}

	return seq, -1, nil
}

// EncodeLen implements the Subtable interface.
func (l *SeqContext2) EncodeLen() int {
	total := 8 + 2*len(l.Rules)
	total += l.Cov.EncodeLen()
	total += l.Classes.AppendLen()
	for _, rule := range l.Rules {
		total += 2 + 2*len(rule)
		for _, r := range rule {
			total += 4 + 2*len(r.Input) + 4*len(r.Actions)
		}
	}
	return total
}

// Encode implements the Subtable interface.
func (l *SeqContext2) Encode() []byte {
	seqRuleSetCount := len(l.Rules)

	total := 8 + 2*seqRuleSetCount
	seqRuleSetOffsets := make([]uint16, seqRuleSetCount)
	for i, rule := range l.Rules {
		seqRuleSetOffsets[i] = uint16(total)
		total += 2 + 2*len(rule)
		for _, r := range rule {
			total += 4 + 2*len(r.Input) + 4*len(r.Actions)
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
	for i, rule := range l.Rules {
		if len(buf) != int(seqRuleSetOffsets[i]) {
			panic("internal error") // TODO(voss): remove
		}
		seqRuleCount := len(rule)
		buf = append(buf,
			byte(seqRuleCount>>8), byte(seqRuleCount),
		)
		pos := 2 + 2*seqRuleCount
		for _, r := range rule {
			buf = append(buf,
				byte(pos>>8), byte(pos),
			)
			pos += 4 + 2*len(r.Input) + 4*len(r.Actions)
		}
		for _, r := range rule {
			glyphCount := len(r.Input) + 1
			seqLookupCount := len(r.Actions)
			buf = append(buf,
				byte(glyphCount>>8), byte(glyphCount),
				byte(seqLookupCount>>8), byte(seqLookupCount),
			)
			for _, gid := range r.Input {
				buf = append(buf, byte(gid>>8), byte(gid))
			}
			for _, action := range r.Actions {
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
	Covv    []coverage.Table
	Actions Nested
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

	actions := make(Nested, seqLookupCount)
	for k := range actions {
		buf, err := p.ReadBytes(4)
		if err != nil {
			return nil, err
		}
		actions[k].SequenceIndex = uint16(buf[0])<<8 | uint16(buf[1])
		actions[k].LookupListIndex = LookupIndex(buf[2])<<8 | LookupIndex(buf[3])
	}

	cov := make([]coverage.Table, glyphCount)
	for i, offset := range coverageOffsets {
		cov[i], err = coverage.ReadTable(p, subtablePos+int64(offset))
		if err != nil {
			return nil, err
		}
	}

	res := &SeqContext3{
		Covv:    cov,
		Actions: actions,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *SeqContext3) Apply(keep KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	if !l.Covv[0].Contains(seq[i].Gid) {
		return seq, -1, nil
	}

	p := i
	matchPos := []int{p}
	glyphsNeeded := len(l.Covv) - 1
	for _, cov := range l.Covv[1:] {
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

	actions := make(Nested, len(l.Actions))
	for j, a := range l.Actions {
		idx := int(a.SequenceIndex)
		if idx < len(matchPos) {
			actions[j].SequenceIndex = uint16(matchPos[idx])
			actions[j].LookupListIndex = a.LookupListIndex
		}
	}

	return seq, p + 1, actions
}

// EncodeLen implements the Subtable interface.
func (l *SeqContext3) EncodeLen() int {
	total := 6 + 2*len(l.Covv) + 4*len(l.Actions)
	for _, cov := range l.Covv {
		total += cov.EncodeLen()
	}
	return total
}

// Encode implements the Subtable interface.
func (l *SeqContext3) Encode() []byte {
	glyphCount := len(l.Covv)
	seqLookupCount := len(l.Actions)

	total := 6 + 2*len(l.Covv) + 4*len(l.Actions)
	coverageOffsets := make([]uint16, glyphCount)
	for i, cov := range l.Covv {
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
	for i, cov := range l.Covv {
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

	cov, err := coverage.ReadTable(p, subtablePos+int64(coverageOffset))
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
			actions := make([]SeqLookup, seqLookupCount)
			for i := 0; i < int(seqLookupCount); i++ {
				buf, err := p.ReadBytes(4)
				if err != nil {
					return nil, err
				}
				actions[i].SequenceIndex = uint16(buf[0])<<8 | uint16(buf[1])
				actions[i].LookupListIndex = LookupIndex(buf[2])<<8 | LookupIndex(buf[3])
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

		actions := make(Nested, len(rule.Actions))
		for j, a := range rule.Actions {
			idx := int(a.SequenceIndex)
			if idx < len(matchPos) {
				actions[j].SequenceIndex = uint16(matchPos[idx])
				actions[j].LookupListIndex = a.LookupListIndex
			}
		}

		return seq, next, actions
	}

	return seq, -1, nil
}

// EncodeLen implements the Subtable interface.
func (l *ChainedSeqContext1) EncodeLen() int {
	total := 6 + 2*len(l.Rules)
	total += l.Cov.EncodeLen()
	for _, rules := range l.Rules {
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
