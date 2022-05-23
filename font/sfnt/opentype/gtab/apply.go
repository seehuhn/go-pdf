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
	"sort"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
	"seehuhn.de/go/pdf/locale"
)

// ApplyLookup applies a single lookup to the given glyphs.
func (ll LookupList) ApplyLookup(seq []font.Glyph, lookupIndex LookupIndex, gdef *gdef.Table) []font.Glyph {
	pos := 0
	numLeft := len(seq)
	for pos < len(seq) {
		// TODO(voss): GSUB 8.1 subtables are applied in reverse order.
		seq, pos = ll.applyLookupAt(seq, lookupIndex, gdef, pos, len(seq))
		newNumLeft := len(seq) - pos
		if newNumLeft >= numLeft {
			// panic("infinite loop")
			pos = len(seq) - numLeft + 1
		}
		numLeft = newNumLeft
	}

	return seq
}

// applyLookupAt applies a single lookup to the given glyphs at position pos.
func (ll LookupList) applyLookupAt(seq []font.Glyph, lookupIndex LookupIndex, gdef *gdef.Table, pos, b int) ([]font.Glyph, int) {
	numLookups := len(ll)

	if int(lookupIndex) >= numLookups {
		return seq, pos + 1
	}
	lookup := ll[lookupIndex]

	keep := MakeFilter(lookup.Meta, gdef)
	match := lookup.Subtables.Apply(keep, seq, pos, b)
	if match == nil {
		return seq, pos + 1
	}
	if match.Actions == nil {
		seq = applyMatch(seq, match, pos)
		return seq, match.Next
	}

	// TODO(voss): turn actions into a stack (new elements at the back)
	actions := extractActions(match)
	next := match.Next

	numActions := 1 // we count the original lookup as an action
	for len(actions) > 0 && numActions < 64 {
		fmt.Println(actions)

		numActions++
		action := actions[0]
		actions = actions[1:]

		if int(action.lookupListIndex) >= numLookups {
			continue
		}
		lookup = ll[action.lookupListIndex]

		keep = MakeFilter(lookup.Meta, gdef)
		match = lookup.Subtables.Apply(keep, seq, action.a, action.b)
		if match == nil {
			continue
		}
		if match.Actions == nil {
			seq = applyMatch(seq, match, action.a)
			// TODO(voss): update matchPos for the subsequent actions
		} else {
			if match.Replace != nil {
				panic("invalid match object")
			}
			actions = append(extractActions(match), actions...)
		}
	}

	return seq, next
}

func fixMatchPos(actions []recursiveLookup, remove []int, insertPos, numInsert int) {
	if len(actions) == 0 {
		return
	}

	minPos := actions[0].a
	maxPos := actions[0].b
	for _, action := range actions[1:] {
		if action.a < minPos {
			minPos = action.a
		}
		if action.b > maxPos {
			maxPos = action.b
		}
	}

	newPos := make([]int, maxPos-minPos+1)
	for i := range newPos {
		newPos[i] = minPos + i
	}
	for l := len(remove) - 1; l >= 0; l-- {
		i := remove[l]
		if i < minPos {
			continue
		}
		if i < insertPos {
			panic("inconsistent insert position")
		}
		newPos[i-minPos] = -1
		for j := i + 1; j <= maxPos; j++ {
			newPos[j-minPos]--
		}
	}
	for i, pos := range newPos {
		if pos >= insertPos {
			newPos[i] = pos + numInsert
		}
	}

	// 0 1 2 3 4 5 6 7 8 9
	//
	// remove 3, 4:
	// 0 1 2 3 -1 4 5 6 7 8
	// 0 1 2 -1 -2 3 4 5 6 7
	// insert single glyph at 3:
	// 0 1 2 -1 -2 4 5 6 7
	//
	// 0 2 4 6 7 -> 0 2 () 5 6
}

