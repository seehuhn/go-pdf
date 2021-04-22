// seehuhn.de/go/pdf - support for reading and writing PDF files
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

package parser

import (
	"fmt"
	"strconv"
	"strings"

	"seehuhn.de/go/pdf/font"
)

// http://adobe-type-tools.github.io/afdko/OpenTypeFeatureFileSpecification.html#7
// TODO(voss): read this carefully

type gsubLookup []gsubLookupSubtable

func (l gsubLookup) Substitute(glyphs []font.GlyphID) []font.GlyphID {
	pos := 0
glyphLoop:
	for pos < len(glyphs) {
		var next int
		for _, subtable := range l {
			next, glyphs = subtable.Replace(pos, glyphs)
			if next > pos {
				pos = next
				continue glyphLoop
			}
		}
		pos++
	}
	return glyphs
}

type gsubLookupSubtable interface {
	Replace(int, []font.GlyphID) (int, []font.GlyphID)
	explain(g *gTab, pfx string)
}

func (g *gTab) getGsubLookup(idx uint16, pfx string) (gsubLookup, error) {
	if int(idx) >= len(g.lookups) {
		return nil, g.error("lookup index %d out of range", idx)
	}
	base := g.lookupListOffset + int64(g.lookups[idx])

	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-table
	s := &State{}
	s.A = base
	err := g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // lookupType
		CmdStash,
		CmdRead16, TypeUInt, // lookupFlag
		CmdStash,
		CmdRead16, TypeUInt, // subtableCount
		CmdLoop,
		CmdRead16, TypeUInt, // subtableOffset
		CmdStash,
		CmdEndLoop,
	)
	if err != nil {
		return nil, err
	}
	data := s.GetStash()
	format := data[0]
	flags := data[1]
	subtables := data[2:]
	markFilteringSet := -1
	if flags&0x0010 != 0 {
		err = g.Exec(s, CmdRead16, TypeUInt) // markFilteringSet
		if err != nil {
			return nil, err
		}
		markFilteringSet = int(s.A)
	}
	_ = markFilteringSet // TODO(voss): use this correctly

	var lookup gsubLookup
	for _, inc := range subtables {
		subtablePos := base + int64(inc)

		res, err := g.readGsubSubtable(s, flags, format, subtablePos, pfx)
		if err != nil {
			return nil, err
		}

		lookup = append(lookup, res)
	}

	return lookup, nil
}

