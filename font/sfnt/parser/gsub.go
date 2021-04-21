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
	"strings"

	"seehuhn.de/go/pdf/font"
)

// http://adobe-type-tools.github.io/afdko/OpenTypeFeatureFileSpecification.html#7
// TODO(voss): read this carefully
// TODO(voss): implement required features
// TODO(voss): sort and deduplicate lookups

type GsubLookup interface {
	Replace(int, []font.GlyphID) (int, []font.GlyphID)
	explain(g *gTab, pfx string)
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
	fmt.Printf(pfx+"lookup type 2.1, flags=0x%04x\n", l.flags)
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
