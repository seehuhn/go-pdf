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
	"sort"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt/opentype/anchor"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/markarray"
)

// readGposSubtable reads a GPOS subtable.
// This function can be used as the SubtableReader argument to readLookupList().
func readGposSubtable(p *parser.Parser, pos int64, meta *LookupMetaInfo) (Subtable, error) {
	err := p.SeekPos(pos)
	if err != nil {
		return nil, err
	}

	format, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}

	reader, ok := gposReaders[10*meta.LookupType+format]
	if !ok {
		fmt.Println("GPOS", meta.LookupType, format)
		return notImplementedGposSubtable{meta.LookupType, format}, nil
	}
	return reader(p, pos)
}

var gposReaders = map[uint16]func(p *parser.Parser, pos int64) (Subtable, error){
	1_1: readGpos1_1,
	1_2: readGpos1_2,
	2_1: readGpos2_1,
	2_2: readGpos2_2,
	4_1: readGpos4_1,
	6_1: readGpos6_1,
	7_1: readSeqContext1,
	7_2: readSeqContext2,
	7_3: readSeqContext3,
	8_1: readChainedSeqContext1,
	8_2: readChainedSeqContext2,
	8_3: readChainedSeqContext3,
	9_1: readExtensionSubtable,
}

type notImplementedGposSubtable struct {
	lookupType, lookupFormat uint16
}

func (st notImplementedGposSubtable) Apply(_ KeepGlyphFn, _ []font.Glyph, _, _ int) *Match {
	return nil
}

func (st notImplementedGposSubtable) EncodeLen() int {
	msg := fmt.Sprintf("GPOS lookup type %d, format %d not implemented",
		st.lookupType, st.lookupFormat)
	panic(msg)
}

func (st notImplementedGposSubtable) Encode() []byte {
	msg := fmt.Sprintf("GPOS lookup type %d, format %d not implemented",
		st.lookupType, st.lookupFormat)
	panic(msg)
}

// Gpos1_1 is a Single Adjustment Positioning Subtable (GPOS type 1, format 1).
// If specifies a single adjustment to be applied to all glyphs in the
// coverage table.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#single-adjustment-positioning-format-1-single-positioning-value
type Gpos1_1 struct {
	Cov    coverage.Table
	Adjust *GposValueRecord
}

