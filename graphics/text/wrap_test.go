// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package text

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
)

func TestWrap(t *testing.T) {
	type testCase struct {
		name string
		in   []string
		out  [][]string
	}
	cases := []testCase{
		{
			name: "single word",
			in:   []string{"word"},
			out:  [][]string{{"word"}},
		},
		{
			name: "single input",
			in:   []string{"a b c"},
			out:  [][]string{{"a", "b", "c"}},
		},
		{
			name: "two paragraphs",
			in:   []string{"a b\nc d"},
			out:  [][]string{{"a", "b"}, {"c", "d"}},
		},
		{
			name: "trailing newline",
			in:   []string{"a b\nc d\n"},
			out:  [][]string{{"a", "b"}, {"c", "d"}},
		},
		{
			name: "empty paragraph",
			in:   []string{"a b\n\nc d"},
			out:  [][]string{{"a", "b"}, {}, {"c", "d"}},
		},
		{
			name: "several inputs",
			in:   []string{"a b", "c d"},
			out:  [][]string{{"a", "b", "c", "d"}},
		},
		{
			name: "many inputs",
			in:   []string{"a", "b", "c", "d"},
			out:  [][]string{{"a", "b", "c", "d"}},
		},
		{
			name: "complex",
			in:   []string{"a b", "c\nd e"},
			out:  [][]string{{"a", "b", "c"}, {"d", "e"}},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			w := Wrap(100, testCase.in...)
			if d := cmp.Diff(testCase.out, w.words); d != "" {
				t.Errorf("Wrap() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestWrapOverfull(t *testing.T) {
	w := Wrap(0, "a\nb")
	F := font.Must(standard.Helvetica.New())
	res := slices.Collect(w.Lines(F, 10))
	if len(res) != 2 {
		t.Fatal("expected two lines, got", len(res))
	}

	for i := range 2 {
		if len(res[i].Seq) != 1 {
			t.Errorf("expected one glyphs in line %d, got %d", i, len(res[i].Seq))
		}
	}
}

// fixedBreaker reports the same break opportunities regardless of its input.
// It is used to test WrapWith with breakers other than WhitespaceBreaker.
type fixedBreaker struct {
	opps []BreakOpportunity
}

func (b fixedBreaker) Opportunities(s string) []BreakOpportunity {
	return b.opps
}

func TestWrapWithHyphenation(t *testing.T) {
	// "photograph" has no internal whitespace; only a custom breaker can
	// split it, at the boundary between "photo" and "graph".
	breaker := fixedBreaker{opps: []BreakOpportunity{{Start: 5, End: 5, Replace: "-"}}}

	w := WrapWith(breaker, 0, "photograph")
	F := font.Must(standard.Helvetica.New())
	lines := slices.Collect(w.Lines(F, 10))
	if len(lines) != 2 {
		t.Fatalf("expected two lines, got %d", len(lines))
	}

	text := func(seq *font.GlyphSeq) string {
		var s string
		for _, g := range seq.Seq {
			s += g.Text
		}
		return s
	}

	if got, want := text(lines[0]), "photo-"; got != want {
		t.Errorf("line 0 = %q, want %q", got, want)
	}
	if got, want := text(lines[1]), "graph"; got != want {
		t.Errorf("line 1 = %q, want %q", got, want)
	}
}

func TestWrapRanges(t *testing.T) {
	w := Wrap(0, "a\nb")
	F := font.Must(standard.Helvetica.New())
	ranges := slices.Collect(w.Ranges(F, 10))

	want := []LineRange{
		{Start: 0, End: 1},
		{Start: 2, End: 3},
	}
	if d := cmp.Diff(want, ranges); d != "" {
		t.Errorf("Ranges() mismatch (-want +got):\n%s", d)
	}
}

func TestWrapRangesWords(t *testing.T) {
	// width chosen so that "a b c" (Helvetica 10pt) breaks after "b".
	F := font.Must(standard.Helvetica.New())
	width := F.Layout(nil, 10, "a b").TotalWidth() + 1
	w := Wrap(width, "a b c")
	ranges := slices.Collect(w.Ranges(F, 10))

	want := []LineRange{
		{Start: 0, End: 3}, // "a b"
		{Start: 4, End: 5}, // "c"
	}
	if d := cmp.Diff(want, ranges); d != "" {
		t.Errorf("Ranges() mismatch (-want +got):\n%s", d)
	}
}

func TestWrapRangesHyphenation(t *testing.T) {
	breaker := fixedBreaker{opps: []BreakOpportunity{{Start: 5, End: 5, Replace: "-"}}}

	w := WrapWith(breaker, 0, "photograph")
	F := font.Must(standard.Helvetica.New())
	ranges := slices.Collect(w.Ranges(F, 10))

	want := []LineRange{
		{Start: 0, End: 5, Hyphen: true},   // "photo"
		{Start: 5, End: 10, Hyphen: false}, // "graph"
	}
	if d := cmp.Diff(want, ranges); d != "" {
		t.Errorf("Ranges() mismatch (-want +got):\n%s", d)
	}
}

func TestWrapRangesEmptyParagraph(t *testing.T) {
	w := Wrap(1000, "a b\n\nc d")
	F := font.Must(standard.Helvetica.New())
	ranges := slices.Collect(w.Ranges(F, 10))

	want := []LineRange{
		{Start: 0, End: 3}, // "a b"
		{Start: 4, End: 4}, // empty line between the two newlines
		{Start: 5, End: 8}, // "c d"
	}
	if d := cmp.Diff(want, ranges); d != "" {
		t.Errorf("Ranges() mismatch (-want +got):\n%s", d)
	}
}

func TestWrapWithOffsetMidRune(t *testing.T) {
	// "aébc": 'é' is byte offset 1-3, so offset 2 falls inside the rune.
	breaker := fixedBreaker{opps: []BreakOpportunity{{Start: 2, End: 2, Replace: "-"}}}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic for a break offset inside a rune")
		}
	}()
	WrapWith(breaker, 100, "aébc")
}
