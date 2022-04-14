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
	"fmt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
)

// readGsubSubtable reads a GSUB subtable.
// This function can be used as the SubtableReader argument to Read().
func readGsubSubtable(p *parser.Parser, pos int64, meta *LookupMetaInfo) (Subtable, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	format, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}

	switch 10*meta.LookupType + format {
	case 1_1:
		return readGsub1_1(p, pos)
	case 1_2:
		return readGsub1_2(p, pos)
	case 2_1:
		return readGsub2_1(p, pos)
	case 3_1:
		return readGsub3_1(p, pos)
	default:
		msg := fmt.Sprintf("GSUB %d.%d\n", meta.LookupType, format)
		fmt.Print(msg)
		return notImplementedGsubSubtable{meta.LookupType, format}, nil
	}
}

type notImplementedGsubSubtable struct {
	lookupType, lookupFormat uint16
}

func (st notImplementedGsubSubtable) Apply(_ KeepGlyphFn, _ []font.Glyph, _ int) ([]font.Glyph, int, Nested) {
	msg := fmt.Sprintf("GSUB lookup type %d, format %d not implemented",
		st.lookupType, st.lookupFormat)
	panic(msg)
}

func (st notImplementedGsubSubtable) EncodeLen() int {
	msg := fmt.Sprintf("GSUB lookup type %d, format %d not implemented",
		st.lookupType, st.lookupFormat)
	panic(msg)
}

func (st notImplementedGsubSubtable) Encode() []byte {
	msg := fmt.Sprintf("GSUB lookup type %d, format %d not implemented",
		st.lookupType, st.lookupFormat)
	panic(msg)
}

// Gsub1_1 is a Single Substitution GSUB subtable (type 1, format 1).
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#11-single-substitution-format-1
type Gsub1_1 struct {
	Cov   coverage.Table
	Delta font.GlyphID
}

func readGsub1_1(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(4)
	if err != nil {
		return nil, err
	}
	coverageOffset := int64(buf[0])<<8 | int64(buf[1])
	deltaGlyphID := font.GlyphID(buf[2])<<8 | font.GlyphID(buf[3])
	cov, err := coverage.ReadTable(p, subtablePos+coverageOffset)
	if err != nil {
		return nil, err
	}
	res := &Gsub1_1{
		Cov:   cov,
		Delta: deltaGlyphID,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *Gsub1_1) Apply(_ KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	gid := seq[i].Gid
	if _, ok := l.Cov[gid]; !ok {
		return seq, -1, nil
	}
	seq[i].Gid = gid + l.Delta
	return seq, i + 1, nil
}

// EncodeLen implements the Subtable interface.
func (l *Gsub1_1) EncodeLen() int {
	return 6 + l.Cov.EncodeLen()
}

// Encode implements the Subtable interface.
func (l *Gsub1_1) Encode() []byte {
	buf := make([]byte, 6+l.Cov.EncodeLen())
	// buf[0] = 0
	buf[1] = 1
	// buf[2] = 0
	buf[3] = 6
	buf[4] = byte(l.Delta >> 8)
	buf[5] = byte(l.Delta)
	copy(buf[6:], l.Cov.Encode())
	return buf
}

// Gsub1_2 is a Single Substitution GSUB subtable (type 1, format 2).
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#12-single-substitution-format-2
type Gsub1_2 struct {
	Cov                coverage.Table
	SubstituteGlyphIDs []font.GlyphID
}

func readGsub1_2(p *parser.Parser, subtablePos int64) (Subtable, error) {
	coverageOffset, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}
	substituteGlyphIDs, err := p.ReadGIDSlice()
	if err != nil {
		return nil, err
	}

	cov, err := coverage.ReadTable(p, subtablePos+int64(coverageOffset))
	if err != nil {
		return nil, err
	}

	if len(cov) != len(substituteGlyphIDs) {
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/gtab",
			Reason:    "malformed format 1.2 GSUB subtable",
		}
	}

	res := &Gsub1_2{
		Cov:                cov,
		SubstituteGlyphIDs: substituteGlyphIDs,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *Gsub1_2) Apply(_ KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	gid := seq[i].Gid
	if idx, ok := l.Cov[gid]; ok {
		seq[i].Gid = l.SubstituteGlyphIDs[idx]
		return seq, i + 1, nil
	}
	return seq, -1, nil
}

// EncodeLen implements the Subtable interface.
func (l *Gsub1_2) EncodeLen() int {
	return 6 + 2*len(l.SubstituteGlyphIDs) + l.Cov.EncodeLen()
}

