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

package builder

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
)

// ExplainGsub returns a human-readable, textual description of the lookups
// in a GSUB table.
func ExplainGsub(fontInfo *sfnt.Info) string {
	ee := newExplainer(fontInfo)

	for _, lookup := range fontInfo.Gsub.LookupList {
		checkType := func(newType int) {
			if newType != int(lookup.Meta.LookupType) {
				panic("inconsistent subtable types")
			}
		}

		for i, subtable := range lookup.Subtables {
			if i == 0 {
				fmt.Fprintf(ee.w, "GSUB%d:", lookup.Meta.LookupType)
				ee.explainFlags(lookup.Meta.LookupFlag)
			} else {
				ee.w.WriteString(" ||\n\t")
			}

			switch l := subtable.(type) {
			case *gtab.Gsub1_1:
				checkType(1)
				var mappings []mapping
				for key := range l.Cov {
					mappings = append(mappings, mapping{
						from: []font.GlyphID{key},
						to:   []font.GlyphID{key + l.Delta},
					})
				}
				ee.explainSeqMappings(mappings)

			case *gtab.Gsub1_2:
				checkType(1)
				var mappings []mapping
				for key, idx := range l.Cov {
					mappings = append(mappings, mapping{
						from: []font.GlyphID{key},
						to:   []font.GlyphID{l.SubstituteGlyphIDs[idx]},
					})
				}
				ee.explainSeqMappings(mappings)

			case *gtab.Gsub2_1:
				checkType(2)
				inputGlyphs := maps.Keys(l.Cov)
				sort.Slice(inputGlyphs, func(i, j int) bool { return inputGlyphs[i] < inputGlyphs[j] })
				for i, gid := range inputGlyphs {
					if i == 0 {
						ee.w.WriteRune(' ')
					} else {
						ee.w.WriteString(", ")
					}
					ee.writeGlyphList([]font.GlyphID{gid})
					ee.w.WriteString(" -> ")
					ee.writeGlyphList(l.Repl[i])
				}

			case *gtab.Gsub3_1:
				checkType(3)
				inputGlyphs := maps.Keys(l.Cov)
				sort.Slice(inputGlyphs, func(i, j int) bool { return inputGlyphs[i] < inputGlyphs[j] })
				for i, gid := range inputGlyphs {
					if i == 0 {
						ee.w.WriteRune(' ')
					} else {
						ee.w.WriteString(", ")
					}
					ee.writeGlyphList([]font.GlyphID{gid})
					ee.w.WriteString(" -> ")
					ee.writeGlyphSet(l.Alternates[i])
				}

			case *gtab.Gsub4_1:
				checkType(4)
				var mappings []mapping
				for gid, idx := range l.Cov {
					for _, lig := range l.Repl[idx] {
						mappings = append(mappings, mapping{
							from: append([]font.GlyphID{gid}, lig.In...),
							to:   []font.GlyphID{lig.Out},
						})
					}
				}
				ee.explainSeqMappings(mappings)

			case *gtab.SeqContext1:
				checkType(5)
				firstGlyphs := maps.Keys(l.Cov)
				sort.Slice(firstGlyphs, func(i, j int) bool { return firstGlyphs[i] < firstGlyphs[j] })
				first := true
				for i, gid := range firstGlyphs {
					input := []font.GlyphID{gid}
					for _, rule := range l.Rules[i] {
						input = append(input[:1], rule.Input...)
						if first {
							ee.w.WriteRune(' ')
							first = false
						} else {
							ee.w.WriteString(", ")
						}
						ee.writeGlyphList(input)
						ee.w.WriteString(" -> ")
						ee.explainNested(rule.Actions)
					}
				}

			case *gtab.SeqContext2:
				checkType(5)
				ee.defineClasses("class", l.Input)
				ee.w.WriteString("/")
				ee.explainCoverage(l.Cov)
				ee.w.WriteRune('/')
				first := true
				for cls, rules := range l.Rules {
					input := []uint16{uint16(cls)}
					for _, rule := range rules {
						if first {
							first = false
						} else {
							ee.w.WriteRune(',')
						}
						input = append(input[:1], rule.Input...)
						ee.writeClassList(input)
						ee.w.WriteString(" -> ")
						ee.explainNested(rule.Actions)
					}
				}

			case *gtab.SeqContext3:
				checkType(5)
				for i, cov := range l.Input {
					if i > 0 {
						ee.w.WriteRune(' ')
					}
					ee.writeCoveredSet(cov)
				}
				ee.w.WriteString(" -> ")
				ee.explainNested(l.Actions)

			case *gtab.ChainedSeqContext1:
				checkType(6)
				firstGlyphs := maps.Keys(l.Cov)
				sort.Slice(firstGlyphs, func(i, j int) bool { return firstGlyphs[i] < firstGlyphs[j] })
				first := true
				for i, gid := range firstGlyphs {
					input := []font.GlyphID{gid}
					for _, rule := range l.Rules[i] {
						backtrack := copyRev(rule.Backtrack)
						input = append(input[:1], rule.Input...)
						lookahead := rule.Lookahead
						if first {
							ee.w.WriteRune(' ')
							first = false
						} else {
							ee.w.WriteString(", ")
						}
						ee.writeGlyphList(backtrack)
						ee.w.WriteString(" | ")
						ee.writeGlyphList(input)
						ee.w.WriteString(" | ")
						ee.writeGlyphList(lookahead)
						ee.w.WriteString(" -> ")
						ee.explainNested(rule.Actions)
					}
				}

			case *gtab.ChainedSeqContext2:
				checkType(6)
				ee.defineClasses("backtrackclass", l.Backtrack)
				ee.defineClasses("inputclass", l.Input)
				ee.defineClasses("lookaheadclass", l.Lookahead)
				ee.w.WriteString("/")
				ee.explainCoverage(l.Cov)
				ee.w.WriteRune('/')
				first := true
				for cls, rules := range l.Rules {
					input := []uint16{uint16(cls)}
					for _, rule := range rules {
						if first {
							first = false
						} else {
							ee.w.WriteRune(',')
						}
						backtrack := copyRev(rule.Backtrack)
						input = append(input[:1], rule.Input...)
						lookahead := rule.Lookahead
						ee.writeClassList(backtrack)
						ee.w.WriteString(" | ")
						ee.writeClassList(input)
						ee.w.WriteString(" | ")
						ee.writeClassList(lookahead)
						ee.w.WriteString(" -> ")
						ee.explainNested(rule.Actions)
					}
				}

			case *gtab.ChainedSeqContext3:
				checkType(6)
				for i, cov := range copyRev(l.Backtrack) {
					if i > 0 {
						ee.w.WriteRune(' ')
					}
					ee.writeCoveredSet(cov)
				}
				ee.w.WriteString(" |")
				for _, cov := range l.Input {
					ee.w.WriteRune(' ')
					ee.writeCoveredSet(cov)
				}
				ee.w.WriteString(" |")
				for _, cov := range l.Lookahead {
					ee.w.WriteRune(' ')
					ee.writeCoveredSet(cov)
				}
				ee.w.WriteString(" -> ")
				ee.explainNested(l.Actions)

			default:
				panic(fmt.Sprintf("unsupported GSUB subtable type %T", l))
			}
		}
		ee.w.WriteRune('\n')
	}

	return ee.w.String()
}