func readGpos1_1(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(4)
	if err != nil {
		return nil, err
	}
	coverageOffset := int64(buf[0])<<8 | int64(buf[1])
	valueFormat := uint16(buf[2])<<8 | uint16(buf[3])
	valueRecord, err := readValueRecord(p, valueFormat)
	if err != nil {
		return nil, err
	}
	cov, err := coverage.Read(p, subtablePos+coverageOffset)
	if err != nil {
		return nil, err
	}
	res := &Gpos1_1{
		Cov:    cov,
		Adjust: valueRecord,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *Gpos1_1) Apply(keep KeepGlyphFn, seq []font.Glyph, a, b int) *Match {
	g := seq[a]
	if !keep(g.Gid) {
		return nil
	}
	_, ok := l.Cov[g.Gid]
	if !ok {
		return nil
	}
	l.Adjust.Apply(&g)
	return &Match{
		InputPos: []int{a},
		Replace:  []font.Glyph{g},
		Next:     a + 1,
	}
}

// EncodeLen implements the Subtable interface.
func (l *Gpos1_1) EncodeLen() int {
	format := l.Adjust.getFormat()
	return 6 + l.Adjust.encodeLen(format) + l.Cov.EncodeLen()
}

// Encode implements the Subtable interface.
func (l *Gpos1_1) Encode() []byte {
	format := l.Adjust.getFormat()
	vrLen := l.Adjust.encodeLen(format)
	coverageOffs := 6 + vrLen
	total := coverageOffs + l.Cov.EncodeLen()
	buf := make([]byte, 0, total)
	buf = append(buf,
		0, 1, // format
		byte(coverageOffs>>8), byte(coverageOffs),
		byte(format>>8), byte(format),
	)
	buf = append(buf, l.Adjust.encode(format)...)
	buf = append(buf, l.Cov.Encode()...)
	return buf
}

// Gpos1_2 is a Single Adjustment Positioning Subtable (GPOS type 1, format 2)
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#single-adjustment-positioning-format-2-array-of-positioning-values
type Gpos1_2 struct {
	Cov    coverage.Table
	Adjust []*GposValueRecord // indexed by coverage index
}

func readGpos1_2(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(6)
	if err != nil {
		return nil, err
	}
	coverageOffset := int64(buf[0])<<8 | int64(buf[1])
	valueFormat := uint16(buf[2])<<8 | uint16(buf[3])
	valueCount := int(buf[4])<<8 | int(buf[5])
	valueRecords := make([]*GposValueRecord, valueCount)
	for i := range valueRecords {
		valueRecords[i], err = readValueRecord(p, valueFormat)
		if err != nil {
			return nil, err
		}
	}
	cov, err := coverage.Read(p, subtablePos+coverageOffset)
	if err != nil {
		return nil, err
	}

	if len(valueRecords) > len(cov) {
		valueRecords = valueRecords[:len(cov)]
	} else if len(valueRecords) < len(cov) {
		cov.Prune(len(valueRecords))
	}

	res := &Gpos1_2{
		Cov:    cov,
		Adjust: valueRecords,
	}
	return res, nil
}

// Apply implements the Subtable interface.
func (l *Gpos1_2) Apply(keep KeepGlyphFn, seq []font.Glyph, a, b int) *Match {
	g := seq[a]
	if !keep(g.Gid) {
		return nil
	}
	idx, ok := l.Cov[g.Gid]
	if !ok {
		return nil
	}
	l.Adjust[idx].Apply(&g)
	return &Match{
		InputPos: []int{a},
		Replace:  []font.Glyph{g},
		Next:     a + 1,
	}
}

// EncodeLen implements the Subtable interface.
func (l *Gpos1_2) EncodeLen() int {
	var valueFormat uint16
	for _, adj := range l.Adjust {
		valueFormat |= adj.getFormat()
	}
	total := 8
	if len(l.Adjust) > 0 {
		total += l.Adjust[0].encodeLen(valueFormat) * len(l.Adjust)
	}
	total += l.Cov.EncodeLen()
	return total
}

// Encode implements the Subtable interface.
func (l *Gpos1_2) Encode() []byte {
	var valueFormat uint16
	for _, adj := range l.Adjust {
		valueFormat |= adj.getFormat()
	}
	valueCount := len(l.Adjust)
	total := 8
	if len(l.Adjust) > 0 {
		total += l.Adjust[0].encodeLen(valueFormat) * valueCount
	}
	coverageOffset := total
	total += l.Cov.EncodeLen()

	buf := make([]byte, 0, total)
	buf = append(buf,
		0, 2, // format
		byte(coverageOffset>>8), byte(coverageOffset),
		byte(valueFormat>>8), byte(valueFormat),
		byte(valueCount>>8), byte(valueCount),
	)
	for _, adj := range l.Adjust {
		buf = append(buf, adj.encode(valueFormat)...)
	}
	buf = append(buf, l.Cov.Encode()...)
	return buf
}

// Gpos2_1 is a Pair Adjustment Positioning Subtable (format 1)
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#pair-adjustment-positioning-format-1-adjustments-for-glyph-pairs
type Gpos2_1 struct {
	Cov    coverage.Table
	Adjust []map[font.GlyphID]*PairAdjust // TODO(voss): use one map with pairs as keys?
}

// PairAdjust represents information from a PairValueRecord table.
type PairAdjust struct {
	First, Second *GposValueRecord
}

// Apply implements the Subtable interface.
func (l *Gpos2_1) Apply(keep KeepGlyphFn, seq []font.Glyph, a, b int) *Match {
	if a+1 >= b {
		return nil
	}

	g1 := seq[a]
	idx, ok := l.Cov[g1.Gid]
	if !ok {
		return nil
	}
	ruleSet := l.Adjust[idx]
	if ruleSet == nil {
		return nil
	}

	g2 := seq[a+1]
	adj, ok := ruleSet[g2.Gid]
	if !ok {
		return nil
	}

	adj.First.Apply(&g1)
	if adj.Second == nil {
		return &Match{
			InputPos: []int{a},
			Replace:  []font.Glyph{g1},
			Next:     a + 1,
		}
	}
	adj.Second.Apply(&g2)
	return &Match{
		InputPos: []int{a, a + 1},
		Replace:  []font.Glyph{g1, g2},
		Next:     a + 2,
	}
}

func readGpos2_1(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(8)
	if err != nil {
		return nil, err
	}
	coverageOffset := int64(buf[0])<<8 | int64(buf[1])
	valueFormat1 := uint16(buf[2])<<8 | uint16(buf[3])
	valueFormat2 := uint16(buf[4])<<8 | uint16(buf[5])
	pairSetCount := int(buf[6])<<8 | int(buf[7])

	pairSetOffsets := make([]uint16, pairSetCount)
	for i := range pairSetOffsets {
		pairSetOffsets[i], err = p.ReadUint16()
		if err != nil {
			return nil, err
		}
	}

	cov, err := coverage.Read(p, subtablePos+coverageOffset)
	if err != nil {
		return nil, err
	}

	if len(pairSetOffsets) > len(cov) {
		pairSetOffsets = pairSetOffsets[:len(cov)]
	} else if len(pairSetOffsets) < len(cov) {
		cov.Prune(len(pairSetOffsets))
	}

	adjust := make([]map[font.GlyphID]*PairAdjust, len(pairSetOffsets))
	for i, offset := range pairSetOffsets {
		err = p.SeekPos(subtablePos + int64(offset))
		if err != nil {
			return nil, err
		}
		pairValueCount, err := p.ReadUint16()
		if err != nil {
			return nil, err
		}
		adj := make(map[font.GlyphID]*PairAdjust, pairValueCount)
		for j := 0; j < int(pairValueCount); j++ {
			secondGlyph, err := p.ReadUint16()
			if err != nil {
				return nil, err
			}
			first, err := readValueRecord(p, valueFormat1)
			if err != nil {
				return nil, err
			}
			second, err := readValueRecord(p, valueFormat2)
			if err != nil {
				return nil, err
			}
			adj[font.GlyphID(secondGlyph)] = &PairAdjust{
				First:  first,
				Second: second,
			}
		}
		adjust[i] = adj
	}

	res := &Gpos2_1{
		Cov:    cov,
		Adjust: adjust,
	}
	return res, nil
}

// EncodeLen implements the Subtable interface.
func (l *Gpos2_1) EncodeLen() int {
	total := 10 + 2*len(l.Adjust)
	total += l.Cov.EncodeLen()
	var valueFormat1, valueFormat2 uint16
	for _, adj := range l.Adjust {
		for _, v := range adj {
			valueFormat1 |= v.First.getFormat()
			valueFormat2 |= v.Second.getFormat()
		}
	}
	for _, adj := range l.Adjust {
		total += 2 + 2*len(adj)
		for _, v := range adj {
			total += v.First.encodeLen(valueFormat1)
			total += v.Second.encodeLen(valueFormat2)
		}
	}
	return total
}

// Encode implements the Subtable interface.
func (l *Gpos2_1) Encode() []byte {
	pairSetCount := len(l.Adjust)
	total := 10 + 2*pairSetCount
	coverageOffset := total
	total += l.Cov.EncodeLen()
	var valueFormat1, valueFormat2 uint16
	for _, adj := range l.Adjust {
		for _, v := range adj {
			valueFormat1 |= v.First.getFormat()
			valueFormat2 |= v.Second.getFormat()
		}
	}
	pairSetOffsets := make([]uint16, pairSetCount)
	for i, adj := range l.Adjust {
		pairSetOffsets[i] = uint16(total)
		total += 2 + 2*len(adj)
		for _, v := range adj {
			total += v.First.encodeLen(valueFormat1)
			total += v.Second.encodeLen(valueFormat2)
		}
	}

	buf := make([]byte, 0, total)
	buf = append(buf,
		0, 1, // format
		byte(coverageOffset>>8), byte(coverageOffset),
		byte(valueFormat1>>8), byte(valueFormat1),
		byte(valueFormat2>>8), byte(valueFormat2),
		byte(pairSetCount>>8), byte(pairSetCount),
	)
	for _, offset := range pairSetOffsets {
		buf = append(buf, byte(offset>>8), byte(offset))
	}

	buf = append(buf, l.Cov.Encode()...)

	for _, adj := range l.Adjust {
		pairValueCount := len(adj)
		buf = append(buf, byte(pairValueCount>>8), byte(pairValueCount))

		keys := maps.Keys(adj)
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		for _, secondGlyph := range keys {
			buf = append(buf, byte(secondGlyph>>8), byte(secondGlyph))
			buf = append(buf, adj[secondGlyph].First.encode(valueFormat1)...)
			buf = append(buf, adj[secondGlyph].Second.encode(valueFormat2)...)
		}
	}

	return buf
}

// Gpos2_2 is a Pair Adjustment Positioning Subtable (format 2)
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#pair-adjustment-positioning-format-2-class-pair-adjustment
type Gpos2_2 struct {
	Cov            coverage.Set
	Class1, Class2 classdef.Table
	Adjust         [][]*PairAdjust // indexed by class1 index, then class2 index
}

// Apply implements the Subtable interface.
func (l *Gpos2_2) Apply(keep KeepGlyphFn, seq []font.Glyph, a, b int) *Match {
	g1 := seq[a]
	_, ok := l.Cov[g1.Gid]
	if !ok {
		return nil
	}

	p := a + 1
	for p < b && !keep(seq[p].Gid) {
		p++
	}
	if p >= b {
		return nil
	}
	g2 := seq[p]

	class1 := l.Class1[g1.Gid]
	if int(class1) >= len(l.Adjust) {
		return nil
	}
	row := l.Adjust[class1]
	class2 := l.Class2[g2.Gid]
	if int(class2) >= len(row) {
		return nil
	}
	adj := row[class2]

	adj.First.Apply(&g1)
	if adj.Second == nil {
		return &Match{
			InputPos: []int{a},
			Replace:  []font.Glyph{g1},
			Next:     a + 1,
		}
	}
	adj.Second.Apply(&g2)
	return &Match{
		InputPos: []int{a, a + 1},
		Replace:  []font.Glyph{g1, g2},
		Next:     a + 2,
	}
}

func readGpos2_2(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(14)
	if err != nil {
		return nil, err
	}
	coverageOffset := int64(buf[0])<<8 | int64(buf[1])
	valueFormat1 := uint16(buf[2])<<8 | uint16(buf[3])
	valueFormat2 := uint16(buf[4])<<8 | uint16(buf[5])
	classDef1Offset := int64(buf[6])<<8 | int64(buf[7])
	classDef2Offset := int64(buf[8])<<8 | int64(buf[9])
	class1Count := uint16(buf[10])<<8 | uint16(buf[11])
	class2Count := uint16(buf[12])<<8 | uint16(buf[13])

	numRecords := int(class1Count) * int(class2Count)
	if numRecords >= 65536 {
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/opentype/gtab",
			Reason:    "GPOS2.1 table too large",
		}
	}
	records := make([]*PairAdjust, numRecords)
	for i := 0; i < numRecords; i++ {
		first, err := readValueRecord(p, valueFormat1)
		if err != nil {
			return nil, err
		}
		second, err := readValueRecord(p, valueFormat2)
		if err != nil {
			return nil, err
		}
		records[i] = &PairAdjust{
			First:  first,
			Second: second,
		}
	}

	cov, err := coverage.ReadSet(p, subtablePos+coverageOffset)
	if err != nil {
		return nil, err
	}

	classDef1, err := classdef.Read(p, subtablePos+classDef1Offset)
	if err != nil {
		return nil, err
	}
	classDef2, err := classdef.Read(p, subtablePos+classDef2Offset)
	if err != nil {
		return nil, err
	}

	adjust := make([][]*PairAdjust, class1Count)
	for i := 0; i < int(class1Count); i++ {
		adjust[i] = records[i*int(class2Count) : (i+1)*int(class2Count)]
	}

	return &Gpos2_2{
		Cov:    cov,
		Class1: classDef1,
		Class2: classDef2,
		Adjust: adjust,
	}, nil
}

