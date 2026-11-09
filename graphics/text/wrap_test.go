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
	"math"
	"slices"
	"strings"
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

	F := font.Must(standard.Helvetica.New())
	// choose a width just wide enough for "photo-", including the hyphen
	width := F.Layout(nil, 10, "photo-").TotalWidth() + 1
	w := WrapWith(breaker, width, "photograph")
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

	F := font.Must(standard.Helvetica.New())
	// choose a width just wide enough for "photo-", including the hyphen
	width := F.Layout(nil, 10, "photo-").TotalWidth() + 1
	w := WrapWith(breaker, width, "photograph")
	ranges := slices.Collect(w.Ranges(F, 10))

	want := []LineRange{
		{Start: 0, End: 5, Hyphen: true},   // "photo"
		{Start: 5, End: 10, Hyphen: false}, // "graph"
	}
	if d := cmp.Diff(want, ranges); d != "" {
		t.Errorf("Ranges() mismatch (-want +got):\n%s", d)
	}
}

// A break candidate that only fits without its Replace text is not a valid
// break: the review's reported defect had "photograph" break as "photo-"
// even though the hyphen made that line wider than the wrap width.
func TestWrapWithHyphenationTooNarrowForHyphen(t *testing.T) {
	breaker := fixedBreaker{opps: []BreakOpportunity{{Start: 5, End: 5, Replace: "-"}}}
	F := font.Must(standard.Helvetica.New())

	// wide enough for "photo" alone, but not for "photo-"
	width := F.Layout(nil, 10, "photo").TotalWidth() + 0.5

	w := WrapWith(breaker, width, "photograph")
	lines := slices.Collect(w.Lines(F, 10))
	if len(lines) != 1 {
		t.Fatalf("expected one (overlong) line, got %d", len(lines))
	}

	var got string
	for _, g := range lines[0].Seq {
		got += g.Text
	}
	if want := "photograph"; got != want {
		t.Errorf("line = %q, want %q", got, want)
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

func TestWrapBlankLine(t *testing.T) {
	F := font.Must(standard.Helvetica.New())
	for _, text := range []string{"a\n   \nb", "a\n \t \nb"} {
		w := Wrap(1000, text)
		if d := cmp.Diff([][]string{{"a"}, {}, {"b"}}, w.words); d != "" {
			t.Errorf("%q: words mismatch (-want +got):\n%s", text, d)
		}
		ranges := slices.Collect(w.Ranges(F, 10))
		want := []LineRange{
			{Start: 0, End: 1},
			{Start: 2, End: 2},
			{Start: len(text) - 1, End: len(text)},
		}
		if d := cmp.Diff(want, ranges); d != "" {
			t.Errorf("%q: ranges mismatch (-want +got):\n%s", text, d)
		}
		if n := len(slices.Collect(w.Lines(F, 10))); n != 3 {
			t.Errorf("%q: got %d lines, want 3", text, n)
		}
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

func TestWrapWithOutOfOrderOpportunities(t *testing.T) {
	// the second opportunity starts before the first one ends
	breaker := fixedBreaker{opps: []BreakOpportunity{
		{Start: 4, End: 4},
		{Start: 2, End: 2},
	}}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic for out-of-order break opportunities")
		}
	}()
	WrapWith(breaker, 100, "abcdef")
}

func TestWrapWithOverlappingOpportunities(t *testing.T) {
	// the second opportunity starts before the first one ends
	breaker := fixedBreaker{opps: []BreakOpportunity{
		{Start: 2, End: 4},
		{Start: 3, End: 5},
	}}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic for overlapping break opportunities")
		}
	}()
	WrapWith(breaker, 100, "abcdef")
}

// Lines must be measured as they are drawn, laid out on their own: Helvetica
// has kern pairs with the space (e.g. "period space", "space A"), which do not
// apply once the break space is removed.
func TestWrapLinesStandaloneWidth(t *testing.T) {
	F := font.Must(standard.Helvetica.New())
	const ptSize = 10
	text := "AWAY To Yo, Test. A Very Wavy day: AV. Why? We fly off. " +
		"Final office affair. P. Y. T. A. Test: Yo. Tavern, Way. " +
		"Waffle iron, flat floor. The quick brown fox."
	width := func(s string) float64 { return F.Layout(nil, ptSize, s).TotalWidth() }

	for width10 := 600; width10 < 3000; width10 += 7 {
		wd := float64(width10) / 10
		w := Wrap(wd, text)
		var ranges []LineRange
		for r := range w.Ranges(F, ptSize) {
			ranges = append(ranges, r)
		}
		lines := slices.Collect(w.Lines(F, ptSize))
		if len(lines) != len(ranges) {
			t.Fatalf("width %g: %d lines but %d ranges", wd, len(lines), len(ranges))
		}
		for i, r := range ranges {
			line := text[r.Start:r.End]
			lw := width(line)
			var drawn float64
			for _, g := range lines[i].Seq {
				drawn += g.Advance
			}
			if drawn != lw {
				t.Errorf("width %g: line %q drawn width %g != standalone %g", wd, line, drawn, lw)
			}
			if lw > wd && strings.Contains(line, " ") {
				t.Errorf("width %g: line %q too wide (%g)", wd, line, lw)
			}
			if i+1 < len(ranges) {
				next := text[r.End:ranges[i+1].End]
				// extended by the next word, the line must not fit
				nextWord, _, _ := strings.Cut(strings.TrimLeft(next, " "), " ")
				if ext := width(line + " " + nextWord); ext <= wd {
					t.Errorf("width %g: line %q broken too early (%g with %q)", wd, line, ext, nextWord)
				}
			}
		}
	}
}

