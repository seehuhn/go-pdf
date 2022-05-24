package gtab

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font"
)

func TestApplyMatch(t *testing.T) {
	cases := []struct {
		m   *Match
		out []font.GlyphID
	}{
		{
			m: &Match{
				InputPos: []int{0},
				Replace: []font.Glyph{
					{Gid: 100},
				},
			},
			out: []font.GlyphID{100, 1, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{0, 1},
				Replace: []font.Glyph{
					{Gid: 100},
				},
			},
			out: []font.GlyphID{100, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{0, 1, 2},
				Replace: []font.Glyph{
					{Gid: 100},
				},
			},
			out: []font.GlyphID{100, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{0, 2, 4},
				Replace: []font.Glyph{
					{Gid: 100},
				},
			},
			out: []font.GlyphID{100, 1, 3, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{1},
				Replace: []font.Glyph{
					{Gid: 100},
				},
			},
			out: []font.GlyphID{100, 0, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{1, 2},
				Replace: []font.Glyph{
					{Gid: 100},
				},
			},
			out: []font.GlyphID{100, 0, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{0},
				Replace: []font.Glyph{
					{Gid: 100},
					{Gid: 101},
				},
			},
			out: []font.GlyphID{100, 101, 1, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{0},
				Replace: []font.Glyph{
					{Gid: 100},
					{Gid: 101},
					{Gid: 102},
				},
			},
			out: []font.GlyphID{100, 101, 102, 1, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				InputPos: []int{1, 5},
				Replace: []font.Glyph{
					{Gid: 100},
					{Gid: 101},
					{Gid: 102},
				},
			},
			out: []font.GlyphID{100, 101, 102, 0, 2, 3, 4, 6},
		},
	}

	for i, test := range cases {
		t.Run(fmt.Sprintf("%02d", i+1), func(t *testing.T) {

			seq := make([]font.Glyph, 7)
			for i := range seq {
				seq[i].Gid = font.GlyphID(i)
			}
			seq = applyMatch(seq, test.m, 0)
			out := make([]font.GlyphID, len(seq))
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
		{ // *******************************
			in:        []int{1, 2, 4},
			remove:    []int{0},
			numInsert: 1,
			out:       []int{0, 1, 2, 4},
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
		{
			in:        []int{1, 2, 4},
			remove:    []int{3},
			numInsert: 1,
			out:       []int{1, 2, 3, 4},
		},
		{
			in:        []int{1, 2, 4},
			remove:    []int{4},
			numInsert: 1,
			out:       []int{1, 2, 4},
		},
		{
			in:        []int{1, 2, 4},
			remove:    []int{5},
			numInsert: 1,
			out:       []int{1, 2, 4, 5},
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
					Keep:     keepAllGlyphs,
				},
			}
			fixMatchPos(actions, test.remove, make([]font.Glyph, test.numInsert))
			if d := cmp.Diff(test.out, actions[0].InputPos); d != "" {
				t.Errorf("%d: %s", i, d)
			}
		}
	}
}
