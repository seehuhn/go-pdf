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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
)

type debugNestedLookup struct {
	matchPos []int
	actions  Nested
}

func (l *debugNestedLookup) Apply(_ KeepGlyphFn, seq []font.Glyph, i int) ([]font.Glyph, int, Nested) {
	if i != 0 {
		seq[i].Gid = 3
		return seq, i + 1, nil
	}
	next := l.matchPos[len(l.matchPos)-1] + 1
	nested := l.actions.substituteLocations(l.matchPos)
	return seq, next, nested
}

func (l *debugNestedLookup) EncodeLen() int {
	panic("unreachable")
}

func (l *debugNestedLookup) Encode() []byte {
	panic("unreachable")
}

// TestNestedSimple tests that the nested lookup works as expected
// when the nested lookups are single glyph substitutions.
func TestNestedSimple(t *testing.T) {
	type testCase struct {
		sequenceIndex []int
		out           []font.GlyphID
	}
	cases := []testCase{
		{[]int{0}, []font.GlyphID{2, 1, 1, 1, 1, 3, 3}},
		{[]int{1}, []font.GlyphID{1, 1, 2, 1, 1, 3, 3}},
		{[]int{2}, []font.GlyphID{1, 1, 1, 1, 2, 3, 3}},
		{[]int{3}, []font.GlyphID{1, 1, 1, 1, 1, 3, 3}},
		{[]int{1, 2}, []font.GlyphID{1, 1, 2, 1, 2, 3, 3}},
		{[]int{1, 3}, []font.GlyphID{1, 1, 2, 1, 1, 3, 3}},
	}
	for _, test := range cases {
		var nested Nested
		for _, seqenceIndex := range test.sequenceIndex {
			nested = append(nested, SeqLookup{
				SequenceIndex:   uint16(seqenceIndex),
				LookupListIndex: 1,
			})
		}
		info := &Info{
			LookupList: LookupList{
				{
					Meta: &LookupMetaInfo{},
					Subtables: []Subtable{
						&debugNestedLookup{
							matchPos: []int{0, 2, 4},
							actions:  nested,
						},
					},
				},
				{
					Meta: &LookupMetaInfo{
						LookupType: 1,
					},
					Subtables: []Subtable{
						&Gsub1_1{
							Cov:   coverage.Table{1: 0},
							Delta: 1,
						},
					},
				},
			},
		}
		seq := []font.Glyph{
			{Gid: 1}, {Gid: 1}, {Gid: 1}, {Gid: 1}, {Gid: 1}, {Gid: 1}, {Gid: 1},
		}
		seq = info.ApplyLookup(seq, 0, nil)
		var out []font.GlyphID
		for _, g := range seq {
			out = append(out, g.Gid)
		}
		if diff := cmp.Diff(test.out, out); diff != "" {
			t.Error(diff)
		}
	}
}

func TestSeqContext1(t *testing.T) {
	in := []font.Glyph{{Gid: 1}, {Gid: 2}, {Gid: 3}, {Gid: 4}, {Gid: 99}, {Gid: 5}}
	l := &SeqContext1{
		Cov: map[font.GlyphID]int{2: 0, 3: 1, 4: 2},
		Rules: [][]*SeqRule{
			{ // seq = 2, ...
				{In: []font.GlyphID{2}},
				{In: []font.GlyphID{3, 4, 6}},
				{In: []font.GlyphID{3, 4}},
				{In: []font.GlyphID{3, 4, 5}}, // does not match since it comes last
			},
			{ // seq = 3, ...
				{In: []font.GlyphID{3}},
				{In: []font.GlyphID{5}},
				{In: []font.GlyphID{4, 5, 6}},
			},
			{ // seq = 4, ...
				{In: []font.GlyphID{5, 6}},
				{In: []font.GlyphID{4}},
				{In: []font.GlyphID{5}},
			},
		},
	}
	keep := func(g font.GlyphID) bool { return g < 50 }

	cases := []struct {
		before, after int
	}{
		{0, -1},
		{1, 4}, // matches 2, 3, 4
		{2, -1},
		{3, 6}, // matches 4, [99,] 5
		{4, -1},
		{5, -1},
	}
	for _, test := range cases {
		_, next, _ := l.Apply(keep, in, test.before)
		if next != test.after {
			t.Errorf("Apply(%d) = %d, want %d", test.before, next, test.after)
		}
	}
}

