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
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
)

// Chained Contexts Substitution Format 3: Coverage-based Glyph Contexts
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#chseqctxt3
type chainedSeq3 struct {
	backtrack []coverage.Table
	input     []coverage.Table
	lookahead []coverage.Table
	actions   []seqLookupRecord
}

type seqLookupRecord struct {
	sequenceIndex   uint16
	lookupListIndex uint16
}

func readChained3(p *parser.Parser, subtablePos int64) (*chainedSeq3, error) {
	backtrackGlyphCount, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}
	backtrackCoverageOffsets := make([]uint16, backtrackGlyphCount)
	for i := 0; i < int(backtrackGlyphCount); i++ {
		backtrackCoverageOffsets[i], err = p.ReadUInt16()
		if err != nil {
			return nil, err
		}
	}

	inputGlyphCount, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}
	inputCoverageOffsets := make([]uint16, inputGlyphCount)
	for i := 0; i < int(inputGlyphCount); i++ {
		inputCoverageOffsets[i], err = p.ReadUInt16()
		if err != nil {
			return nil, err
		}
	}

	lookaheadGlyphCount, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}
	lookaheadCoverageOffsets := make([]uint16, lookaheadGlyphCount)
	for i := 0; i < int(lookaheadGlyphCount); i++ {
		lookaheadCoverageOffsets[i], err = p.ReadUInt16()
		if err != nil {
			return nil, err
		}
	}

	seqLookupCount, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}
	actions := make([]seqLookupRecord, seqLookupCount)
	for i := 0; i < int(seqLookupCount); i++ {
		buf, err := p.ReadBytes(4)
		if err != nil {
			return nil, err
		}
		actions[i].sequenceIndex = uint16(buf[0])<<8 + uint16(buf[1])
		actions[i].lookupListIndex = uint16(buf[2])<<8 + uint16(buf[3])
	}

	res := &chainedSeq3{
		actions: actions,
	}

	for _, offs := range backtrackCoverageOffsets {
		cover, err := coverage.ReadTable(p, subtablePos+int64(offs))
		if err != nil {
			return nil, err
		}
		res.backtrack = append(res.backtrack, cover)
	}
	for _, offs := range inputCoverageOffsets {
		cover, err := coverage.ReadTable(p, subtablePos+int64(offs))
		if err != nil {
			return nil, err
		}
		res.input = append(res.input, cover)
	}
	for _, offs := range lookaheadCoverageOffsets {
		cover, err := coverage.ReadTable(p, subtablePos+int64(offs))
		if err != nil {
			return nil, err
		}
		res.lookahead = append(res.lookahead, cover)
	}
	return res, nil
}
