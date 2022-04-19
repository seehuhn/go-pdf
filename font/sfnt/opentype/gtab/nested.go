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
	Rules [][]SequenceRule
}

// SequenceRule describes a sequence of glyphs and the actions to be performed
type SequenceRule struct {
	In      []font.GlyphID // excludes the first input glyph, since this is in Cov
	Actions Nested
}

func readSeqContext1(p *parser.Parser, subtablePos int64) (Subtable, error) {
	coverageOffset, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}
	seqRuleSetOffsets, err := p.ReadUInt16Slice()
	if err != nil {
		return nil, err
	}

	cov, err := coverage.ReadTable(p, subtablePos+int64(coverageOffset))
	if err != nil {
		return nil, err
	}
	// The seqRuleSetCount should match the number of glyphs in the Coverage
	// table. If these differ, the extra coverage glyphs or extra sequence rule
	// sets are ignored.
	if len(cov) < len(seqRuleSetOffsets) {
		seqRuleSetOffsets = seqRuleSetOffsets[:len(cov)]
	} else if len(cov) > len(seqRuleSetOffsets) {
		var gg []font.GlyphID
		for gid, class := range cov {
			if class >= len(seqRuleSetOffsets) {
				gg = append(gg, gid)
			}
		}
		for _, gid := range gg {
			delete(cov, font.GlyphID(gid))
		}
	}

	res := &SeqContext1{
		Cov:   cov,
		Rules: make([][]SequenceRule, len(seqRuleSetOffsets)),
	}

	for i, seqRuleSetOffset := range seqRuleSetOffsets {
		base := subtablePos + int64(seqRuleSetOffset)
		err = p.SeekPos(base)
		if err != nil {
			return nil, err
		}
		seqRuleOffsets, err := p.ReadUInt16Slice()
		if err != nil {
			return nil, err
		}
		res.Rules[i] = make([]SequenceRule, len(seqRuleOffsets))
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
				xk, err := p.ReadUInt16()
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
			res.Rules[i][j] = SequenceRule{
				In:      inputSequence,
				Actions: actions,
			}
		}
	}

	return res, nil
}

// Apply implements the Subtable interface.
func (l *SeqContext1) Apply(keep KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	ruleIdx, ok := l.Cov[seq[i].Gid]
	if !ok {
		return seq, -1, nil
	}
	rule := l.Rules[ruleIdx]

	var strays []font.Glyph
	var rr []rune
ruleLoop:
	for _, r := range rule {
		strays = strays[:0]
		rr = rr[:0]
		p := i
		for _, gid := range r.In {
			p++
			for p < len(seq) && !keep(seq[p].Gid) {
				strays = append(strays, seq[p])
				p++
			}
			if p >= len(seq) {
				continue ruleLoop
			}
			if seq[p].Gid != gid {
				continue ruleLoop
			}
			rr = append(rr, seq[p].Text...)
		}

		panic("not implemented")
	}

	return seq, -1, nil
}

// EncodeLen implements the Subtable interface.
func (l *SeqContext1) EncodeLen() int {
	total := 6 + 2*len(l.Rules)
	for _, rule := range l.Rules {
		total += 2 + 2*len(rule)
		for _, r := range rule {
			total += 4 + 2*len(r.In) + 4*len(r.Actions)
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
			total += 4 + 2*len(r.In) + 4*len(r.Actions)
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
			pos += 4 + 2*len(r.In) + 4*len(r.Actions)
		}
		for _, r := range rule {
			glyphCount := len(r.In) + 1
			seqLookupCount := len(r.Actions)
			buf = append(buf,
				byte(glyphCount>>8), byte(glyphCount),
				byte(seqLookupCount>>8), byte(seqLookupCount),
			)
			for _, gid := range r.In {
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
	Rules   [][]ClassSequenceRule
}

// ClassSequenceRule describes a sequence of glyph classes and the actions to
// be performed
type ClassSequenceRule struct {
	In      []uint16 // excludes the first input glyph, since this is in Cov
	Actions Nested
}

func readSeqContext2(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(4)
	if err != nil {
		return nil, err
	}
	coverageOffset := uint16(buf[0])<<8 | uint16(buf[1])
	classDefOffset := uint16(buf[2])<<8 | uint16(buf[3])
	seqRuleSetOffsets, err := p.ReadUInt16Slice()
	if err != nil {
		return nil, err
	}

	cov, err := coverage.ReadTable(p, subtablePos+int64(coverageOffset))
	if err != nil {
		return nil, err
	}
	// The seqRuleSetCount should match the number of glyphs in the Coverage
	// table. If these differ, the extra coverage glyphs or extra sequence rule
	// sets are ignored.
	if len(cov) < len(seqRuleSetOffsets) {
		seqRuleSetOffsets = seqRuleSetOffsets[:len(cov)]
	} else if len(cov) > len(seqRuleSetOffsets) {
		var gg []font.GlyphID
		for gid, class := range cov {
			if class >= len(seqRuleSetOffsets) {
				gg = append(gg, gid)
			}
		}
		for _, gid := range gg {
			delete(cov, font.GlyphID(gid))
		}
	}

	classDef, err := classdef.ReadTable(p, subtablePos+int64(classDefOffset))
	if err != nil {
		return nil, err
	}

	res := &SeqContext2{
		Cov:     cov,
		Classes: classDef,
		Rules:   make([][]ClassSequenceRule, len(seqRuleSetOffsets)),
	}

	for i, seqRuleSetOffset := range seqRuleSetOffsets {
		base := subtablePos + int64(seqRuleSetOffset)
		err = p.SeekPos(base)
		if err != nil {
			return nil, err
		}
		seqRuleOffsets, err := p.ReadUInt16Slice()
		if err != nil {
			return nil, err
		}
		res.Rules[i] = make([]ClassSequenceRule, len(seqRuleOffsets))
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
				xk, err := p.ReadUInt16()
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
			res.Rules[i][j] = ClassSequenceRule{
				In:      inputSequence,
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

	var strays []font.Glyph
	var rr []rune
ruleLoop:
	for _, r := range rule {
		strays = strays[:0]
		rr = rr[:0]
		p := i
		for _, cls := range r.In {
			p++
			for p < len(seq) && !keep(seq[p].Gid) {
				strays = append(strays, seq[p])
				p++
			}
			if p >= len(seq) {
				continue ruleLoop
			}
			if l.Classes[seq[p].Gid] != cls {
				continue ruleLoop
			}
			rr = append(rr, seq[p].Text...)
		}

		panic("not implemented")
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
			total += 4 + 2*len(r.In) + 4*len(r.Actions)
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
			total += 4 + 2*len(r.In) + 4*len(r.Actions)
		}
	}
	coverageOffset := total
	total += l.Cov.EncodeLen()
	classDefOffset := total
	total += l.Classes.AppendLen()

	buf := make([]byte, 0, total)
	buf = append(buf,
		0, 1, // format
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
			pos += 4 + 2*len(r.In) + 4*len(r.Actions)
		}
		for _, r := range rule {
			glyphCount := len(r.In) + 1
			seqLookupCount := len(r.Actions)
			buf = append(buf,
				byte(glyphCount>>8), byte(glyphCount),
				byte(seqLookupCount>>8), byte(seqLookupCount),
			)
			for _, gid := range r.In {
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