// Encode implements the Subtable interface.
func (l *Gsub1_2) Encode() []byte {
	n := len(l.SubstituteGlyphIDs)
	covOffs := 6 + 2*n

	buf := make([]byte, covOffs+l.Cov.EncodeLen())
	// buf[0] = 0
	buf[1] = 2
	buf[2] = byte(covOffs >> 8)
	buf[3] = byte(covOffs)
	buf[4] = byte(n >> 8)
	buf[5] = byte(n)
	for i := 0; i < n; i++ {
		buf[6+2*i] = byte(l.SubstituteGlyphIDs[i] >> 8)
		buf[6+2*i+1] = byte(l.SubstituteGlyphIDs[i])
	}
	copy(buf[covOffs:], l.Cov.Encode())
	return buf
}

// Gsub2_1 is a Multiple Substitution GSUB subtable (type 2, format 1).
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#21-multiple-substitution-format-1
type Gsub2_1 struct {
	Cov  coverage.Table
	Repl [][]font.GlyphID // individual sequences must have non-zero length
}

func readGsub2_1(p *parser.Parser, subtablePos int64) (Subtable, error) {
	coverageOffset, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}
	sequenceOffsets, err := p.ReadUInt16Slice()
	if err != nil {
		return nil, err
	}
	sequenceCount := len(sequenceOffsets)

	repl := make([][]font.GlyphID, sequenceCount)
	for i := 0; i < sequenceCount; i++ {
		err := p.SeekPos(subtablePos + int64(sequenceOffsets[i]))
		if err != nil {
			return nil, err
		}
		repl[i], err = p.ReadGIDSlice()
		if err != nil {
			return nil, err
		}
	}

	cov, err := coverage.ReadTable(p, subtablePos+int64(coverageOffset))
	if err != nil {
		return nil, err
	}

	if len(cov) != sequenceCount {
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/gtab",
			Reason:    "malformed format 2.1 GSUB subtable",
		}
	}

	res := &Gsub2_1{
		Cov:  cov,
		Repl: repl,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *Gsub2_1) Apply(_ KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	gid := seq[i].Gid
	idx, ok := l.Cov[gid]
	if !ok {
		return seq, -1, nil
	}

	repl := l.Repl[idx]
	k := len(repl)

	res := make([]font.Glyph, len(seq)-1+k)
	copy(res, seq[:i])
	for j := 0; j < k; j++ {
		res[i+j].Gid = repl[j]
	}
	copy(res[i+k:], seq[i+1:])

	if k > 0 {
		res[i].Text = seq[i].Text
	}

	return res, i + k, nil
}

// EncodeLen implements the Subtable interface.
func (l *Gsub2_1) EncodeLen() int {
	total := 6 + 2*len(l.Repl)
	for _, repl := range l.Repl {
		total += 2 + 2*len(repl)
	}
	total += l.Cov.EncodeLen()
	return total
}

// Encode implements the Subtable interface.
func (l *Gsub2_1) Encode() []byte {
	sequenceCount := len(l.Repl)
	covOffs := 6 + 2*sequenceCount

	sequenceOffsets := make([]uint16, sequenceCount)
	for i, repl := range l.Repl {
		sequenceOffsets[i] = uint16(covOffs)
		covOffs += 2 + 2*len(repl)
	}

	buf := make([]byte, covOffs+l.Cov.EncodeLen())
	// buf[0] = 0
	buf[1] = 1
	buf[2] = byte(covOffs >> 8)
	buf[3] = byte(covOffs)
	buf[4] = byte(len(l.Repl) >> 8)
	buf[5] = byte(len(l.Repl))
	pos := 6
	for i := range l.Repl {
		buf[pos] = byte(sequenceOffsets[i] >> 8)
		buf[pos+1] = byte(sequenceOffsets[i])
		pos += 2
	}
	for _, repl := range l.Repl {
		buf[pos] = byte(len(repl) >> 8)
		buf[pos+1] = byte(len(repl))
		pos += 2
		for _, gid := range repl {
			buf[pos] = byte(gid >> 8)
			buf[pos+1] = byte(gid)
			pos += 2
		}
	}
	copy(buf[covOffs:], l.Cov.Encode())
	return buf
}

// Gsub3_1 is an Alternate Substitution GSUB subtable (type 3, format 1).
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#31-alternate-substitution-format-1
type Gsub3_1 struct {
	Cov coverage.Table
	Alt [][]font.GlyphID
}

