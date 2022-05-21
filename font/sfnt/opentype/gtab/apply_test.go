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
				MatchPos: []int{0},
				Replace: []font.Glyph{
					{Gid: 100},
				},
			},
			out: []font.GlyphID{100, 1, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				MatchPos: []int{0, 1},
				Replace: []font.Glyph{
					{Gid: 100},
				},
			},
			out: []font.GlyphID{100, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				MatchPos: []int{0, 1, 2},
				Replace: []font.Glyph{
					{Gid: 100},
				},
			},
			out: []font.GlyphID{100, 3, 4, 5, 6},
		},
		{
			m: &Match{
				MatchPos: []int{0, 2, 4},
				Replace: []font.Glyph{
					{Gid: 100},
				},
			},
			out: []font.GlyphID{100, 1, 3, 5, 6},
		},
		{
			m: &Match{
				MatchPos: []int{1},
				Replace: []font.Glyph{
					{Gid: 100},
				},
			},
			out: []font.GlyphID{100, 0, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				MatchPos: []int{1, 2},
				Replace: []font.Glyph{
					{Gid: 100},
				},
			},
			out: []font.GlyphID{100, 0, 3, 4, 5, 6},
		},
		{
			m: &Match{
				MatchPos: []int{0},
				Replace: []font.Glyph{
					{Gid: 100},
					{Gid: 101},
				},
			},
			out: []font.GlyphID{100, 101, 1, 2, 3, 4, 5, 6},
		},
		{
			m: &Match{
				MatchPos: []int{0},
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
				MatchPos: []int{1, 5},
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