// EncodeLen implements the Subtable interface.
func (l *Gpos2_2) EncodeLen() int {
	var valueFormat1, valueFormat2 uint16
	for _, adj := range l.Adjust {
		for _, v := range adj {
			valueFormat1 |= v.First.getFormat()
			valueFormat2 |= v.Second.getFormat()
		}
	}
	var vr *GposValueRecord
	recLen := vr.encodeLen(valueFormat1) + vr.encodeLen(valueFormat2)

	class1Count := len(l.Adjust)
	var class2Count int
	if class1Count > 0 {
		class2Count = len(l.Adjust[0])
	}

	total := 16
	total += class1Count * class2Count * recLen
	total += l.Cov.ToTable().EncodeLen()
	total += l.Class1.AppendLen()
	total += l.Class2.AppendLen()
	return total
}

// Encode implements the Subtable interface.
func (l *Gpos2_2) Encode() []byte {
	var valueFormat1, valueFormat2 uint16
	for _, adj := range l.Adjust {
		for _, v := range adj {
			valueFormat1 |= v.First.getFormat()
			valueFormat2 |= v.Second.getFormat()
		}
	}
	var vr *GposValueRecord
	recLen := vr.encodeLen(valueFormat1) + vr.encodeLen(valueFormat2)

	class1Count := len(l.Adjust)
	var class2Count int
	if class1Count > 0 {
		class2Count = len(l.Adjust[0])
	}

	total := 16
	total += class1Count * class2Count * recLen
	coverageOffset := total
	total += l.Cov.ToTable().EncodeLen()
	classDef1Offset := total
	total += l.Class1.AppendLen()
	classDef2Offset := total
	total += l.Class2.AppendLen()

	res := make([]byte, 0, total)
	res = append(res,
		0, 2, // posFormat
		byte(coverageOffset>>8), byte(coverageOffset),
		byte(valueFormat1>>8), byte(valueFormat1),
		byte(valueFormat2>>8), byte(valueFormat2),
		byte(classDef1Offset>>8), byte(classDef1Offset),
		byte(classDef2Offset>>8), byte(classDef2Offset),
		byte(class1Count>>8), byte(class1Count),
		byte(class2Count>>8), byte(class2Count),
	)
	for _, row := range l.Adjust {
		for _, adj := range row {
			res = append(res, adj.First.encode(valueFormat1)...)
			res = append(res, adj.Second.encode(valueFormat2)...)
		}
	}
	if len(res) != coverageOffset { // TODO(voss): remove
		panic("internal error")
	}
	res = append(res, l.Cov.ToTable().Encode()...)
	if len(res) != classDef1Offset { // TODO(voss): remove
		panic("internal error")
	}
	res = l.Class1.Append(res)
	if len(res) != classDef2Offset { // TODO(voss): remove
		panic("internal error")
	}
	res = l.Class2.Append(res)
	if len(res) != total { // TODO(voss): remove
		panic("internal error")
	}

	return res
}

