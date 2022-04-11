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

// Gsub1_1 is a format 1.1 GSUB subtable.
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

// Apply implements the Subtable interface.
func (l *Gsub1_1) Apply(meta *LookupMetaInfo, seq []font.Glyph, i int) ([]font.Glyph, int) {
	gid := seq[i].Gid
	if _, ok := l.Cov[gid]; !ok {
		return seq, -1
	}
	seq[i].Gid = gid + l.Delta
	return seq, i + 1
}

// EncodeLen implements the Subtable interface.
func (l *Gsub1_1) EncodeLen(*LookupMetaInfo) int {
	return 6 + l.Cov.EncodeLen()
}

// Encode implements the Subtable interface.
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

// Gsub1_2 is a format 1.2 GSUB subtable.
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#12-single-substitution-format-2
type Gsub1_2 struct {
	Cov                coverage.Table
	SubstituteGlyphIDs []font.GlyphID
}

func readGsub1_2(p *parser.Parser, subtablePos int64) (*Gsub1_2, error) {
	buf, err := p.ReadBytes(4)
	if err != nil {
		return nil, err
	}
	coverageOffset := int64(buf[0])<<8 | int64(buf[1])
	glyphCount := int(buf[2])<<8 | int(buf[3])
	substituteGlyphIDs := make([]font.GlyphID, glyphCount)
	for i := 0; i < glyphCount; i++ {
		gid, err := p.ReadUInt16()
		if err != nil {
			return nil, err
		}
		substituteGlyphIDs[i] = font.GlyphID(gid)
	}

	cov, err := coverage.ReadTable(p, subtablePos+coverageOffset)
	if err != nil {
		return nil, err
	}

	if len(cov) != glyphCount {
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
func (l *Gsub1_2) Apply(meta *LookupMetaInfo, seq []font.Glyph, i int) ([]font.Glyph, int) {
	gid := seq[i].Gid
	if idx, ok := l.Cov[gid]; ok {
		seq[i].Gid = l.SubstituteGlyphIDs[idx]
		return seq, i + 1
	}
	return seq, -1
}

// EncodeLen implements the Subtable interface.
func (l *Gsub1_2) EncodeLen(*LookupMetaInfo) int {
	return 6 + 2*len(l.SubstituteGlyphIDs) + l.Cov.EncodeLen()
}

// Encode implements the Subtable interface.
func (l *Gsub1_2) Encode(*LookupMetaInfo) []byte {
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
