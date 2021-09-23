// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
)

// The most common GSUB features seen on my system:
//     5630 "liga"
//     4185 "frac"
//     3857 "aalt"
//     3746 "onum"
//     3434 "sups"
//     3010 "lnum"
//     2992 "pnum"
//     2989 "ccmp"
//     2976 "dnom"
//     2962 "numr"

// http://adobe-type-tools.github.io/afdko/OpenTypeFeatureFileSpecification.html#7
// TODO(voss): read this carefully

// ReadGsubTable reads the "GSUB" table of a font.
func (g *GTab) ReadGsubTable(extraFeatures ...string) (Lookups, error) {
	includeFeature := map[string]bool{
		"calt": true,
		"ccmp": true,
		"clig": true,
		"liga": true,
		"locl": true,
	}
	for _, feature := range extraFeatures {
		includeFeature[feature] = true
	}

	g.classDefCache = make(map[int64]ClassDef)
	g.coverageCache = make(map[int64]coverage)
	g.subtableReader = g.readGsubSubtable

	ll, err := g.selectLookups("GSUB", includeFeature)
	if err != nil {
		return nil, err
	}

	return g.readLookups(ll)
}

func (g *GTab) readGsubSubtable(s *parser.State, lookupType uint16, subtablePos int64) (LookupSubtable, error) {
	// TODO(voss): is this called more than once for the same subtablePos? -> use caching?
	s.A = subtablePos
	err := g.Exec(s,
		parser.CmdSeek,
		parser.CmdRead16, parser.TypeUInt, // format
	)
	if err != nil {
		return nil, err
	}
	format := s.A

	// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#table-organization
	switch 10*lookupType + uint16(format) {
	case 1_1: // Single Substitution Format 1
		return g.readGsub1_1(s, subtablePos)

	case 1_2: // Single Substitution Format 2
		return g.readGsub1_2(s, subtablePos)

	case 2_1: // Multiple Substitution Subtable
		return g.readGsub2_1(s, subtablePos)

	case 4_1: // Ligature Substitution Subtable
		return g.readGsub4_1(s, subtablePos)

	case 5_2: // Context Substitution: Class-based Glyph Contexts
		return g.readSeqContext2(s, subtablePos)

	case 6_1: // Chained Contexts Substitution: Simple Glyph Contexts
		return g.readChained1(s, subtablePos)

	case 6_2: // Chained Contexts Substitution: Class-based Glyph Contexts
		return g.readChained2(s, subtablePos)

	case 6_3: // Chained Contexts Substitution: Coverage-based Glyph Contexts
		return g.readChained3(s, subtablePos)

	case 7_1: // Extension Substitution Subtable Format 1
		err = g.Exec(s,
			parser.CmdRead16, parser.TypeUInt, // extensionLookupType
			parser.CmdStoreInto, 0,
			parser.CmdRead32, parser.TypeUInt, // extensionOffset
		)
		if err != nil {
			return nil, err
		}
		if s.R[0] == 7 {
			return nil, g.Error("invalid extension lookup")
		}
		return g.readGsubSubtable(s, uint16(s.R[0]), subtablePos+s.A)

	default:
		return &lookupNotImplemented{
			table:      "GSUB",
			lookupType: lookupType,
			format:     uint16(format),
		}, nil
	}
}

type gsub1_1 struct {
	cov   coverage
	delta uint16
}

func (g *GTab) readGsub1_1(s *parser.State, subtablePos int64) (*gsub1_1, error) {
	err := g.Exec(s,
		parser.CmdRead16, parser.TypeUInt, // coverageOffset
		parser.CmdStoreInto, 0,
		parser.CmdRead16, parser.TypeUInt, // deltaGlyphID (mod 65536)
	)
	if err != nil {
		return nil, err
	}
	delta := uint16(s.A)
	cov, err := g.readCoverageTable(subtablePos + s.R[0])
	if err != nil {
		return nil, err
	}
	res := &gsub1_1{
		cov:   cov,
		delta: delta,
	}
	return res, nil
}

func (l *gsub1_1) Apply(filter KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int) {
	gid := seq[i].Gid
	if _, ok := l.cov[gid]; !ok {
		return seq, -1
	}
	seq[i].Gid = font.GlyphID(uint16(gid) + l.delta)
	return seq, i + 1
}

type gsub1_2 struct {
	cov                coverage
	substituteGlyphIDs []font.GlyphID
}