// Gpos4_1 is a Mark-to-Base Attachment Positioning Subtable (format 1)
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#mark-to-base-attachment-positioning-format-1-mark-to-base-attachment-point
type Gpos4_1 struct {
	MarkCov   coverage.Table
	BaseCov   coverage.Table
	MarkArray []markarray.Record // indexed by mark coverage index
	BaseArray [][]anchor.Table   // indexed by base coverage index, then by mark class
}

// Apply implements the Subtable interface.
func (l *Gpos4_1) Apply(keep KeepGlyphFn, seq []font.Glyph, a, b int) *Match {
	// TODO(voss): does this apply to the base or the mark?
	if !keep(seq[a].Gid) {
		return nil
	}
	markIdx, ok := l.MarkCov[seq[a].Gid]
	if !ok {
		return nil
	}
	markRecord := l.MarkArray[markIdx]

	if a == 0 {
		return nil
	}
	p := a - 1
	var baseIdx int
	for p >= 0 {
		baseIdx, ok = l.BaseCov[seq[p].Gid]
		if ok {
			break
		}
		p--
	}
	if p < 0 {
		return nil
	}
	baseRecord := l.BaseArray[baseIdx][markRecord.Class]
	if baseRecord.IsEmpty() {
		// TODO(voss): verify that this is what others do, too.
		return nil
	}

	dx := baseRecord.X - markRecord.X
	dy := baseRecord.Y - markRecord.Y
	for i := p; i < a; i++ {
		dx -= seq[i].Advance
	}
	g := seq[a]
	g.XOffset = dx
	g.YOffset = dy
	_ = dy
	return &Match{
		InputPos: []int{a},
		Replace:  []font.Glyph{g},
		Next:     a + 1,
	}
}

