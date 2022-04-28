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
	"sort"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
	"seehuhn.de/go/pdf/locale"
)

// ApplyLookup applies a single lookup to the given glyphs.
func (info *Info) ApplyLookup(glyphs []font.Glyph, lookupIndex LookupIndex, gdef *gdef.Table) []font.Glyph {
	pos := 0
	numLeft := len(glyphs)
	for pos < len(glyphs) {
		glyphs, pos = info.applyLookupAt(glyphs, lookupIndex, gdef, pos)
		newNumLeft := len(glyphs) - pos
		if newNumLeft >= numLeft {
			// panic("infinite loop")
			pos = len(glyphs) - numLeft + 1
		}
		numLeft = newNumLeft
	}

	return glyphs
}

// applyLookupAt applies a single lookup to the given glyphs at position pos.
func (info *Info) applyLookupAt(seq []font.Glyph, lookupIndex LookupIndex, gdef *gdef.Table, pos int) ([]font.Glyph, int) {
	numLookups := len(info.LookupList)

	lookup := info.LookupList[lookupIndex]
	keep := MakeFilter(lookup.Meta, gdef)
	newSeq, newPos, nested := lookup.Subtables.Apply(keep, seq, pos)
	if newPos < 0 {
		return seq, pos + 1
	}
	if len(nested) == 0 {
		return newSeq, newPos
	}

	orig := seq
	seq = newSeq
	next := newPos
	numActions := 1 // we count the original lookup as an action
	for len(nested) > 0 {
		if numActions >= 64 {
			return orig, pos + 1
		}
		numActions++

		// fmt.Println(next, seq)

		a := nested[0]
		nested = nested[1:]
		if int(a.SequenceIndex) < pos || int(a.SequenceIndex) >= next || int(a.LookupListIndex) >= numLookups {
			continue
		}

		lookup = info.LookupList[a.LookupListIndex]
		keep = MakeFilter(lookup.Meta, gdef)
		newSeq, newPos, n2 := lookup.Subtables.Apply(keep, seq, int(a.SequenceIndex))
		if newPos < 0 {
			continue
		}

		seq = newSeq
		if newPos > next {
			next = newPos
		}
		delta := len(newSeq) - len(seq)
		for _, a2 := range nested {
			if a2.SequenceIndex > a.SequenceIndex && int(a2.SequenceIndex) < newPos {
				continue
			} else if int(a2.SequenceIndex) >= newPos {
				a2.SequenceIndex += uint16(delta)
			}
			n2 = append(n2, a2)
		}
		nested = n2
	}
	return seq, next
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