func (g *GTab) readGsub1_2(s *parser.State, subtablePos int64) (*gsub1_2, error) {
	err := g.Exec(s,
		parser.CmdRead16, parser.TypeUInt, // coverageOffset
		parser.CmdStoreInto, 0,
		parser.CmdRead16, parser.TypeUInt, // glyphCount
		parser.CmdLoop,
		parser.CmdStash, // substitutefont.GlyphIndex[i]
		parser.CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	repl := s.GetStash()
	cov, err := g.readCoverageTable(subtablePos + s.R[0])
	if err != nil {
		return nil, err
	}
	err = cov.check(len(repl))
	if err != nil {
		return nil, err
	}
	res := &gsub1_2{
		cov:                cov,
		substituteGlyphIDs: stashToGlyphs(repl),
	}
	return res, nil
}

func (l *gsub1_2) Apply(filter KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int) {
	glyph := seq[i]
	j, ok := l.cov[glyph.Gid]
	if !ok {
		return seq, -1
	}
	seq[i].Gid = l.substituteGlyphIDs[j]
	return seq, i + 1
}

// Multiple Substitution Subtable
type gsub2_1 struct {
	cov  coverage
	repl [][]font.GlyphID
}

func (g *GTab) readGsub2_1(s *parser.State, subtablePos int64) (*gsub2_1, error) {
	err := g.Exec(s,
		parser.CmdRead16, parser.TypeUInt, // coverageOffset
		parser.CmdStoreInto, 0,
		parser.CmdRead16, parser.TypeUInt, // sequenceCount
		parser.CmdLoop,
		parser.CmdStash, // sequenceOffset[i]
		parser.CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	sequenceOffsets := s.GetStash()
	cov, err := g.readCoverageTable(subtablePos + s.R[0])
	if err != nil {
		return nil, err
	}
	err = cov.check(len(sequenceOffsets))
	if err != nil {
		return nil, err
	}
	repl := make([][]font.GlyphID, len(sequenceOffsets))
	for _, i := range cov {
		s.A = subtablePos + int64(sequenceOffsets[i])
		err = g.Exec(s,
			parser.CmdSeek,
			parser.CmdRead16, parser.TypeUInt, // glyphCount
			parser.CmdLoop,
			parser.CmdStash, // substituteGlyphID[j]
			parser.CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		repl[i] = stashToGlyphs(s.GetStash())
	}
	res := &gsub2_1{
		cov:  cov,
		repl: repl,
	}
	return res, nil
}

func (l *gsub2_1) Apply(filter KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int) {
	k, ok := l.cov[seq[i].Gid]
	if !ok {
		return seq, -1
	}
	replGid := l.repl[k]

	n := len(replGid)
	repl := make([]font.Glyph, n)
	for i, gid := range replGid {
		repl[i].Gid = gid
	}
	if n > 0 {
		// TODO(voss): What should we do for n=0?
		repl[i].Chars = seq[i].Chars
	}

	res := append(seq, repl...) // just to allocate enough backing space
	copy(seq[i:], repl)
	copy(seq[i+n:], seq[i+1:])
	return res, i + n
}

// Ligature Substitution Format 1
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#41-ligature-substitution-format-1
type gsub4_1 struct {
	cov  coverage // maps first glyphs to repl indices
	repl [][]ligature
}

type ligature struct {
	in  []font.GlyphID // excludes the first input glyph, since this is in cov
	out font.GlyphID
}

func (g *GTab) readGsub4_1(s *parser.State, subtablePos int64) (*gsub4_1, error) {
	err := g.Exec(s,
		parser.CmdRead16, parser.TypeUInt, // coverageOffset
		parser.CmdStoreInto, 0,
		parser.CmdRead16, parser.TypeUInt, // ligatureSetCount
		parser.CmdLoop,
		parser.CmdStash, // ligatureSetOffset[i]
		parser.CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	ligatureSetOffsets := s.GetStash()
	cov, err := g.readCoverageTable(subtablePos + s.R[0])
	if err != nil {
		return nil, err
	}
	xxx := &gsub4_1{
		cov:  cov,
		repl: make([][]ligature, len(ligatureSetOffsets)),
	}
	for _, i := range cov {
		if i >= len(ligatureSetOffsets) {
			return nil, g.Error("ligatureSetOffset %d out of range", i)
		}
		ligSetTablePos := subtablePos + int64(ligatureSetOffsets[i])
		s.A = ligSetTablePos
		err = g.Exec(s,
			parser.CmdSeek,
			parser.CmdRead16, parser.TypeUInt, // ligatureCount
			parser.CmdLoop,
			parser.CmdStash, // ligatureOffset[i]
			parser.CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		ligatureOffsets := s.GetStash()
		for _, o2 := range ligatureOffsets {
			s.A = ligSetTablePos + int64(o2)
			err = g.Exec(s,
				parser.CmdSeek,
				parser.CmdStash,                   // ligatureGlyph
				parser.CmdRead16, parser.TypeUInt, // componentCount
				parser.CmdAssertGt, 0,
				parser.CmdDec,
				parser.CmdLoop,
				parser.CmdStash, // componentfont.GlyphIndex[i]
				parser.CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			xx := s.GetStash()

			xxx.repl[i] = append(xxx.repl[i], ligature{
				in:  stashToGlyphs(xx[1:]),
				out: font.GlyphID(xx[0]),
			})
		}
	}
	return xxx, nil
}

func (l *gsub4_1) Apply(filter KeepGlyphFn, seq []font.Glyph, pos int) ([]font.Glyph, int) {
	ligSetIdx, ok := l.cov[seq[pos].Gid]
	if !ok {
		return seq, -1
	}
	ligSet := l.repl[ligSetIdx]

ligLoop:
	for j := range ligSet {
		lig := &ligSet[j]
		p := pos
		for k, gid := range lig.in {
			p++
			if p >= len(seq) {
				continue ligLoop
			}
			if !filter(seq[pos+1+k].Gid) {
				panic("not implemented")
			}
			if seq[p].Gid != gid {
				continue ligLoop
			}
		}
		next := p + 1

		// gather the unicode representations
		var rr []rune
		for i := pos; i < next; i++ {
			rr = append(rr, seq[i].Chars...)
		}

		seq[pos] = font.Glyph{
			Gid:   lig.out,
			Chars: rr,
		}
		seq = append(seq[:pos+1], seq[next:]...)
		return seq, pos + 1
	}

	return seq, -1
}

func stashToGlyphs(x []uint16) []font.GlyphID {
	res := make([]font.GlyphID, len(x))
	for i, gid := range x {
		res[i] = font.GlyphID(gid)
	}
	return res
}