func readGpos4_1(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(10)
	if err != nil {
		return nil, err
	}
	markCoverageOffset := int64(buf[0])<<8 | int64(buf[1])
	baseCoverageOffset := int64(buf[2])<<8 | int64(buf[3])
	markClassCount := int(buf[4])<<8 | int(buf[5])
	markArrayOffset := int64(buf[6])<<8 | int64(buf[7])
	baseArrayOffset := int64(buf[8])<<8 | int64(buf[9])

	markCov, err := coverage.Read(p, subtablePos+markCoverageOffset)
	if err != nil {
		return nil, err
	}
	baseCov, err := coverage.Read(p, subtablePos+baseCoverageOffset)
	if err != nil {
		return nil, err
	}

	markArray, err := markarray.Read(p, subtablePos+markArrayOffset, len(markCov))
	if err != nil {
		return nil, err
	}
	if len(markCov) > len(markArray) {
		markCov.Prune(len(markArray))
	} else {
		markArray = markArray[:len(markCov)]
	}

	baseArrayPos := subtablePos + baseArrayOffset
	err = p.SeekPos(baseArrayPos)
	if err != nil {
		return nil, err
	}

	baseCount, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	if int(baseCount) > len(baseCov) {
		baseCount = uint16(len(baseCov))
	} else {
		baseCov.Prune(int(baseCount))
	}
	numOffsets := uint(baseCount) * uint(markClassCount)
	if numOffsets > (65536-6-2)/2 {
		// Offsets are 16-bit from baseArrayPos, and there must still be
		// space for at least one achor table.
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/opentype/gtab",
			Reason:    "GPOS4.1 table too large",
		}
	}
	offsets := make([]uint16, numOffsets)
	for i := range offsets {
		offsets[i], err = p.ReadUint16()
		if err != nil {
			return nil, err
		}
	}

	baseArray := make([][]anchor.Table, baseCount)
	for i := range baseArray {
		row := make([]anchor.Table, markClassCount)
		for j := range row {
			if offsets[j] == 0 {
				continue
			}
			row[j], err = anchor.Read(p, baseArrayPos+int64(offsets[j]))
			if err != nil {
				return nil, err
			}
		}
		baseArray[i] = row
		offsets = offsets[markClassCount:]
	}

	return &Gpos4_1{
		MarkCov:   markCov,
		BaseCov:   baseCov,
		MarkArray: markArray,
		BaseArray: baseArray,
	}, nil
}