// ExplainGpos returns a human-readable, textual description of the lookups
// in a GPOS table.
func ExplainGpos(fontInfo *sfnt.Info) string {
	ee := newExplainer(fontInfo)

	for _, lookup := range fontInfo.Gpos.LookupList {
		checkType := func(newType int) {
			if newType != int(lookup.Meta.LookupType) {
				panic("inconsistent subtable types")
			}
		}

		for i, subtable := range lookup.Subtables {
			if i == 0 {
				fmt.Fprintf(ee.w, "GPOS%d:", lookup.Meta.LookupType)
				ee.explainFlags(lookup.Meta.LookupFlag)
			} else {
				ee.w.WriteString(" ||\n\t")
			}

			switch l := subtable.(type) {
			case *gtab.Gpos1_1:
				checkType(1)
				ee.w.WriteRune(' ')
				ee.writeGlyphSet(maps.Keys(l.Cov))
				ee.w.WriteString(" -> ")
				ee.writeValueRecord(l.Adjust)

			case *gtab.Gpos1_2:
				checkType(1)
				gids := maps.Keys(l.Cov)
				sort.Slice(gids, func(i, j int) bool { return gids[i] < gids[j] })
				for i, gid := range gids {
					if i > 0 {
						ee.w.WriteString(", ")
					} else {
						ee.w.WriteRune(' ')
					}
					ee.writeGlyph(gid)
					ee.w.WriteString(" -> ")
					ee.writeValueRecord(l.Adjust[l.Cov[gid]])
				}

			case *gtab.Gpos2_1:
				checkType(2)
				firstGids := maps.Keys(l.Cov)
				sort.Slice(firstGids, func(i, j int) bool { return firstGids[i] < firstGids[j] })
				first := true
				for _, firstGid := range firstGids {
					idx := l.Cov[firstGid]
					row := l.Adjust[idx]
					secondGids := maps.Keys(row)
					sort.Slice(secondGids, func(i, j int) bool { return secondGids[i] < secondGids[j] })
					for _, secondGid := range secondGids {
						if first {
							ee.w.WriteRune(' ')
							first = false
						} else {
							ee.w.WriteString(", ")
						}
						ee.writeGlyphList([]font.GlyphID{firstGid, secondGid})
						ee.w.WriteString(" -> ")
						col := row[secondGid]
						ee.writeValueRecord(col.First)
						if col.Second != nil {
							ee.w.WriteString(" & ")
							ee.writeValueRecord(col.Second)
						}
					}
				}

			default:
				panic(fmt.Sprintf("unsupported GPOS subtable type %T", l))
			}
		}
		ee.w.WriteRune('\n')
	}

	return ee.w.String()
}

