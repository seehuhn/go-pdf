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
	default:
		msg := fmt.Sprintf("GSUB %d.%d\n", meta.LookupType, format)
		fmt.Print(msg)
		return notImplementedGsubSubtable(format), nil
	}
}

type notImplementedGsubSubtable uint16

func (st notImplementedGsubSubtable) Apply(meta *LookupMetaInfo, _ []font.Glyph, _ int) ([]font.Glyph, int) {
	msg := fmt.Sprintf("GSUB lookup type %d, format %d not implemented",
		meta.LookupType, st)
	panic(msg)
}

func (st notImplementedGsubSubtable) EncodeLen(meta *LookupMetaInfo) int {
	msg := fmt.Sprintf("GSUB lookup type %d, format %d not implemented",
		meta.LookupType, st)
	panic(msg)
}

func (st notImplementedGsubSubtable) Encode(meta *LookupMetaInfo) []byte {
	msg := fmt.Sprintf("GSUB lookup type %d, format %d not implemented",
		meta.LookupType, st)
	panic(msg)
}

// Gsub1_1 is a GSUB subtable for format 1.1.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#11-single-substitution-format-1
type Gsub1_1 struct {
	Cov   coverage.Table
	Delta font.GlyphID
}

func readGsub1_1(p *parser.Parser, subtablePos int64) (*Gsub1_1, error) {
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

func (l *Gsub1_1) Apply(meta *LookupMetaInfo, seq []font.Glyph, i int) ([]font.Glyph, int) {
	gid := seq[i].Gid
	if _, ok := l.Cov[gid]; !ok {
		return seq, -1
	}
	seq[i].Gid = gid + l.Delta
	return seq, i + 1
}

func (l *Gsub1_1) EncodeLen(*LookupMetaInfo) int {
	return 6 + l.Cov.EncodeLen()
}

func (l *Gsub1_1) Encode(*LookupMetaInfo) []byte {
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