func (l *Gpos4_1) countMarkClasses() int {
	if len(l.BaseArray) > 0 {
		return len(l.BaseArray[0])
	}

	var maxClass uint16
	for _, rec := range l.MarkArray {
		if rec.Class > maxClass {
			maxClass = rec.Class
		}
	}
	return int(maxClass) + 1
}

// EncodeLen implements the Subtable interface.
func (l *Gpos4_1) EncodeLen() int {
	total := 12
	total += l.MarkCov.EncodeLen()
	total += l.BaseCov.EncodeLen()
	total += 2 + (4+6)*len(l.MarkArray)

	total += 2
	for _, row := range l.BaseArray {
		for _, rec := range row {
			total += 2
			if !rec.IsEmpty() {
				total += 6
			}
		}
	}
	return total
}

// Encode implements the Subtable interface.
func (l *Gpos4_1) Encode() []byte {
	markCount := len(l.MarkArray)
	markClassCount := l.countMarkClasses()
	baseCount := len(l.BaseArray)

	total := 12
	markCoverageOffset := total
	total += l.MarkCov.EncodeLen()
	baseCoverageOffset := total
	total += l.BaseCov.EncodeLen()
	markArrayOffset := total
	total += 2 + (4+6)*markCount
	baseArrayOffset := total
	total += 2
	for _, row := range l.BaseArray {
		for _, rec := range row {
			total += 2
			if !rec.IsEmpty() {
				total += 6
			}
		}
	}
	res := make([]byte, 0, total)

	res = append(res,
		0, 1, // posFormat
		byte(markCoverageOffset>>8), byte(markCoverageOffset),
		byte(baseCoverageOffset>>8), byte(baseCoverageOffset),
		byte(markClassCount>>8), byte(markClassCount),
		byte(markArrayOffset>>8), byte(markArrayOffset),
		byte(baseArrayOffset>>8), byte(baseArrayOffset),
	)

	res = append(res, l.MarkCov.Encode()...)
	res = append(res, l.BaseCov.Encode()...)

	if len(res) != markArrayOffset { // TODO(voss): remove
		panic("internal error")
	}
	res = append(res,
		byte(markCount>>8), byte(markCount),
	)
	offs := 2 + 4*markCount
	for _, rec := range l.MarkArray {
		res = append(res,
			byte(rec.Class>>8), byte(rec.Class),
			byte(offs>>8), byte(offs),
		)
		offs += 6
	}
	for _, rec := range l.MarkArray {
		res = append(res,
			0, 1, // anchorFormat
			byte(rec.X>>8), byte(rec.X),
			byte(rec.Y>>8), byte(rec.Y),
		)
	}

	if len(res) != baseArrayOffset { // TODO(voss): remove
		panic("internal error")
	}
	res = append(res,
		byte(baseCount>>8), byte(baseCount),
	)
	offs = 2 + 2*baseCount*markClassCount
	for _, row := range l.BaseArray {
		for _, rec := range row {
			if rec.IsEmpty() {
				res = append(res, 0, 0)
				continue
			}
			res = append(res,
				byte(offs>>8), byte(offs),
			)
			offs += 6
		}
	}
	for _, row := range l.BaseArray {
		for _, rec := range row {
			if rec.IsEmpty() {
				continue
			}
			res = append(res,
				0, 1, // anchorFormat
				byte(rec.X>>8), byte(rec.X),
				byte(rec.Y>>8), byte(rec.Y),
			)
		}
	}

	if len(res) != total { // TODO(voss): remove
		panic("internal error")
	}

	return res
}