type explainer struct {
	w      *strings.Builder
	mapped []string
	names  []string
}

func newExplainer(fontInfo *sfnt.Info) *explainer {
	mappings := make([]string, fontInfo.NumGlyphs())
	a, b := fontInfo.CMap.CodeRange()
	for r := a; r <= b; r++ {
		gid := fontInfo.CMap.Lookup(r)
		if gid != 0 {
			mappings[gid] = fmt.Sprintf("%q", string([]rune{r}))
		}
	}

	names := make([]string, fontInfo.NumGlyphs())
	for i := range names {
		name := fontInfo.GlyphName(font.GlyphID(i))
		if name != "" {
			names[i] = string(name)
		} else {
			names[i] = fmt.Sprintf("%d", i)
		}
	}

	return &explainer{
		w:      &strings.Builder{},
		mapped: mappings,
		names:  names,
	}
}

func (ee *explainer) explainFlags(flags gtab.LookupFlags) {
	if flags&gtab.LookupIgnoreMarks != 0 {
		ee.w.WriteString(" -marks")
	}
	if flags&gtab.LookupRightToLeft != 0 {
		ee.w.WriteString(" -rtl")
	}
	if flags&gtab.LookupIgnoreBaseGlyphs != 0 {
		ee.w.WriteString(" -base")
	}
	if flags&gtab.LookupIgnoreLigatures != 0 {
		ee.w.WriteString(" -lig")
	}
	// if flags&LookupUseMarkFilteringSet != 0 {
	// 	ee.w.WriteString(" -UseMarkFilteringSet")
	// }
	// if flags&LookupMarkAttachTypeMask != 0 {
	// 	ee.w.WriteString(" -MarkAttachTypeMask")
	// }
}

type mapping struct {
	from []font.GlyphID
	to   []font.GlyphID
}

func (ee *explainer) explainSeqMappings(mm []mapping) {
	sort.SliceStable(mm, func(i, j int) bool {
		return mm[i].from[0] < mm[j].from[0]
	})

	sep := " "
	for len(mm) > 0 {
		ee.w.WriteString(sep)
		sep = ", "

		canRange := len(mm) > 2
		for i := 1; canRange && i < len(mm); i++ {
			if len(mm[i].from) != 1 || len(mm[i].to) != 1 {
				canRange = false
			}
		}

		rangeLen := 1
		if canRange {
			delta := mm[0].to[0] - mm[0].from[0]
			for i := 1; i < len(mm); i++ {
				from := mm[i].from[0]
				to := mm[i].to[0]
				if from != mm[i-1].from[0]+1 || to != from+delta {
					break
				}
				rangeLen++
			}
		}

		if rangeLen > 2 {
			fmt.Fprintf(ee.w, "%s-%s -> %s-%s",
				ee.names[mm[0].from[0]],
				ee.names[mm[rangeLen-1].from[0]],
				ee.names[mm[0].to[0]],
				ee.names[mm[rangeLen-1].to[0]],
			)
			mm = mm[rangeLen:]
		} else {
			ee.writeGlyphList(mm[0].from)
			ee.w.WriteString(" -> ")
			ee.writeGlyphList(mm[0].to)
			mm = mm[1:]
		}
	}
}

