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
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"seehuhn.de/go/pdf/font"
)

type gTab struct {
	*Parser

	glyphNames map[font.GlyphID]rune // TODO(voss): remove

	lookupIndices    map[string][]uint16
	lookupListOffset int64
	lookups          []uint16

	coverageCache map[int64]coverage
}

// modifies p.Funcs
func newGTab(p *Parser, script, lang string) (*gTab, error) {
	scriptOffs := int64(-1)
	scriptSeen := false
	chooseScript := func(s *State) {
		T := uint32(s.R[4])
		tag := string([]byte{byte(T >> 24), byte(T >> 16), byte(T >> 8), byte(T)})
		if tag == script {
			scriptOffs = s.A
			scriptSeen = true
		} else if tag == "DFLT" && !scriptSeen || scriptOffs < 0 {
			scriptOffs = s.A
		}
		s.A = scriptOffs
	}

	chooseLang := func(s *State) {
		T := uint32(s.R[4])
		tag := string([]byte{byte(T >> 24), byte(T >> 16), byte(T >> 8), byte(T)})
		if tag == lang {
			s.R[0] = s.A
		}
	}

	p.Funcs = []func(*State){
		chooseScript,
		chooseLang,
	}

	res := &gTab{
		Parser:        p,
		coverageCache: make(map[int64]coverage),
	}

	res.glyphNames = make(map[font.GlyphID]rune)
	cmap, err := p.tt.SelectCmap()
	if err != nil {
		return nil, err
	}
	for r, gid := range cmap {
		res.glyphNames[gid] = r
	}

	return res, nil
}

func (g *gTab) glyphName(gid font.GlyphID) string {
	r, ok := g.glyphNames[gid]
	if !ok {
		return fmt.Sprintf("[%d]", gid)
	}
	if unicode.IsMark(r) {
		return string([]rune{' ', r})
	}
	return string(r)
}

func (g *gTab) explainCoverage(cov coverage) string {
	var res []string
	for gid := range cov {
		res = append(res, g.glyphName(gid))
	}
	return "{" + strings.Join(res, "") + "}"
}

func (g *gTab) init(tableName string, includeFeature map[string]bool) error {
	err := g.SetTable(tableName)
	if err != nil {
		return err
	}

	s := &State{}

	err = g.Exec(s,
		// Read the table header.
		CmdRead16, TypeUInt, // majorVersion
		CmdAssertEq, 1,
		CmdRead16, TypeUInt, // minorVersion
		CmdAssertLe, 1,
		CmdStore, 0,
		CmdRead16, TypeUInt, // scriptListOffset
		CmdStore, 1,
		CmdRead16, TypeUInt, // featureListOffset
		CmdStore, 2,
		CmdRead16, TypeUInt, // lookupListOffset
		CmdStore, 3,
		CmdLoad, 0,
		CmdCmpLt, 1,
		CmdJNZ, JumpOffset(2),
		CmdRead16, TypeUInt, // featureVariationsOffset (only version 1.1)

		// Read the script list and pick a script.
		CmdLoad, 1, // scriptListOffset
		CmdSeek,
		CmdRead16, TypeUInt,
		CmdLoop,
		CmdRead32, TypeUInt, // scriptTag
		CmdStore, 4,
		CmdRead16, TypeUInt, // scriptOffset
		CmdCall, 0, // chooseScript(R4=tag, A=offs) -> A=best offs
		CmdEndLoop,
		CmdAssertGt, 0, // TODO(voss): otherwise use all available features?

		// Read the language list for the script, and pick a LangSys
		CmdAdd, 1, // position of Script Table
		CmdStore, 1,
		CmdSeek,
		CmdRead16, TypeUInt, // defaultLangSysOffset
		CmdStore, 0,
		CmdRead16, TypeUInt, // langSysCount
		CmdLoop,
		CmdRead32, TypeUInt, // langSysTag
		CmdStore, 4,
		CmdRead16, TypeUInt, // langSysOffset
		CmdCall, 1, // chooseLang(R4=tag, A=offs), updates R0
		CmdEndLoop,
		CmdLoad, 0, // this is the chosen langSysOffset
		CmdAssertGt, 0, // TODO(voss): otherwise use any features?

		// Read the LangSys table
		CmdAdd, 1, // position of the LangSys table
		CmdSeek,
		CmdRead16, TypeUInt, // lookupOrderOffset
		CmdAssertEq, 0,
		CmdRead16, TypeUInt, // requiredFeatureIndex
		CmdStash,
		CmdRead16, TypeUInt, // featureIndexCount
		CmdLoop,
		CmdRead16, TypeUInt, // featureIndices[i]
		CmdStash,
		CmdEndLoop,

		// Read the number of features in the feature list
		CmdLoad, 2, // featureListOffset
		CmdSeek,
		CmdRead16, TypeUInt, // featureCount
	)
	if err != nil {
		return err
	}
	featureIndices := append([]uint16{}, s.GetStash()...)
	if featureIndices[0] == 0xFFFF {
		// missing requiredFeatureIndex
		featureIndices = featureIndices[1:]
	}
	numFeatures := int(s.A)

	featureOffsets := map[string][]int64{}
	for _, fi := range featureIndices {
		if int(fi) >= numFeatures {
			return errors.New("feature index out of range")
		}
		s.A = s.R[2] + 2 + 6*int64(fi)
		err = g.Exec(s,
			CmdSeek,
			CmdRead32, TypeUInt, // featureTag
			CmdStore, 4,
			CmdRead16, TypeUInt, // featureOffset
			CmdAdd, 2, // add the base address (featureListOffset)
		)
		if err != nil {
			return err
		}

		T := uint32(s.R[4])
		tag := string([]byte{byte(T >> 24), byte(T >> 16), byte(T >> 8), byte(T)})
		if includeFeature[tag] {
			featureOffsets[tag] = append(featureOffsets[tag], s.A)
		}
	}

	g.lookupIndices = make(map[string][]uint16)
	for name, featureOffs := range featureOffsets {
		fmt.Println("feature", name+":", len(featureOffs), "entries")
		for _, offs := range featureOffs {
			s.A = offs
			err = g.Exec(s,
				CmdSeek,             // start of Feature table
				CmdRead16, TypeUInt, // featureParamsOffset
				CmdAssertEq, 0,
				CmdRead16, TypeUInt, // lookupIndexCount
				CmdLoop,
				CmdRead16, TypeUInt, // lookupIndex
				CmdStash,
				CmdEndLoop,
			)
			if err != nil {
				return err
			}
			g.lookupIndices[name] = append(g.lookupIndices[name], s.GetStash()...)
		}
	}
	for key := range g.lookupIndices {
		tab := g.lookupIndices[key]
		sort.Slice(tab, func(i, j int) bool {
			return tab[i] < tab[j]
		})
	}

	err = g.Exec(s,
		CmdLoad, 3, // lookupListOffset
		CmdSeek,
		CmdRead16, TypeUInt, // lookupCount
		CmdLoop,
		CmdRead16, TypeUInt, // lookupOffset[i]
		CmdStash,
		CmdEndLoop,
	)
	if err != nil {
		return err
	}
	g.lookups = s.GetStash()
	g.lookupListOffset = s.R[3]

	return nil
}