// Gpos6_1 is a Mark-to-Mark Attachment Positioning Subtable (format 1)
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#lookup-type-6-mark-to-mark-attachment-positioning-subtable
type Gpos6_1 struct {
	Mark1Cov   coverage.Table
	Mark2Cov   coverage.Table
	Mark1Array []markarray.Record // indexed by mark1 coverage index
	Mark2Array [][]anchor.Table   // indexed by mark2 coverage index, then by mark class
}

// Apply implements the Subtable interface.
func (l *Gpos6_1) Apply(keep KeepGlyphFn, seq []font.Glyph, a, b int) *Match {
	// TODO(voss): does this apply to mark1 or mark2?
	if !keep(seq[a].Gid) {
		return nil
	}
	mark1Idx, ok := l.Mark1Cov[seq[a].Gid]
	if !ok {
		return nil
	}
	mark1Record := l.Mark1Array[mark1Idx]

	if a == 0 {
		return nil
	}
	p := a - 1
	var mark2Idx int
	for p >= 0 {
		mark2Idx, ok = l.Mark2Cov[seq[p].Gid]
		if ok {
			break
		}
		p--
	}
	if p < 0 {
		return nil
	}
	mark2Record := l.Mark2Array[mark2Idx][mark1Record.Class]

	dx := mark2Record.X - mark1Record.X
	dy := mark2Record.Y - mark1Record.Y
	for i := p; i < a; i++ {
		dx -= seq[i].Advance
	}
	g := seq[a]
	g.XOffset = dx
	g.YOffset = dy
	_ = dy
	return &Match{
		InputPos: []int{a},
		Replace:  []font.Glyph{g},
		Next:     a + 1,
	}
}

func readGpos6_1(p *parser.Parser, subtablePos int64) (Subtable, error) {
	buf, err := p.ReadBytes(10)
	if err != nil {
		return nil, err
	}
	mark1CoverageOffset := int64(buf[0])<<8 | int64(buf[1])
	mark2CoverageOffset := int64(buf[2])<<8 | int64(buf[3])
	markClassCount := int(buf[4])<<8 | int(buf[5])
	mark1ArrayOffset := int64(buf[6])<<8 | int64(buf[7])
	mark2ArrayOffset := int64(buf[8])<<8 | int64(buf[9])

	mark1Cov, err := coverage.Read(p, subtablePos+mark1CoverageOffset)
	if err != nil {
		return nil, err
	}
	mark2Cov, err := coverage.Read(p, subtablePos+mark2CoverageOffset)
	if err != nil {
		return nil, err
	}

	mark1Array, err := markarray.Read(p, subtablePos+mark1ArrayOffset, len(mark1Cov))
	if err != nil {
		return nil, err
	}
	if len(mark1Cov) > len(mark1Array) {
		mark1Cov.Prune(len(mark1Array))
	} else {
		mark1Array = mark1Array[:len(mark1Cov)]
	}

	mark2ArrayPos := subtablePos + mark2ArrayOffset
	err = p.SeekPos(mark2ArrayPos)
	if err != nil {
		return nil, err
	}

	mark2Count, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	if int(mark2Count) > len(mark2Cov) {
		mark2Count = uint16(len(mark2Cov))
	} else {
		mark2Cov.Prune(int(mark2Count))
	}
	numOffsets := uint(mark2Count) * uint(markClassCount)
	if numOffsets > (65536-6-2)/2 {
		// Offsets are 16-bit from mark2ArrayPos, and there must still be
		// space for at least one achor table.
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/opentype/gtab",
			Reason:    "GPOS6.1 table too large",
		}
	}
	offsets := make([]uint16, numOffsets)
	for i := range offsets {
		offsets[i], err = p.ReadUint16()
		if err != nil {
			return nil, err
		}
	}

	mark2Array := make([][]anchor.Table, mark2Count)
	for i := range mark2Array {
		row := make([]anchor.Table, markClassCount)
		for j := range row {
			row[j], err = anchor.Read(p, mark2ArrayPos+int64(offsets[j]))
			if err != nil {
				return nil, err
			}
		}
		mark2Array[i] = row
		offsets = offsets[markClassCount:]
	}

	return &Gpos6_1{
		Mark1Cov:   mark1Cov,
		Mark2Cov:   mark2Cov,
		Mark1Array: mark1Array,
		Mark2Array: mark2Array,
	}, nil
}