func TestSeqContext2(t *testing.T) {
	in := []font.Glyph{{Gid: 1}, {Gid: 2}, {Gid: 3}, {Gid: 4}, {Gid: 99}, {Gid: 5}}
	l := &SeqContext2{
		Cov:     map[font.GlyphID]int{2: 0, 3: 1, 4: 2, 99: 3},
		Classes: classdef.Table{1: 1, 3: 1, 5: 1},
		Rules: [][]*ClassSequenceRule{
			{ // seq = class0, ...
				{Input: []uint16{1, 0}},
				{Input: []uint16{1}},
			},
			{ // seq = class1, ...
				{Input: []uint16{1}},
				{Input: []uint16{0, 1, 0}},
			},
		},
	}
	keep := func(g font.GlyphID) bool { return g < 50 }

	cases := []struct {
		before, after int
	}{
		{0, -1}, // not in coverage table
		{1, 4},  // matches class0, class1, class0
		{2, -1}, // no match for class1, class0, class1
		{3, 6},  // matches 4, [99,] 5
		{4, -1}, // keep returns false
		{5, -1}, // not in coverage table
	}
	for _, test := range cases {
		_, next, _ := l.Apply(keep, in, test.before)
		if next != test.after {
			t.Errorf("Apply(%d) = %d, want %d", test.before, next, test.after)
		}
	}
}

func TestSeqContext3(t *testing.T) {
	in := []font.Glyph{{Gid: 1}, {Gid: 2}, {Gid: 3}, {Gid: 4}, {Gid: 99}, {Gid: 5}}
	l := &SeqContext3{
		InputCov: []coverage.Table{
			{1: 0, 3: 1, 4: 2},
			{2: 0, 4: 1, 5: 2},
			{3: 0, 5: 1},
		},
	}
	keep := func(g font.GlyphID) bool { return g < 50 }

	cases := []struct {
		before, after int
	}{
		{0, 3}, // matches 1, 2, 3
		{1, -1},
		{2, 6}, // matches 3, 4, [99,] 5
		{3, -1},
		{4, -1},
		{5, -1},
	}
	for _, test := range cases {
		_, next, _ := l.Apply(keep, in, test.before)
		if next != test.after {
			t.Errorf("Apply(%d) = %d, want %d", test.before, next, test.after)
		}
	}
}

func TestChainedSeqContext1(t *testing.T) {
	in := []font.Glyph{
		{Gid: 1}, {Gid: 99}, {Gid: 2}, {Gid: 99}, {Gid: 3}, {Gid: 4}, {Gid: 99}, {Gid: 5},
	}
	l := &ChainedSeqContext1{
		Cov: map[font.GlyphID]int{2: 0, 3: 1, 4: 2},
		Rules: [][]*ChainedSeqRule{
			{ // seq = 2, ...
				{
					Input: []font.GlyphID{2},
				},
				{
					Input:     []font.GlyphID{3, 4},
					Lookahead: []font.GlyphID{99},
				},
				{
					Input:     []font.GlyphID{3, 4, 5},
					Backtrack: []font.GlyphID{2},
				},
			},
			{ // seq = 3, ...
				{
					Input:     []font.GlyphID{4},
					Lookahead: []font.GlyphID{5},
					Backtrack: []font.GlyphID{2, 1},
				},
			},
			{ // seq = 4, ...
			},
		},
	}
	keep := func(g font.GlyphID) bool { return g < 50 }

	cases := []struct {
		before, after int
	}{
		{0, -1},
		{1, -1},
		{2, -1},
		{3, -1},
		{4, 6}, // matches [1, 2,] 3, 4, [5]
	}
	for _, test := range cases {
		_, next, _ := l.Apply(keep, in, test.before)
		if next != test.after {
			t.Errorf("Apply(%d) = %d, want %d", test.before, next, test.after)
		}
	}
}

func FuzzSeqContext1(f *testing.F) {
	sub := &SeqContext1{}
	f.Add(sub.Encode())
	sub.Cov = coverage.Table{3: 0, 5: 1}
	sub.Rules = [][]*SeqRule{
		{},
		{},
	}
	f.Add(sub.Encode())
	sub.Rules = [][]*SeqRule{
		{
			{
				In: []font.GlyphID{4},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 1},
					{SequenceIndex: 1, LookupListIndex: 5},
					{SequenceIndex: 0, LookupListIndex: 4},
				},
			},
		},
		{
			{
				In: []font.GlyphID{6, 7},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 2},
				},
			},
			{
				In: []font.GlyphID{6},
				Actions: []SeqLookup{
					{SequenceIndex: 2, LookupListIndex: 1},
					{SequenceIndex: 1, LookupListIndex: 2},
					{SequenceIndex: 0, LookupListIndex: 3},
				},
			},
		},
	}
	f.Add(sub.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 1, 1, readSeqContext1, data)
	})
}

func FuzzSeqContext2(f *testing.F) {
	sub := &SeqContext2{}
	f.Add(sub.Encode())
	sub.Cov = coverage.Table{3: 0, 5: 1}
	sub.Rules = [][]*ClassSequenceRule{
		{},
		{},
	}
	f.Add(sub.Encode())
	sub.Rules = [][]*ClassSequenceRule{
		{
			{
				Input: []uint16{4},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 1},
					{SequenceIndex: 1, LookupListIndex: 5},
					{SequenceIndex: 0, LookupListIndex: 4},
				},
			},
		},
		{
			{
				Input: []uint16{6, 7},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 2},
				},
			},
			{
				Input: []uint16{6},
				Actions: []SeqLookup{
					{SequenceIndex: 2, LookupListIndex: 1},
					{SequenceIndex: 1, LookupListIndex: 2},
					{SequenceIndex: 0, LookupListIndex: 3},
				},
			},
		},
	}
	f.Add(sub.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 1, 2, readSeqContext2, data)
	})
}