func readGsub3_1(p *parser.Parser, subtablePos int64) (Subtable, error) {
	coverageOffset, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}
	alternateSetOffsets, err := p.ReadUInt16Slice()
	if err != nil {
		return nil, err
	}
	alternateSetCount := len(alternateSetOffsets)

	alt := make([][]font.GlyphID, alternateSetCount)
	for i := 0; i < alternateSetCount; i++ {
		err := p.SeekPos(subtablePos + int64(alternateSetOffsets[i]))
		if err != nil {
			return nil, err
		}
		glyphCount, err := p.ReadUInt16()
		if err != nil {
			return nil, err
		}
		alt[i] = make([]font.GlyphID, glyphCount)
		for j := 0; j < int(glyphCount); j++ {
			gid, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			}
			alt[i][j] = font.GlyphID(gid)
		}
	}

	cov, err := coverage.ReadTable(p, subtablePos+int64(coverageOffset))
	if err != nil {
		return nil, err
	}

	if len(cov) != alternateSetCount {
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/gtab",
			Reason:    "malformed format 3.1 GSUB subtable",
		}
	}

	res := &Gsub3_1{
		Cov: cov,
		Alt: alt,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *Gsub3_1) Apply(_ KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	idx, ok := l.Cov[seq[i].Gid]
	if !ok {
		return seq, -1, nil
	}
	if len(l.Alt[idx]) > 0 {
		// TODO(voss): implement a mechanism to select alternate glyphs.
		seq[i].Gid = l.Alt[idx][0]
	}
	return seq, i + 1, nil
}

// EncodeLen implements the Subtable interface.
func (l *Gsub3_1) EncodeLen() int {
	total := 6 + 2*len(l.Alt)
	for _, repl := range l.Alt {
		total += 2 + 2*len(repl)
	}
	total += l.Cov.EncodeLen()
	return total
}

// Encode implements the Subtable interface.
func (l *Gsub3_1) Encode() []byte {
	alternateSetCount := len(l.Alt)
	covOffs := 6 + 2*alternateSetCount

	alternateSetOffsets := make([]uint16, alternateSetCount)
	for i, repl := range l.Alt {
		alternateSetOffsets[i] = uint16(covOffs)
		covOffs += 2 + 2*len(repl)
	}

	buf := make([]byte, covOffs+l.Cov.EncodeLen())
	// buf[0] = 0
	buf[1] = 1
	buf[2] = byte(covOffs >> 8)
	buf[3] = byte(covOffs)
	buf[4] = byte(len(l.Alt) >> 8)
	buf[5] = byte(len(l.Alt))
	pos := 6
	for i := range l.Alt {
		buf[pos] = byte(alternateSetOffsets[i] >> 8)
		buf[pos+1] = byte(alternateSetOffsets[i])
		pos += 2
	}
	for _, alt := range l.Alt {
		buf[pos] = byte(len(alt) >> 8)
		buf[pos+1] = byte(len(alt))
		pos += 2
		for _, gid := range alt {
			buf[pos] = byte(gid >> 8)
			buf[pos+1] = byte(gid)
			pos += 2
		}
	}
	copy(buf[covOffs:], l.Cov.Encode())
	return buf
}

// Gsub4_1 is a Ligature Substitution GSUB subtable (type 4, format 1).
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#41-ligature-substitution-format-1
type Gsub4_1 struct {
	Cov  coverage.Table
	Repl [][]Ligature
}

// Ligature represents a substitution of a sequence of glyphs into a single glyph.
type Ligature struct {
	In  []font.GlyphID // excludes the first input glyph, since this is in Cov
	Out font.GlyphID
}

func readGsub4_1(p *parser.Parser, subtablePos int64) (Subtable, error) {
	coverageOffset, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}
	ligatureSetOffsets, err := p.ReadUInt16Slice()
	if err != nil {
		return nil, err
	}

	repl := make([][]Ligature, len(ligatureSetOffsets))
	for i, ligatureSetOffset := range ligatureSetOffsets {
		ligatureSetPos := subtablePos + int64(ligatureSetOffset)
		err := p.SeekPos(ligatureSetPos)
		if err != nil {
			return nil, err
		}
		ligatureOffsets, err := p.ReadUInt16Slice()
		if err != nil {
			return nil, err
		}

		repl[i] = make([]Ligature, len(ligatureOffsets))
		for j, ligatureOffset := range ligatureOffsets {
			err = p.SeekPos(ligatureSetPos + int64(ligatureOffset))
			if err != nil {
				return nil, err
			}
			ligatureGlyph, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			}
			componentCount, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			}
			componentGlyphIDs := make([]font.GlyphID, componentCount-1)
			for k := range componentGlyphIDs {
				gid, err := p.ReadUInt16()
				if err != nil {
					return nil, err
				}
				componentGlyphIDs[k] = font.GlyphID(gid)
			}

			repl[i][j].In = componentGlyphIDs
			repl[i][j].Out = font.GlyphID(ligatureGlyph)
		}
	}

	cov, err := coverage.ReadTable(p, subtablePos+int64(coverageOffset))
	if err != nil {
		return nil, err
	}

	if len(cov) != len(repl) {
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/gsub",
			Reason:    "malformed format 4.1 GSUB subtable",
		}
	}

	return &Gsub4_1{
		Cov:  cov,
		Repl: repl,
	}, nil
}