func (l *Gpos6_1) countMarkClasses() int {
	if len(l.Mark2Array) > 0 {
		return len(l.Mark2Array[0])
	}

	var maxClass uint16
	for _, rec := range l.Mark1Array {
		if rec.Class > maxClass {
			maxClass = rec.Class
		}
	}
	return int(maxClass) + 1
}

// EncodeLen implements the Subtable interface.
func (l *Gpos6_1) EncodeLen() int {
	total := 12
	total += l.Mark1Cov.EncodeLen()
	total += l.Mark2Cov.EncodeLen()
	total += 2 + (4+6)*len(l.Mark1Array)
	markClassCount := l.countMarkClasses()
	total += 2 + (2+6)*len(l.Mark2Array)*markClassCount
	return total
}

// Encode implements the Subtable interface.
func (l *Gpos6_1) Encode() []byte {
	mark1Count := len(l.Mark1Array)
	markClassCount := l.countMarkClasses()
	mark2Count := len(l.Mark2Array)

	total := 12
	mark1CoverageOffset := total
	total += l.Mark1Cov.EncodeLen()
	mark2CoverageOffset := total
	total += l.Mark2Cov.EncodeLen()
	mark1ArrayOffset := total
	total += 2 + (4+6)*mark1Count
	mark2ArrayOffset := total
	total += 2 + (2+6)*len(l.Mark2Array)*markClassCount
	res := make([]byte, 0, total)

	res = append(res,
		0, 1, // posFormat
		byte(mark1CoverageOffset>>8), byte(mark1CoverageOffset),
		byte(mark2CoverageOffset>>8), byte(mark2CoverageOffset),
		byte(markClassCount>>8), byte(markClassCount),
		byte(mark1ArrayOffset>>8), byte(mark1ArrayOffset),
		byte(mark2ArrayOffset>>8), byte(mark2ArrayOffset),
	)

	res = append(res, l.Mark1Cov.Encode()...)
	res = append(res, l.Mark2Cov.Encode()...)

	if len(res) != mark1ArrayOffset { // TODO(voss): remove
		panic("internal error")
	}
	res = append(res,
		byte(mark1Count>>8), byte(mark1Count),
	)
	offs := 2 + 4*mark1Count
	for _, rec := range l.Mark1Array {
		res = append(res,
			byte(rec.Class>>8), byte(rec.Class),
			byte(offs>>8), byte(offs),
		)
		offs += 6
	}
	for _, rec := range l.Mark1Array {
		res = append(res,
			0, 1, // anchorFormat
			byte(rec.X>>8), byte(rec.X),
			byte(rec.Y>>8), byte(rec.Y),
		)
	}

	if len(res) != mark2ArrayOffset { // TODO(voss): remove
		panic("internal error")
	}
	res = append(res,
		byte(mark2Count>>8), byte(mark2Count),
	)
	offs = 2 + 2*mark2Count*markClassCount
	for _, row := range l.Mark2Array {
		for range row {
			res = append(res,
				byte(offs>>8), byte(offs),
			)
			offs += 6
		}
	}
	for _, row := range l.Mark2Array {
		for _, rec := range row {
			res = append(res,
				0, 1, // anchorFormat
				byte(rec.X>>8), byte(rec.X),
				byte(rec.Y>>8), byte(rec.Y),
			)
		}
	}

	if len(res) != total { // TODO(voss): remove
		panic("internal error")
	}

	return res
}