func FuzzSeqContext3(f *testing.F) {
	sub := &SeqContext3{}
	f.Add(sub.Encode())
	sub.InputCov = append(sub.InputCov, coverage.Table{3: 0, 4: 1})
	sub.Actions = []SeqLookup{
		{SequenceIndex: 0, LookupListIndex: 1},
		{SequenceIndex: 1, LookupListIndex: 5},
		{SequenceIndex: 0, LookupListIndex: 4},
	}
	f.Add(sub.Encode())
	sub.InputCov = append(sub.InputCov, coverage.Table{1: 0, 3: 1, 5: 2})
	f.Add(sub.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 1, 3, readSeqContext3, data)
	})
}

func FuzzChainedSeqContext1(f *testing.F) {
	sub := &ChainedSeqContext1{}
	f.Add(sub.Encode())
	sub.Cov = coverage.Table{1: 0, 3: 1}
	sub.Rules = [][]*ChainedSeqRule{
		{
			{
				Backtrack: []font.GlyphID{},
				Input:     []font.GlyphID{1},
				Lookahead: []font.GlyphID{2, 3},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 1},
					{SequenceIndex: 0, LookupListIndex: 2},
				},
			},
			{
				Backtrack: []font.GlyphID{4, 5, 6},
				Input:     []font.GlyphID{7, 8},
				Lookahead: []font.GlyphID{9},
				Actions: []SeqLookup{
					{SequenceIndex: 1, LookupListIndex: 0},
				},
			},
			{
				Backtrack: []font.GlyphID{10, 11},
				Input:     []font.GlyphID{12},
				Lookahead: []font.GlyphID{},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 1000},
				},
			},
		},
		{
			{
				Backtrack: []font.GlyphID{},
				Input:     []font.GlyphID{13},
				Lookahead: []font.GlyphID{},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 1},
					{SequenceIndex: 0, LookupListIndex: 2},
					{SequenceIndex: 0, LookupListIndex: 3},
				},
			},
		},
	}
	f.Add(sub.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 2, 1, readChainedSeqContext1, data)
	})
}

func FuzzChainedSeqContext2(f *testing.F) {
	sub := &ChainedSeqContext2{}
	f.Add(sub.Encode())
	sub.Cov = coverage.Table{1: 0, 3: 1}
	sub.Backtrack = classdef.Table{2: 1, 3: 1, 4: 2}
	sub.Input = classdef.Table{3: 1, 4: 2}
	sub.Lookahead = classdef.Table{3: 1, 4: 2, 5: 2}
	sub.Rules = [][]*ChainedClassSeqRule{
		{
			{
				Backtrack: []uint16{},
				Input:     []uint16{1},
				Lookahead: []uint16{2, 3},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 1},
					{SequenceIndex: 0, LookupListIndex: 2},
				},
			},
			{
				Backtrack: []uint16{4, 5, 6},
				Input:     []uint16{7, 8},
				Lookahead: []uint16{9},
				Actions: []SeqLookup{
					{SequenceIndex: 1, LookupListIndex: 0},
				},
			},
			{
				Backtrack: []uint16{10, 11},
				Input:     []uint16{12},
				Lookahead: []uint16{},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 1000},
				},
			},
		},
		{
			{
				Backtrack: []uint16{},
				Input:     []uint16{13},
				Lookahead: []uint16{},
				Actions: []SeqLookup{
					{SequenceIndex: 0, LookupListIndex: 1},
					{SequenceIndex: 0, LookupListIndex: 2},
					{SequenceIndex: 0, LookupListIndex: 3},
				},
			},
		},
	}
	f.Add(sub.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 2, 2, readChainedSeqContext2, data)
	})
}

func FuzzChainedSeqContext3(f *testing.F) {
	sub := &ChainedSeqContext3{}
	f.Add(sub.Encode())
	sub.BacktrackCov = []coverage.Table{
		{1: 0, 3: 1},
	}
	sub.InputCov = []coverage.Table{
		{2: 0, 3: 1},
		{3: 0, 4: 1},
	}
	sub.LookaheadCov = []coverage.Table{
		{4: 0, 5: 1, 6: 2},
	}
	sub.Actions = []SeqLookup{
		{SequenceIndex: 0, LookupListIndex: 1},
		{SequenceIndex: 0, LookupListIndex: 2},
	}
	f.Add(sub.Encode())

	f.Fuzz(func(t *testing.T, data []byte) {
		doFuzz(t, 2, 3, readChainedSeqContext3, data)
	})
}