// countingFont counts the layout calls and the bytes laid out.
type countingFont struct {
	font.Layouter
	calls, bytes int
}

func (c *countingFont) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	c.calls++
	c.bytes += len(s)
	return c.Layouter.Layout(seq, ptSize, s)
}

// midHyphenBreaker breaks at whitespace and allows hyphenation after the
// second byte of every word longer than four bytes.
type midHyphenBreaker struct{}

func (midHyphenBreaker) Opportunities(s string) []BreakOpportunity {
	res := WhitespaceBreaker{}.Opportunities(s)
	pos := 0
	for word := range strings.FieldsSeq(s) {
		i := pos + strings.Index(s[pos:], word)
		if len(word) > 4 {
			res = append(res, BreakOpportunity{Start: i + 2, End: i + 2, Replace: "-"})
		}
		pos = i + len(word)
	}
	slices.SortFunc(res, func(a, b BreakOpportunity) int { return a.Start - b.Start })
	return res
}

func TestWrapLayoutCostLinear(t *testing.T) {
	text := strings.Repeat("lorem ipsum dolor sit amet ", 4000)
	breakers := map[string]LineBreaker{
		"whitespace":  WhitespaceBreaker{},
		"hyphenation": midHyphenBreaker{},
	}
	for name, b := range breakers {
		for _, width := range []float64{math.Inf(1), 300, 100, 30} {
			F := &countingFont{Layouter: font.Must(standard.Helvetica.New())}
			n := 0
			for range WrapWith(b, width, text).Lines(F, 10) {
				n++
			}
			if math.IsInf(width, 1) && n != 1 {
				t.Errorf("%s: got %d lines, want 1", name, n)
			}
			// measured ratios are below 3.8; a quadratic cost would be far above
			if F.bytes > 4*len(text) {
				t.Errorf("%s, width %g: laid out %d bytes for %d bytes of text",
					name, width, F.bytes, len(text))
			}
		}
	}
}

// hyphenAtBreaker breaks at whitespace and allows hyphenation at the given
// offsets into the word starting with "Super".
type hyphenAtBreaker struct{ at []int }

func (b hyphenAtBreaker) Opportunities(s string) []BreakOpportunity {
	res := WhitespaceBreaker{}.Opportunities(s)
	if i := strings.Index(s, "Super"); i >= 0 {
		for _, a := range b.at {
			res = append(res, BreakOpportunity{Start: i + a, End: i + a, Replace: "-"})
		}
	}
	slices.SortFunc(res, func(x, y BreakOpportunity) int { return x.Start - y.Start })
	return res
}

func TestWrapOverlongWordContinues(t *testing.T) {
	F := font.Must(standard.Helvetica.New())
	const long = "Supercalifragilisticexpialidocious"
	text := "a b " + long + " c d e f g h i j"

	type testCase struct {
		name    string
		breaker LineBreaker
		width   float64
		want    []string
	}
	cases := []testCase{
		{
			name:    "plain",
			breaker: WhitespaceBreaker{},
			width:   40,
			want:    []string{"a b", long, "c d e f g", "h i j"},
		},
		{
			// the only hyphenation point is too far into the word to fit
			name:    "hyphen does not fit",
			breaker: hyphenAtBreaker{at: []int{20}},
			width:   40,
			want:    []string{"a b", long, "c d e f g", "h i j"},
		},
		{
			// the hyphenation point after "Super" fits on a line of its own
			name:    "hyphen fits",
			breaker: hyphenAtBreaker{at: []int{5, 20}},
			width:   40,
			want:    []string{"a b", "Super", "califragilisticexpialidocious", "c d e f g", "h i j"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for r := range WrapWith(c.breaker, c.width, text).Ranges(F, 10) {
				got = append(got, text[r.Start:r.End])
			}
			if d := cmp.Diff(c.want, got); d != "" {
				t.Errorf("ranges mismatch (-want +got):\n%s", d)
			}
		})
	}
}

// penaltyFont adds a fixed amount to the advance of strings containing both
// a space and another character, so that a line is wider than the sum of the
// widths of its words and the spaces between them.
type penaltyFont struct {
	font.Layouter
}

func (p penaltyFont) Layout(seq *font.GlyphSeq, ptSize float64, s string) *font.GlyphSeq {
	res := p.Layouter.Layout(seq, ptSize, s)
	if len(res.Seq) > 0 && len(strings.TrimSpace(s)) > 0 && strings.Contains(s, " ") {
		res.Seq[len(res.Seq)-1].Advance += 3
	}
	return res
}

