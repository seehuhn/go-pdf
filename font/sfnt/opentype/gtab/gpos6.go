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
	"seehuhn.de/go/pdf/font/sfnt/opentype/anchor"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/markarray"
)

// Gpos6_1 is a Mark-to-Mark Attachment Positioning Subtable (format 1)
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#lookup-type-6-mark-to-mark-attachment-positioning-subtable
type Gpos6_1 struct {
	Mark1Cov   coverage.Table
	Mark2Cov   coverage.Table
	Mark1Array []markarray.Record // indexed by mark1 coverage index
	Mark2Array [][]anchor.Table   // indexed by mark2 coverage index, then by mark class
}

// Apply implements the Subtable interface.
func (l *Gpos6_1) Apply(keep keepGlyphFn, seq []font.Glyph, a, b int) *Match {
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
	if mark2Record.IsEmpty() {
		// TODO(voss): verify that this is what others do, too.
		return nil
	}

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
			if offsets[j] == 0 {
				continue
			}
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

	total += 2
	for _, row := range l.Mark2Array {
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
	total += 2
	for _, row := range l.Mark2Array {
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
		byte(mark1CoverageOffset>>8), byte(mark1CoverageOffset),
		byte(mark2CoverageOffset>>8), byte(mark2CoverageOffset),
		byte(markClassCount>>8), byte(markClassCount),
		byte(mark1ArrayOffset>>8), byte(mark1ArrayOffset),
		byte(mark2ArrayOffset>>8), byte(mark2ArrayOffset),
	)

	res = append(res, l.Mark1Cov.Encode()...)
	res = append(res, l.Mark2Cov.Encode()...)

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
		res = rec.Append(res)
	}

	res = append(res,
		byte(mark2Count>>8), byte(mark2Count),
	)
	offs = 2 + 2*mark2Count*markClassCount
	for _, row := range l.Mark2Array {
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
	for _, row := range l.Mark2Array {
		for _, rec := range row {
			if rec.IsEmpty() {
				continue
			}
			res = rec.Append(res)
		}
	}

	return res
}
