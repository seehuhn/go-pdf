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
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/font/sfntcff"
)

// ExplainGsub returns a human-readable, textual description of the lookups
// in a GSUB table.  This function panics if `ll` is not a GSUB table.
func ExplainGsub(fontInfo *sfntcff.Info) string {
	ee := newExplainer(fontInfo)

	ll := fontInfo.Gsub.LookupList
	for _, lookup := range ll {
		checkType := func(newType int) {
			if newType != int(lookup.Meta.LookupType) {
				panic("inconsistent subtables")
			}
		}

		for i, subtable := range lookup.Subtables {
			var mappings []mapping
			var actions []action
			var classActions []classAction
			var cov coverage.Table

			switch l := subtable.(type) {
			case *gtab.Gsub1_1:
				checkType(1)
				for key := range l.Cov {
					mappings = append(mappings, mapping{
						from: []font.GlyphID{key},
						to:   []font.GlyphID{key + l.Delta},
					})
				}
			case *gtab.Gsub1_2:
				checkType(1)
				for key, idx := range l.Cov {
					mappings = append(mappings, mapping{
						from: []font.GlyphID{key},
						to:   []font.GlyphID{l.SubstituteGlyphIDs[idx]},
					})
				}
			case *gtab.Gsub2_1:
				checkType(2)
				for key, idx := range l.Cov {
					mappings = append(mappings, mapping{
						from: []font.GlyphID{key},
						to:   l.Repl[idx],
					})
				}
			case *gtab.Gsub3_1:
				checkType(3)
				for gid, idx := range l.Cov {
					mappings = append(mappings, mapping{
						from: []font.GlyphID{gid},
						to:   l.Alternates[idx],
					})
				}
			case *gtab.Gsub4_1:
				checkType(4)
				for gid, idx := range l.Cov {
					for _, lig := range l.Repl[idx] {
						mappings = append(mappings, mapping{
							from: append([]font.GlyphID{gid}, lig.In...),
							to:   []font.GlyphID{lig.Out},
						})
					}
				}
			case *gtab.SeqContext1:
				checkType(5)
				for gid, idx := range l.Cov {
					for _, rule := range l.Rules[idx] {
						actions = append(actions, action{
							from:    append([]font.GlyphID{gid}, rule.In...),
							actions: rule.Actions,
						})
					}
				}
			case *gtab.SeqContext2:
				checkType(5)
				cov = l.Cov
				for classIdx, rules := range l.Rules {
					for _, rule := range rules {
						classActions = append(classActions, classAction{
							from:    append([]uint16{uint16(classIdx)}, rule.In...),
							actions: rule.Actions,
						})
					}
				}
			case *gtab.SeqContext3:
				checkType(5)
			case *gtab.ChainedSeqContext1:
				checkType(6)
			case *gtab.ChainedSeqContext2:
				checkType(6)
			case *gtab.ChainedSeqContext3:
				checkType(6)
			default:
				panic(fmt.Sprintf("unsupported subtable type %T", l))
			}

			if i == 0 {
				fmt.Fprintf(ee.w, "GSUB_%d:", lookup.Meta.LookupType)
				ee.explainFlags(lookup.Meta.LookupFlag)
			} else {
				ee.w.WriteString(" ||\n    ")
			}
			switch lookup.Meta.LookupType {
			case 1, 2, 4:
				ee.explainMappings(mappings)
			case 3:
				ee.explainSetMappings(mappings)
			case 5:
				if len(actions) > 0 { // format 1
					ee.explainActions(actions)
				} else if len(classActions) > 0 { // format 2
					ee.explainCoverage(cov)
				} else {
					ee.w.WriteString("...")
				}
			default:
				ee.w.WriteString(" # not yet implemented")
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

func newExplainer(fontInfo *sfntcff.Info) *explainer {
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

func (ee *explainer) explainMappings(mm []mapping) {
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
			ee.explainSequence(mm[0].from)
			ee.w.WriteString(" -> ")
			ee.explainSequence(mm[0].to)
			mm = mm[1:]
		}
	}
}

func (ee *explainer) explainSetMappings(mm []mapping) {
	sort.SliceStable(mm, func(i, j int) bool {
		return mm[i].from[0] < mm[j].from[0]
	})

	sep := " "
	for len(mm) > 0 {
		ee.w.WriteString(sep)
		sep = ", "

		ee.explainSequence(mm[0].from)
		ee.w.WriteString(" -> [")
		ee.explainSequence(mm[0].to)
		ee.w.WriteRune(']')
		mm = mm[1:]
	}
}

type action struct {
	from    []font.GlyphID
	actions gtab.Nested
}

func (ee *explainer) explainActions(aa []action) {
	sep := " "
	for len(aa) > 0 {
		ee.w.WriteString(sep)
		sep = ", "

		ee.explainSequence(aa[0].from)
		ee.w.WriteString(" -> ")
		ee.explainNested(aa[0].actions)
		aa = aa[1:]
	}
}

type classAction struct {
	from    []uint16
	actions gtab.Nested
}

func (ee *explainer) explainSequence(seq []font.GlyphID) {
	if len(seq) == 0 {
		return
	}

	if len(seq) == 1 && "\""+ee.names[seq[0]]+"\"" == ee.mapped[seq[0]] {
		ee.w.WriteString(ee.names[seq[0]])
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
		ee.w.WriteRune('"')
		for _, gid := range seq {
			name := ee.mapped[gid]
			ee.w.WriteString(name[1 : len(name)-1])
		}
		ee.w.WriteRune('"')
	} else {
		for i, gid := range seq {
			if i > 0 {
				ee.w.WriteRune(' ')
			}
			ee.w.WriteString(ee.names[gid])
		}
	}
}

func (ee *explainer) explainCoverage(cov coverage.Table) {
	keys := maps.Keys(cov)
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	ee.w.WriteRune('/')
	ee.explainSequence(keys)
	ee.w.WriteRune('/')
}

func (ee *explainer) explainNested(actions gtab.Nested) {
	for i, a := range actions {
		if i > 0 {
			ee.w.WriteRune(' ')
		}
		fmt.Fprintf(ee.w, "%d@%d", a.LookupListIndex, a.SequenceIndex)
	}
}