func (ee *explainer) writeGlyph(gid font.GlyphID) {
	if "\""+ee.names[gid]+"\"" == ee.mapped[gid] {
		ee.w.WriteString(ee.names[gid])
	} else if ee.mapped[gid] != "" {
		ee.w.WriteString(ee.mapped[gid])
	} else if ee.names[gid] != "" {
		ee.w.WriteString(ee.names[gid])
	} else {
		ee.w.WriteString(fmt.Sprintf("%d", gid))
	}
}

func (ee *explainer) writeGlyphList(seq []font.GlyphID) {
	if len(seq) == 0 {
		return
	}

	w := ee.w

	if len(seq) == 1 && "\""+ee.names[seq[0]]+"\"" == ee.mapped[seq[0]] {
		w.WriteString(ee.names[seq[0]])
		return
	}

	canUseMapped := true
	for _, gid := range seq {
		if ee.mapped[gid] == "" {
			canUseMapped = false
			break
		}
	}

	if canUseMapped {
		w.WriteRune('"')
		for _, gid := range seq {
			name := ee.mapped[gid]
			w.WriteString(name[1 : len(name)-1])
		}
		w.WriteRune('"')
	} else {
		for i, gid := range seq {
			if i > 0 {
				w.WriteRune(' ')
			}
			w.WriteString(ee.names[gid])
		}
	}
}

func (ee *explainer) writeGlyphSet(seq []font.GlyphID) {
	ee.w.WriteRune('[')
	ee.writeGlyphList(seq)
	ee.w.WriteRune(']')
}

func (ee *explainer) writeClassList(input []uint16) {
	for _, cls := range input {
		if cls == 0 {
			ee.w.WriteString(" ::")
		} else {
			fmt.Fprintf(ee.w, " :c%d:", cls)
		}
	}
}

func (ee *explainer) explainCoverage(cov coverage.Table) {
	keys := maps.Keys(cov)
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	ee.writeGlyphList(keys)
}

func (ee *explainer) writeCoveredSet(cov coverage.Table) {
	keys := maps.Keys(cov)
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	ee.writeGlyphSet(keys)
}

func (ee *explainer) explainNested(actions gtab.SeqLookups) {
	for i, a := range actions {
		if i > 0 {
			ee.w.WriteRune(' ')
		}
		fmt.Fprintf(ee.w, "%d@%d", a.LookupListIndex, a.SequenceIndex)
	}
}

func (ee *explainer) defineClasses(classType string, class classdef.Table) {
	numClasses := class.NumClasses()
	glyphs := make([][]font.GlyphID, numClasses)
	for gid, cls := range class {
		glyphs[cls] = append(glyphs[cls], gid)
	}
	for j := 0; j < numClasses; j++ {
		if j == 0 {
			continue
		}
		sort.Slice(glyphs[j], func(k, l int) bool { return glyphs[j][k] < glyphs[j][l] })
		fmt.Fprintf(ee.w, "%s :c%d: = ", classType, j)
		ee.writeGlyphSet(glyphs[j])
		ee.w.WriteString("\n\t")
	}
}

func (ee *explainer) writeValueRecord(adjust *gtab.GposValueRecord) {
	var parts []string
	if adjust.XPlacement != 0 {
		parts = append(parts, fmt.Sprintf("x%+d", adjust.XPlacement))
	}
	if adjust.YPlacement != 0 {
		parts = append(parts, fmt.Sprintf("y%+d", adjust.YPlacement))
	}
	if adjust.XAdvance != 0 {
		parts = append(parts, fmt.Sprintf("dx%+d", adjust.XAdvance))
	}
	if adjust.YAdvance != 0 {
		parts = append(parts, fmt.Sprintf("dy%+d", adjust.YAdvance))
	}
	if len(parts) == 0 {
		ee.w.WriteString("dx+0")
	} else {
		ee.w.WriteString(strings.Join(parts, " "))
	}
}
