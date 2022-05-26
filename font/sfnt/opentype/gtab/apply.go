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
	"math"
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

// Match describes the effect of applying a Lookup to a glyph sequence.
type Match struct {
	InputPos []int // in increasing order
	Replace  []font.Glyph
	Actions  SeqLookups
	Next     int
}

type nested struct {
	InputPos []int
	Actions  SeqLookups
	EndPos   int
}

// applyLookupAt applies a single lookup to the given glyphs at position pos.
func (ll LookupList) applyLookupAt(seq []font.Glyph, lookupIndex LookupIndex, gdef *gdef.Table, pos, b int) ([]font.Glyph, int) {
	numLookups := len(ll)
	stack := []*nested{
		{
			InputPos: []int{pos},
			Actions: SeqLookups{
				{SequenceIndex: 0, LookupListIndex: lookupIndex},
			},
			EndPos: b,
		},
	}

	next := pos + 1
	nextUpdated := false

	numActions := 0
	for len(stack) > 0 && numActions < 64 {
		k := len(stack) - 1
		if len(stack[k].Actions) == 0 {
			stack = stack[:k]
			continue
		}

		numActions++

		lookupIndex := stack[k].Actions[0].LookupListIndex
		seqIdx := stack[k].Actions[0].SequenceIndex
		if int(seqIdx) >= len(stack[k].InputPos) {
			continue
		}
		pos := stack[k].InputPos[seqIdx]
		end := stack[k].EndPos
		stack[k].Actions = stack[k].Actions[1:]

		if int(lookupIndex) >= numLookups {
			continue
		}
		lookup := ll[lookupIndex]

		keep := MakeFilter(lookup.Meta, gdef)
		match := lookup.Subtables.Apply(keep, seq, pos, end)
		if match == nil {
			continue
		}

		if !nextUpdated {
			next = match.InputPos[len(match.InputPos)-1] + 1
			nextUpdated = true
		}

		if match.Actions == nil {
			seq = applyMatch(seq, match, pos)
			fixMatchPos(stack, match.InputPos, len(match.Replace))
			next += len(match.Replace) - len(match.InputPos)
		} else {
			if match.Replace != nil {
				panic("invalid match object")
			}
			stack = append(stack, &nested{
				InputPos: match.InputPos,
				Actions:  match.Actions,
				EndPos:   match.InputPos[len(match.InputPos)-1] + 1,
			})
		}
	}

	return seq, next
}

func fixMatchPos(actions []*nested, remove []int, numInsert int) {
	if len(actions) == 0 {
		return
	}

	minPos := math.MaxInt
	maxPos := math.MinInt
	for _, action := range actions {
		for _, pos := range action.InputPos {
			if pos < minPos {
				minPos = pos
			}
			if pos > maxPos {
				maxPos = pos
			}
		}
		if action.EndPos > maxPos {
			maxPos = action.EndPos
		}
	}

	insertPos := remove[0]
	lastRemoved := remove[len(remove)-1]

	newPos := make([]int, maxPos-minPos+1)
	for i := range newPos {
		newPos[i] = minPos + i
	}
	for l := len(remove) - 1; l >= 0; l-- {
		i := remove[l]
		if i < insertPos {
			panic("inconsistent insert position")
		}
		start := i + 1
		if i >= minPos {
			newPos[i-minPos] = -1
		} else {
			start = minPos
		}
		for j := start; j <= maxPos; j++ {
			newPos[j-minPos]--
		}
	}

	for _, action := range actions {
		numRemoved := 0
		for _, pos := range remove {
			if pos < action.EndPos {
				numRemoved++
			} else {
				break
			}
		}

		var out []int
		in := action.InputPos
		for len(in) > 0 && in[0] < insertPos {
			out = append(out, in[0])
			in = in[1:]
		}

		// Decide whether or not to add the new glyphs to the input glyph
		// sequence of this action. We try to imitate the behavior of the
		// Windows layout engine, but I failed to reverse engineer the rules
		// completely.  The rule we are using here is that we include the
		// new glyphs, if and only if one of the endpoints of the match
		// was included in the original action input sequence.
		addToInput := false
		if len(in) > 0 && in[0] == insertPos {
			// first matched glyph was present
			addToInput = true
		} else {
			// final matched glyph was present
			for i := 0; i < len(in); i++ {
				if in[i] == lastRemoved {
					addToInput = true
				}
				if in[i] >= lastRemoved {
					break
				}
			}
		}

		if addToInput {
			for j := 0; j < numInsert; j++ {
				out = append(out, insertPos+j)
			}
		}
		for _, pos := range in {
			pos = newPos[pos-minPos]
			if pos >= 0 {
				out = append(out, pos+numInsert)
			}
		}
		action.InputPos = out
		action.EndPos += numInsert - numRemoved
	}
}

func applyMatch(seq []font.Glyph, m *Match, pos int) []font.Glyph {
	matchPos := m.InputPos

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