func applyMatch(seq []font.Glyph, m *Match, pos int) []font.Glyph {
	matchPos := m.MatchPos

	oldLen := len(seq)
	oldTailPos := matchPos[len(matchPos)-1] + 1
	tailLen := oldLen - oldTailPos
	newLen := oldLen - len(matchPos) + len(m.Replace)
	newTailPos := newLen - tailLen

	var newText []rune
	for _, offs := range matchPos {
		newText = append(newText, seq[offs].Text...)
	}

	out := seq

	if newLen > oldLen {
		// In case the sequence got longer, move the tail out of the way first.
		out = append(out, make([]font.Glyph, newLen-oldLen)...)
		copy(out[newTailPos:], out[oldTailPos:])
	}

	// copy the ignored glyphs into position, just before the new tail
	removeListIdx := len(matchPos) - 1
	insertPos := newTailPos - 1
	for i := oldTailPos - 1; i >= pos; i-- {
		if removeListIdx >= 0 && matchPos[removeListIdx] == i {
			removeListIdx--
		} else {
			out[insertPos] = seq[i]
			insertPos--
		}
	}

	// copy the new glyphs into position
	if len(m.Replace) > 0 {
		copy(out[pos:], m.Replace)
		out[pos].Text = newText
	}

	if newLen < oldLen {
		// In case the sequence got shorter, move the tail to the new position now.
		copy(out[newTailPos:], out[oldTailPos:])
		out = out[:newLen]
	}
	return out
}

type recursiveLookup struct {
	a, b            int
	lookupListIndex LookupIndex
}

func extractActions(match *Match) []recursiveLookup {
	actions := make([]recursiveLookup, 0, len(match.Actions))
	for _, nested := range match.Actions {
		if int(nested.SequenceIndex) >= len(match.MatchPos) {
			continue
		}
		actionPos := match.MatchPos[nested.SequenceIndex]
		if actionPos >= match.Next {
			continue
		}
		actions = append(actions, recursiveLookup{
			a:               actionPos,
			b:               match.Next,
			lookupListIndex: nested.LookupListIndex,
		})
	}
	return actions
}

// FindLookups returns the lookups required to implement the given
// features in the specified locale.
func (info *Info) FindLookups(loc *locale.Locale, includeFeature map[string]bool) []LookupIndex {
	if info == nil || len(info.ScriptList) == 0 {
		return nil
	}

	candidates := []ScriptLang{
		{Script: locale.ScriptUndefined, Lang: locale.LangUndefined},
	}
	if loc.Script != locale.ScriptUndefined {
		candidates = append(candidates,
			ScriptLang{Script: loc.Script, Lang: locale.LangUndefined})
	}
	if loc.Language != locale.LangUndefined {
		candidates = append(candidates,
			ScriptLang{Script: locale.ScriptUndefined, Lang: loc.Language})
	}
	if len(candidates) == 3 { // both are defined
		candidates = append(candidates,
			ScriptLang{Script: loc.Script, Lang: loc.Language})
	}
	var features *Features
	for _, cand := range candidates {
		f, ok := info.ScriptList[cand]
		if ok {
			features = f
			break
		}
	}
	if features == nil {
		return nil
	}

	includeLookup := make(map[LookupIndex]bool)
	numFeatures := FeatureIndex(len(info.FeatureList))
	if features.Required < numFeatures {
		feature := info.FeatureList[features.Required]
		for _, l := range feature.Lookups {
			includeLookup[l] = true
		}
	}
	for _, f := range features.Optional {
		if f >= numFeatures {
			continue
		}
		feature := info.FeatureList[f]
		if !includeFeature[feature.Tag] {
			continue
		}
		for _, l := range feature.Lookups {
			includeLookup[l] = true
		}
	}

	numLookups := LookupIndex(len(info.LookupList))
	var ll []LookupIndex
	for l := range includeLookup {
		if l >= numLookups {
			continue
		}
		ll = append(ll, l)
	}
	sort.Slice(ll, func(i, j int) bool {
		return ll[i] < ll[j]
	})
	return ll
}
