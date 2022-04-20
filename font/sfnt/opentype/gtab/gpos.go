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

// readGposSubtable reads a GPOS subtable.
// This function can be used as the SubtableReader argument to Read().
func readGposSubtable(p *parser.Parser, pos int64, meta *LookupMetaInfo) (Subtable, error) {
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
		return readGpos1_1(p, pos)
	case 7_1:
		return readSeqContext1(p, pos)
	case 7_2:
		return readSeqContext2(p, pos)
	default:
		fmt.Println("GPOS", meta.LookupType, format)
		return notImplementedGposSubtable{meta.LookupType, format}, nil
	}
}

type notImplementedGposSubtable struct {
	lookupType, lookupFormat uint16
}

func (st notImplementedGposSubtable) Apply(_ KeepGlyphFn, _ []font.Glyph, _ int) ([]font.Glyph, int, Nested) {
	msg := fmt.Sprintf("GPOS lookup type %d, format %d not implemented",
		st.lookupType, st.lookupFormat)
	panic(msg)
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

// Gpos1_1 is a Single Adjustment Positioning Subtable (GPOS type 1, format 1)
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#single-adjustment-positioning-format-1-single-positioning-value
type Gpos1_1 struct {
	Cov    coverage.Table
	Adjust *ValueRecord
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
	cov, err := coverage.ReadTable(p, subtablePos+coverageOffset)
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
func (l *Gpos1_1) Apply(_ KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	_, ok := l.Cov[seq[i].Gid]
	if !ok {
		return seq, -1, nil
	}
	l.Adjust.Apply(&seq[i])
	return seq, i + 1, nil
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
		0, 1,
		byte(coverageOffs>>8), byte(coverageOffs),
		byte(format>>8), byte(format),
	)
	buf = append(buf, l.Adjust.encode(format)...)
	buf = append(buf, l.Cov.Encode()...)
	return buf
}