// Apply implements the Subtable interface.
func (l *Gsub4_1) Apply(keep KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	ligSetIdx, ok := l.Cov[seq[i].Gid]
	if !ok {
		return seq, -1, nil
	}
	ligSet := l.Repl[ligSetIdx]

ligLoop:
	for j := range ligSet {
		lig := &ligSet[j]
		p := i
		for _, gid := range lig.In {
			p++ // TODO(voss): skip ignored glyphs
			if p >= len(seq) {
				continue ligLoop
			}
			if seq[p].Gid != gid {
				continue ligLoop
			}
		}
		next := p + 1

		// gather the unicode representations
		var rr []rune
		for i := i; i < next; i++ {
			rr = append(rr, seq[i].Text...)
		}

		seq[i] = font.Glyph{
			Gid:  lig.Out,
			Text: rr,
		}
		seq = append(seq[:i+1], seq[next:]...)
		return seq, i + 1, nil
	}

	return seq, -1, nil
}

// EncodeLen implements the Subtable interface.
func (l *Gsub4_1) EncodeLen() int {
	total := 6 + 2*len(l.Repl)
	for _, repl := range l.Repl {
		total += 2 + 2*len(repl)
		for _, lig := range repl {
			total += 4 + 2*len(lig.In)
		}
	}
	total += l.Cov.EncodeLen()
	return total
}

// Encode implements the Subtable interface.
func (l *Gsub4_1) Encode() []byte {
	ligatureSetCount := len(l.Repl)
	total := 6 + 2*ligatureSetCount
	ligatureSetOffsets := make([]uint16, ligatureSetCount)
	for i, repl := range l.Repl {
		ligatureSetOffsets[i] = uint16(total)
		total += 2 + 2*len(repl)
		for _, lig := range repl {
			total += 4 + 2*len(lig.In)
		}
	}
	coverageOffset := total
	total += l.Cov.EncodeLen()

	buf := make([]byte, 0, total)

	buf = append(buf,
		0, 1, // version
		byte(coverageOffset>>8), byte(coverageOffset),
		byte(ligatureSetCount>>8), byte(ligatureSetCount),
	)
	for _, offs := range ligatureSetOffsets {
		buf = append(buf, byte(offs>>8), byte(offs))
	}
	for _, repl := range l.Repl {
		ligatureCount := len(repl)
		buf = append(buf, byte(ligatureCount>>8), byte(ligatureCount))
		pos := 2 + 2*ligatureCount
		for _, lig := range repl {
			buf = append(buf, byte(pos>>8), byte(pos))
			pos += 4 + 2*len(lig.In)
		}
		for _, lig := range repl {
			componentCount := len(lig.In) + 1
			buf = append(buf,
				byte(lig.Out>>8), byte(lig.Out),
				byte(componentCount>>8), byte(componentCount),
			)
			for _, gid := range lig.In {
				buf = append(buf, byte(gid>>8), byte(gid))
			}
		}
	}
	buf = append(buf, l.Cov.Encode()...)

	return buf
}

// A Contextual Substitution subtable describes glyph substitutions in context
// that replace one or more glyphs within a certain pattern of glyphs,
// using nested lookups.
//
// GSUB 5.1 - Context Substitution Format 1: Simple Glyph Contexts
//     Sequence Context Format 1: simple glyph contexts
// GSUB 5.2 - Context Substitution Format 2: Class-based Glyph Contexts
//     Sequence Context Format 2: class-based glyph contexts
// GSUB 5.3 - Context Substitution Format 3: Coverage-based Glyph Contexts
//     Sequence Context Format 3: coverage-based glyph contexts

// A Chained Contexts Substitution subtable describes glyph substitutions in
// context with an ability to look back and/or look ahead in the sequence of
// glyphs.  It can replace one or more glyphs within a certain pattern of
// glyphs, using nested lookups.
//
// GSUB 6.1 - Chained Contexts Substitution Format 1: Simple Glyph Contexts
//     Chained Sequence Context Format 1: simple glyph contexts
// GSUB 6.2 - Chained Contexts Substitution Format 2: Class-based Glyph Contexts
//     Chained Sequence Context Format 2: class-based glyph contexts
// GSUB 6.3 - Chained Contexts Substitution Format 3: Coverage-based Glyph Contexts
//     Chained Sequence Context Format 3: coverage-based glyph contexts