func (g *gTab) readCoverageTable(pos int64) (coverage, error) {
	res, ok := g.coverageCache[pos]
	if ok {
		return res, nil
	}

	s := &State{
		A: pos,
	}
	err := g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // format
	)
	if err != nil {
		return nil, err
	}
	format := int(s.A)

	res = make(coverage)

	switch format {
	case 1: // coverage table format 1
		err = g.Exec(s,
			CmdRead16, TypeUInt, // glyphCount
			CmdLoop,
			CmdRead16, TypeUInt, // glyphArray[i]
			CmdStash,
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		for k, gid := range s.GetStash() {
			res[font.GlyphID(gid)] = k
		}
	case 2: // coverage table format 2
		err = g.Exec(s,
			CmdRead16, TypeUInt, // rangeCount
		)
		if err != nil {
			return nil, err
		}
		rangeCount := int(s.A)
		for i := 0; i < rangeCount; i++ {
			err = g.Exec(s,
				CmdRead16, TypeUInt, // startfont.GlyphIndex
				CmdStash,
				CmdRead16, TypeUInt, // endfont.GlyphIndex
				CmdStash,
				CmdRead16, TypeUInt, // startCoverageIndex
				CmdStash,
			)
			if err != nil {
				return nil, err
			}
			xx := s.GetStash()
			for k := int(xx[0]); k <= int(xx[1]); k++ {
				res[font.GlyphID(k)] = k - int(xx[0]) + int(xx[2])
			}
		}
	default:
		return nil, fmt.Errorf("unsupported coverage format %d", format)
	}

	g.coverageCache[pos] = res
	return res, nil
}

func (g *gTab) getGsubLookup(idx uint16, pfx string) (interface{}, error) {
	if idx < 0 || int(idx) >= len(g.lookups) {
		return nil, errors.New("lookup out of range")
	}
	pos := g.lookupListOffset + int64(g.lookups[idx])

	s := &State{}
	s.A = pos
	err := g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // lookupType
		CmdStore, 0,
		CmdStash,
		CmdRead16, TypeUInt, // lookupFlag
		CmdStash,
		CmdRead16, TypeUInt, // subtableCount
		CmdLoop,
		CmdRead16, TypeUInt, // subtableOffset
		CmdStash,
		CmdEndLoop,
		// TODO(voss): conditionally read markFilteringSet
	)
	if err != nil {
		return nil, err
	}
	data := s.GetStash()

	format := data[0]
	flags := data[1]
	subtables := data[2:]
	for _, inc := range subtables {
		subtablePos := pos + int64(inc)
		s.A = subtablePos
		err = g.Exec(s,
			CmdSeek,
			CmdRead16, TypeUInt,
		)
		if err != nil {
			return nil, err
		}
		subFormat := s.A

		var res GsubLookup

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
						return nil, errors.New("ligatureSetOffset out of range")
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
			default:
				fmt.Printf("%s- unsupported subtable format %d.%d",
					pfx, format, subFormat)
			}
		}
		if res == nil {
			fmt.Printf("%sunsupported lookup type %d.%d\n",
				pfx, format, subFormat)
		} else {
			res.explain(g, pfx)
		}
	}

	return "something", nil
}

func stashToGlyphs(x []uint16) []font.GlyphID {
	var res []font.GlyphID
	for _, gid := range x {
		res = append(res, font.GlyphID(gid))
	}
	return res
}

type coverage map[font.GlyphID]int

func (cov coverage) check(size int) error {
	for _, k := range cov {
		if k < 0 || size >= 0 && k >= size {
			return errors.New("invalid coverage table")
		}
	}
	return nil
}