// If a line is wider than the estimate from the individual words, the line
// must still be shortened until it fits.
func TestWrapEstimateTooNarrow(t *testing.T) {
	F := penaltyFont{font.Must(standard.Helvetica.New())}
	const ptSize = 10
	text := "lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod"

	for width10 := 300; width10 < 1500; width10 += 3 {
		width := float64(width10) / 10
		for line := range Wrap(width, text).Lines(F, ptSize) {
			if line.TotalWidth() <= width {
				continue
			}
			var s string
			for _, g := range line.Seq {
				s += g.Text
			}
			if strings.Contains(s, " ") {
				t.Errorf("width %g: line %q too wide (%g)", width, s, line.TotalWidth())
			}
		}
	}
}

func TestWhitespaceBreakerNoBreakSpaces(t *testing.T) {
	for _, sp := range []string{"\u00a0", "\u202f", "\u2007", "\u2060", "\ufeff"} {
		s := "a" + sp + "b"
		if got := (WhitespaceBreaker{}).Opportunities(s); len(got) != 0 {
			t.Errorf("%U: unexpected break opportunities %v", []rune(sp)[0], got)
		}
		w := Wrap(100, s)
		if d := cmp.Diff([][]string{{s}}, w.words); d != "" {
			t.Errorf("%U: words mismatch (-want +got):\n%s", []rune(sp)[0], d)
		}
	}

	// no-break spaces at the ends of the text are not trimmed
	w := Wrap(100, "\u00a0a b\u00a0")
	if d := cmp.Diff([][]string{{"\u00a0a", "b\u00a0"}}, w.words); d != "" {
		t.Errorf("words mismatch (-want +got):\n%s", d)
	}

	// an ordinary space next to a no-break space still breaks
	got := (WhitespaceBreaker{}).Opportunities("a\u00a0 b")
	want := []BreakOpportunity{{Start: 3, End: 4}}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("opportunities mismatch (-want +got):\n%s", d)
	}
}

func TestWrapLineEndings(t *testing.T) {
	w := Wrap(100, "a b\r\nc d\r\n")
	if d := cmp.Diff([][]string{{"a", "b"}, {"c", "d"}}, w.words); d != "" {
		t.Errorf("words mismatch (-want +got):\n%s", d)
	}
	F := font.Must(standard.Helvetica.New())
	ranges := slices.Collect(w.Ranges(F, 10))
	want := []LineRange{{Start: 0, End: 3}, {Start: 5, End: 8}}
	if d := cmp.Diff(want, ranges); d != "" {
		t.Errorf("ranges mismatch (-want +got):\n%s", d)
	}

	// every mandatory line break of UAX #14 starts a new line
	for _, lb := range []string{"\r", "\n", "\r\n", "\v", "\f", "\u0085", "\u2028", "\u2029"} {
		text := "a b" + lb + "c"
		w = Wrap(100, text)
		if d := cmp.Diff([][]string{{"a", "b"}, {"c"}}, w.words); d != "" {
			t.Errorf("%q: words mismatch (-want +got):\n%s", lb, d)
		}
		ranges := slices.Collect(w.Ranges(F, 10))
		want := []LineRange{{Start: 0, End: 3}, {Start: 3 + len(lb), End: len(text)}}
		if d := cmp.Diff(want, ranges); d != "" {
			t.Errorf("%q: ranges mismatch (-want +got):\n%s", lb, d)
		}
	}

	// CR LF is one line break, LF CR are two
	w = Wrap(100, "a\n\rb")
	if d := cmp.Diff([][]string{{"a"}, {}, {"b"}}, w.words); d != "" {
		t.Errorf("words mismatch (-want +got):\n%s", d)
	}
}

func TestLineParts(t *testing.T) {
	type part struct {
		line     string
		breakLen int
	}
	cases := []struct {
		in   string
		want []part
	}{
		{"", []part{{"", 0}}},
		{"abc", []part{{"abc", 0}}},
		{"a\r\nb", []part{{"a", 2}, {"b", 0}}},
		{"a\r\r\n", []part{{"a", 1}, {"", 2}, {"", 0}}},
		{"\u2028x\u0085", []part{{"", 3}, {"x", 2}, {"", 0}}},
		{"a\r", []part{{"a", 1}, {"", 0}}},
	}
	for _, c := range cases {
		var got []part
		for line, n := range lineParts(c.in) {
			got = append(got, part{line, n})
		}
		if d := cmp.Diff(c.want, got, cmp.AllowUnexported(part{})); d != "" {
			t.Errorf("%q: mismatch (-want +got):\n%s", c.in, d)
		}
	}

	// stopping early
	for line := range lineParts("a\nb\nc") {
		if line != "a" {
			t.Errorf("got %q after stopping", line)
		}
		break
	}
}

func TestWrapJoinsArguments(t *testing.T) {
	w := Wrap(100, "foo", "bar")
	if d := cmp.Diff([][]string{{"foo", "bar"}}, w.words); d != "" {
		t.Errorf("words mismatch (-want +got):\n%s", d)
	}
}
