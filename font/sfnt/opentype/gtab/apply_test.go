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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font/glyph"
)

func TestApplyMatch(t *testing.T) {
	cases := []struct {
		m   *Match
		out []glyph.ID
	}{
		{
			m: &Match{
				InputPos: []int{0},
				Replace: []glyph.Info{
					{Gid: 100},
				},
			},
			out: []glyph.ID{100, 1, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{0, 1},
				Replace: []glyph.Info{
					{Gid: 100},
				},
			},
			out: []glyph.ID{100, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{0, 1, 2},
				Replace: []glyph.Info{
					{Gid: 100},
				},
			},
			out: []glyph.ID{100, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{0, 2, 4},
				Replace: []glyph.Info{
					{Gid: 100},
				},
			},
			out: []glyph.ID{100, 1, 3, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{1},
				Replace: []glyph.Info{
					{Gid: 100},
				},
			},
			out: []glyph.ID{100, 0, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{1, 2},
				Replace: []glyph.Info{
					{Gid: 100},
				},
			},
			out: []glyph.ID{100, 0, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{0},
				Replace: []glyph.Info{
					{Gid: 100},
					{Gid: 101},
				},
			},
			out: []glyph.ID{100, 101, 1, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{0},
				Replace: []glyph.Info{
					{Gid: 100},
					{Gid: 101},
					{Gid: 102},
				},
			},
			out: []glyph.ID{100, 101, 102, 1, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{1, 5},
				Replace: []glyph.Info{
					{Gid: 100},
					{Gid: 101},
					{Gid: 102},
				},
			},
			out: []glyph.ID{100, 101, 102, 0, 2, 3, 4, 6},
		},
	}

	for i, test := range cases {
		t.Run(fmt.Sprintf("%02d", i+1), func(t *testing.T) {
			seq := make([]glyph.Info, 7)
			for i := range seq {
				seq[i].Gid = glyph.ID(i)
			}
			seq = applyMatch(seq, test.m, 0)
			out := make([]glyph.ID, len(seq))
			for i, g := range seq {
				out[i] = g.Gid
			}
			if d := cmp.Diff(out, test.out); d != "" {
				t.Error(d)
			}
		})
	}
}

func TestFixMatchPos(t *testing.T) {
	cases := []struct {
		in        []int
		remove    []int
		numInsert int
		out       []int
	}{
		{ // common case: replace two glyphs with one
			in:        []int{1, 2},
			remove:    []int{1, 2},
			numInsert: 1,
			out:       []int{1},
		},
		{ // common case: replace one glyph with two
			in:        []int{1},
			remove:    []int{1},
			numInsert: 2,
			out:       []int{1, 2},
		},
		{ // replace two glyphs with one, with extra glyphs present at end
			in:        []int{1, 2, 4},
			remove:    []int{1, 2},
			numInsert: 1,
			out:       []int{1, 3},
		},
		{ // glyph 0 was not in input, so is not included in the output either
			in:        []int{1, 2, 4},
			remove:    []int{0},
			numInsert: 1,
			out:       []int{1, 2, 4},
		},
		{
			in:        []int{1, 2, 4},
			remove:    []int{1},
			numInsert: 1,
			out:       []int{1, 2, 4},
		},
		{
			in:        []int{1, 2, 4},
			remove:    []int{2},
			numInsert: 1,
			out:       []int{1, 2, 4},
		},
		{ // glyph 3 was not in input, so is not included in the output either
			in:        []int{1, 2, 4},
			remove:    []int{3},
			numInsert: 1,
			out:       []int{1, 2, 4},
		},
		{
			in:        []int{1, 2, 4},
			remove:    []int{4},
			numInsert: 1,
			out:       []int{1, 2, 4},
		},
		{ // glyph 5 was not in input, so is not included in the output either
			in:        []int{1, 2, 4},
			remove:    []int{5},
			numInsert: 1,
			out:       []int{1, 2, 4},
		},
	}
	for i, test := range cases {
		for _, endOffs := range []int{1, 10} {
			endPos := test.in[len(test.in)-1] + endOffs
			actions := []*nested{
				{
					InputPos: test.in,
					Actions:  []SeqLookup{},
					EndPos:   endPos,
				},
			}
			fixMatchPos(actions, test.remove, test.numInsert)
			if d := cmp.Diff(test.out, actions[0].InputPos); d != "" {
				t.Errorf("%d: %s", i, d)
			}
		}
	}
}