func (g *gTab) readGsubSubtable(s *State, flags, format uint16, subtablePos int64, pfx string) (gsubLookupSubtable, error) {
	s.A = subtablePos
	err := g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt,
	)
	if err != nil {
		return nil, err
	}
	subFormat := s.A

	var res gsubLookupSubtable

	// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#table-organization
	switch format {
	case 1: // Single Substitution
		// LookupType 1: Single Substitution Subtable
		switch subFormat {
		case 2:
			err = g.Exec(s,
				CmdRead16, TypeUInt, // coverageOffset
				CmdStore, 0,
				CmdRead16, TypeUInt, // glyphCount
				CmdLoop,
				CmdRead16, TypeUInt, // substitutefont.GlyphIndex[i]
				CmdStash,
				CmdEndLoop,
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
			res = &gsubLookup1_2{
				flags:              flags,
				cov:                cov,
				substituteGlyphIDs: stashToGlyphs(repl),
			}
		}
	case 2: // Multiple Substitution Subtable
		switch subFormat {
		case 1:
			err = g.Exec(s,
				CmdRead16, TypeUInt, // coverageOffset
				CmdStore, 0,
				CmdRead16, TypeUInt, // sequenceCount
				CmdLoop,
				CmdRead16, TypeUInt, // sequenceOffset[i]
				CmdStash,
				CmdEndLoop,
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
					CmdSeek,
					CmdRead16, TypeUInt, // glyphCount
					CmdLoop,
					CmdRead16, TypeUInt, // substituteGlyphID[j]
					CmdStash,
					CmdEndLoop,
				)
				if err != nil {
					return nil, err
				}
				repl[i] = stashToGlyphs(s.GetStash())
			}
			res = &gsubLookup2_1{
				flags: flags,
				cov:   cov,
				repl:  repl,
			}
			res.explain(g, pfx)
		}
	case 4: // Ligature Substitution Subtable
		switch subFormat {
		case 1:
			err = g.Exec(s,
				CmdRead16, TypeUInt, // coverageOffset
				CmdStore, 0,
				CmdRead16, TypeUInt, // ligatureSetCount
				CmdLoop,
				CmdRead16, TypeUInt, // ligatureSetOffset[i]
				CmdStash,
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			ligatureSetOffsets := s.GetStash()
			cov, err := g.readCoverageTable(subtablePos + s.R[0])
			if err != nil {
				return nil, err
			}
			fmt.Println(pfx+"\t"+g.explainCoverage(cov), len(ligatureSetOffsets))
			for firstGlyph, i := range cov {
				firstGlyphName := g.glyphName(font.GlyphID(firstGlyph))
				if i >= len(ligatureSetOffsets) {
					return nil, g.error("ligatureSetOffset %d out of range", i)
				}
				ligSetTablePos := subtablePos + int64(ligatureSetOffsets[i])
				s.A = ligSetTablePos
				err = g.Exec(s,
					CmdSeek,
					CmdRead16, TypeUInt, // ligatureCount
					CmdLoop,
					CmdRead16, TypeUInt, // ligatureOffset[i]
					CmdStash,
					CmdEndLoop,
				)
				if err != nil {
					return nil, err
				}
				ligatureOffsets := s.GetStash()
				for _, o2 := range ligatureOffsets {
					s.A = ligSetTablePos + int64(o2)
					err = g.Exec(s,
						CmdSeek,
						CmdRead16, TypeUInt, // ligatureGlyph
						CmdStash,
						CmdRead16, TypeUInt, // componentCount
						CmdDec,
						CmdLoop,
						CmdRead16, TypeUInt, // componentfont.GlyphIndex[i]
						CmdStash,
						CmdEndLoop,
					)
					if err != nil {
						return nil, err
					}
					xx := s.GetStash()

					in := []string{firstGlyphName}
					for _, gid := range xx[1:] {
						in = append(in, g.glyphName(font.GlyphID(gid)))
					}
					out := g.glyphName(font.GlyphID(xx[0]))
					fmt.Println(pfx+"\t"+strings.Join(in, ""), "->", out)
				}
			}
			// TODO(voss): set res
		}
	case 6: // Chained Contexts Substitution Subtable
		fmt.Printf(pfx+"lookup type %d.%d, flags=0x%04x\n",
			format, subFormat, flags)

		switch subFormat {
		case 3:
			err = g.Exec(s,
				CmdRead16, TypeUInt, // backtrackGlyphCount
				CmdLoop,
				CmdRead16, TypeUInt, // backtrackCoverageOffset
				CmdStash,
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			backtrackCoverageOffsets := s.GetStash()

			err = g.Exec(s,
				CmdRead16, TypeUInt, // inputGlyphCount
				CmdLoop,
				CmdRead16, TypeUInt, // inputCoverageOffset
				CmdStash,
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			inputCoverageOffsets := s.GetStash()

			err = g.Exec(s,
				CmdRead16, TypeUInt, // lookaheadGlyphCount
				CmdLoop,
				CmdRead16, TypeUInt, // lookaheadCoverageOffset
				CmdStash,
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			lookaheadCoverageOffsets := s.GetStash()

			err = g.Exec(s,
				CmdRead16, TypeUInt, // seqLookupCount
				CmdLoop,
				CmdRead16, TypeUInt, // seqLookupRecord.sequenceIndex
				CmdStash,
				CmdRead16, TypeUInt, // seqLookupRecord.lookupListIndex
				CmdStash,
				CmdEndLoop,
			)
			if err != nil {
				return nil, err
			}
			seqLookupRecord := s.GetStash()

			for _, offs := range backtrackCoverageOffsets {
				cover, err := g.readCoverageTable(subtablePos + int64(offs))
				if err != nil {
					return nil, err
				}
				fmt.Println(pfx+"\tB", g.explainCoverage(cover))
			}
			for _, offs := range inputCoverageOffsets {
				cover, err := g.readCoverageTable(subtablePos + int64(offs))
				if err != nil {
					return nil, err
				}
				fmt.Println(pfx+"\tI", g.explainCoverage(cover))
			}
			for _, offs := range lookaheadCoverageOffsets {
				cover, err := g.readCoverageTable(subtablePos + int64(offs))
				if err != nil {
					return nil, err
				}
				fmt.Println(pfx+"\tL", g.explainCoverage(cover))
			}

			for len(seqLookupRecord) >= 2 {
				g.getGsubLookup(seqLookupRecord[1], pfx+"\taction "+strconv.Itoa(int(seqLookupRecord[0]))+": ")
				seqLookupRecord = seqLookupRecord[2:]
			}
		}
	}

	if res == nil {
		fmt.Printf("%sunsupported lookup type %d.%d\n", pfx, format, subFormat)
	} else {
		res.explain(g, pfx)
	}
	fmt.Println()

	return res, nil
}

type gsubLookup1_2 struct {
	flags              uint16
	cov                coverage
	substituteGlyphIDs []font.GlyphID
}

func (l *gsubLookup1_2) Replace(i int, seq []font.GlyphID) (int, []font.GlyphID) {
	if l.flags != 0 {
		panic("not implemented")
	}
	in := seq[i]
	j, ok := l.cov[in]
	if ok {
		seq[i] = l.substituteGlyphIDs[j]
	}
	return i + 1, seq
}

func (l *gsubLookup1_2) explain(g *gTab, pfx string) {
	fmt.Printf(pfx+"lookup type 1.2, flags=0x%04x\n", l.flags)
	for gid, k := range l.cov {
		fmt.Printf("%s\t%s -> %s\n",
			pfx, g.glyphName(gid), g.glyphName(l.substituteGlyphIDs[k]))
	}
}

type gsubLookup2_1 struct {
	flags uint16
	cov   coverage
	repl  [][]font.GlyphID
}

func (l *gsubLookup2_1) Replace(i int, seq []font.GlyphID) (int, []font.GlyphID) {
	if l.flags != 0 {
		panic("not implemented")
	}
	k, ok := l.cov[seq[i]]
	if !ok {
		return i + 1, seq
	}
	repl := l.repl[k]
	n := len(repl)

	res := append(seq, repl...) // just to allocate enough backing space
	copy(seq[i+n:], seq[i:])
	copy(seq[i:], repl)
	return i + n, res
}

func (l *gsubLookup2_1) explain(g *gTab, pfx string) {
	fmt.Printf(pfx+"lookup type 2.1, flags=0x%04x\n", l.flags)
	for inGid, k := range l.cov {
		var out []string
		for _, outGid := range l.repl[k] {
			out = append(out, g.glyphName(outGid))
		}
		fmt.Printf("%s\t%s -> %s\n",
			pfx, g.glyphName(inGid), strings.Join(out, ""))
	}
}

type gsubLookup4_1 struct {
	flags uint16
	cov   coverage
	repl  [][]ligature
}

type ligature struct {
	in  []font.GlyphID
	out font.GlyphID
}

func (l *gsubLookup4_1) Replace(i int, seq []font.GlyphID) (int, []font.GlyphID) {
	if l.flags != 0 {
		panic("not implemented")
	}
	ligSetIdx, ok := l.cov[seq[i]]
	if !ok {
		return i + 1, seq
	}
	ligSet := l.repl[ligSetIdx]

ligLoop:
	for j := range ligSet {
		lig := &ligSet[j]
		if i+1+len(lig.in) > len(seq) {
			continue
		}
		for k, gid := range lig.in {
			if seq[i+1+k] != gid {
				continue ligLoop
			}
		}

		seq[i] = lig.out
		seq = append(seq[:i+1], seq[i+1+len(lig.in):]...)
		return i + 1, seq
	}

	return i + 1, seq
}

func (l *gsubLookup4_1) explain(g *gTab, pfx string) {
	fmt.Printf(pfx+"lookup type 4.1, flags=0x%04x\n", l.flags)
	for inGid, k := range l.cov {
		var out []string
		ligSet := l.repl[k]
		for j := range ligSet {
			lig := &ligSet[j]
			for _, outGid := range lig.in {
				out = append(out, g.glyphName(outGid))
			}
			fmt.Printf("%s\t%s -> %s\n",
				pfx, g.glyphName(inGid), strings.Join(out, ""))
		}
	}
}
