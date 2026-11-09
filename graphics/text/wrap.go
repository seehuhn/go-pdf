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
	"fmt"
	"iter"
	"strings"
	"unicode"
	"unicode/utf8"

	"seehuhn.de/go/pdf/font"
)

// A LineBreaker reports where a run of text may be broken into lines.
type LineBreaker interface {
	// Opportunities returns the break opportunities in s, in order.
	Opportunities(s string) []BreakOpportunity
}

// BreakOpportunity is a position at which a line may end.
//
// Offsets are byte offsets into the string passed to
// [LineBreaker.Opportunities], on rune boundaries.  A whitespace break
// covers the run of spaces to remove, with Replace empty; a hyphenation
// point has Start == End (no text is removed) and Replace "-".
type BreakOpportunity struct {
	Start, End int    // bytes removed if the line breaks here, e.g. whitespace
	Replace    string // text ending the line if it breaks here, "-" for a hyphen
}

// WhitespaceBreaker breaks after runs of whitespace.  It is the default line
// breaker used by [Wrap].
//
// Whitespace is as defined by [unicode.IsSpace], except for the no-break
// spaces U+00A0, U+2007 and U+202F, which are never break opportunities and
// remain part of the surrounding word.
type WhitespaceBreaker struct{}

// isBreakSpace reports whether r is whitespace at which a line may break.
func isBreakSpace(r rune) bool {
	switch r {
	case 0xa0, 0x2007, 0x202f: // no-break spaces
		return false
	}
	return unicode.IsSpace(r)
}

// Opportunities implements the [LineBreaker] interface.
func (WhitespaceBreaker) Opportunities(s string) []BreakOpportunity {
	var res []BreakOpportunity
	start := -1
	for i, r := range s {
		if isBreakSpace(r) {
			if start < 0 {
				start = i
			}
		} else if start >= 0 {
			res = append(res, BreakOpportunity{Start: start, End: i})
			start = -1
		}
	}
	if start >= 0 {
		res = append(res, BreakOpportunity{Start: start, End: len(s)})
	}
	return res
}

// gap describes how a paragraph's words are joined when line breaking.
type gap struct {
	// keep is the text used to join two words when the line does not break
	// between them.
	keep string

	// replace is the text appended at the end of the line when it breaks
	// between the two words.
	replace string
}

// span is the byte range of a word within the original text passed to
// [WrapWith] (the concatenation of its variadic text arguments).
type span struct {
	start, end int
}

type wrap struct {
	width float64
	words [][]string
	gaps  [][]gap
	spans [][]span

	// base is, for a paragraph with no words, the byte offset at which the
	// (empty) paragraph occurred; unused for non-empty paragraphs, whose
	// range comes from spans instead.
	base []int
}

// Wrap creates a text wrapper that breaks text at the specified width, using
// [WhitespaceBreaker].
// Whitespace at the beginning of the first text and the end of the last text is ignored.
// Line breaks in the input strings start a new line, see [WrapWith].
// Adjacent text arguments are joined by a single space, so Wrap(w, "foo", "bar")
// is equivalent to Wrap(w, "foo bar") unless the first argument ends in a line
// break.
func Wrap(width float64, text ...string) *wrap {
	return WrapWith(WhitespaceBreaker{}, width, text...)
}

// WrapWith creates a text wrapper that breaks text at the specified width,
// using b to find break opportunities within each line of input.
// Whitespace at the beginning of the first text and the end of the last text is ignored.
// The mandatory line breaks of Unicode UAX #14 in the input strings start a
// new line, regardless of b: CR LF, CR, LF, NEL (U+0085), VT, FF, LINE
// SEPARATOR (U+2028) and PARAGRAPH SEPARATOR (U+2029).  Adjacent text
// arguments are joined by a single space.
func WrapWith(b LineBreaker, width float64, text ...string) *wrap {
	var allWords [][]string
	var allGaps [][]gap
	var allSpans [][]span
	var allBase []int
	var currentWords []string
	var currentGaps []gap
	var currentSpans []span
	var currentBase int

	argBase := 0
	for i, orig := range text {
		t := orig
		trimStart := 0
		if i == 0 {
			trimmed := strings.TrimLeftFunc(t, isBreakSpace)
			trimStart = len(t) - len(trimmed)
			t = trimmed
		}
		if i == len(text)-1 {
			t = strings.TrimRightFunc(t, isBreakSpace)
		}

		first := true
		localOffset := 0
		for part, breakLen := range lineParts(t) {
			base := argBase + trimStart + localOffset
			words, gaps, spans := tokenize(b, part, base)
			if first {
				if len(currentWords) == 0 {
					currentBase = base
				}
				if len(currentWords) > 0 && len(words) > 0 {
					currentGaps = append(currentGaps, gap{keep: " "})
				}
				currentWords = append(currentWords, words...)
				currentGaps = append(currentGaps, gaps...)
				currentSpans = append(currentSpans, spans...)
				first = false
			} else {
				allWords = append(allWords, currentWords)
				allGaps = append(allGaps, currentGaps)
				allSpans = append(allSpans, currentSpans)
				allBase = append(allBase, currentBase)
				currentWords = words
				currentGaps = gaps
				currentSpans = spans
				currentBase = base
			}
			localOffset += len(part) + breakLen
		}
		argBase += len(orig)
	}
	if currentWords != nil {
		allWords = append(allWords, currentWords)
		allGaps = append(allGaps, currentGaps)
		allSpans = append(allSpans, currentSpans)
		allBase = append(allBase, currentBase)
	}

	return &wrap{width: width, words: allWords, gaps: allGaps, spans: allSpans, base: allBase}
}

// lineParts splits s at the mandatory line breaks listed for [WrapWith].
// It yields each line together with the byte length of the line break which
// ends it, or 0 for the last line.
func lineParts(s string) iter.Seq2[string, int] {
	return func(yield func(string, int) bool) {
		start := 0
		for i := 0; i < len(s); {
			r, size := utf8.DecodeRuneInString(s[i:])
			switch r {
			case '\r':
				if strings.HasPrefix(s[i+1:], "\n") {
					size = 2
				}
			case '\n', '\v', '\f', '\u0085', '\u2028', '\u2029':
			default:
				i += size
				continue
			}
			if !yield(s[start:i], size) {
				return
			}
			i += size
			start = i
		}
		yield(s[start:], 0)
	}
}

// tokenize splits s into words at the break opportunities b reports, along
// with the gap information needed to rejoin them and the byte span of each
// word relative to the start of the text passed to [WrapWith] (base is the
// offset of s itself within that text).  It panics if b reports an
// opportunity outside s or not on a rune boundary.
func tokenize(b LineBreaker, s string, base int) ([]string, []gap, []span) {
	opps := b.Opportunities(s)
	if len(opps) == 0 {
		if s == "" {
			return []string{}, nil, nil
		}
		return []string{s}, nil, []span{{base, base + len(s)}}
	}

	words := make([]string, 0, len(opps)+1)
	gaps := make([]gap, 0, len(opps))
	spans := make([]span, 0, len(opps)+1)
	pos := 0
	for _, o := range opps {
		checkOffset(s, o.Start)
		checkOffset(s, o.End)
		if o.Start < pos || o.End < o.Start {
			panic(fmt.Sprintf("text: break opportunities out of order in %q", s))
		}
		words = append(words, s[pos:o.Start])
		spans = append(spans, span{base + pos, base + o.Start})
		keep := ""
		if o.End > o.Start {
			keep = " "
		}
		gaps = append(gaps, gap{keep: keep, replace: o.Replace})
		pos = o.End
	}
	words = append(words, s[pos:])
	spans = append(spans, span{base + pos, base + len(s)})

	// drop empty words at either end, together with the gap next to them
	if len(words) > 1 && words[0] == "" {
		words = words[1:]
		gaps = gaps[1:]
		spans = spans[1:]
	}
	if len(words) > 1 && words[len(words)-1] == "" {
		words = words[:len(words)-1]
		gaps = gaps[:len(gaps)-1]
		spans = spans[:len(spans)-1]
	}
	if len(words) == 1 && words[0] == "" {
		return []string{}, nil, nil
	}
	return words, gaps, spans
}

// checkOffset panics unless i is a valid rune boundary within s (including
// the two ends of the string).
func checkOffset(s string, i int) {
	if i < 0 || i > len(s) {
		panic(fmt.Sprintf("text: break offset %d out of range for %q", i, s))
	}
	if i > 0 && i < len(s) && !utf8.RuneStart(s[i]) {
		panic(fmt.Sprintf("text: break offset %d is not a rune boundary in %q", i, s))
	}
}

// lineBreak is one line decided by [wrap.breakLines]: the line consists of
// words[startWord..gapIdx] joined by their gaps, followed by the Replace text
// of gap gapIdx, or gapIdx == -1 for the paragraph's final line, which
// consumes no gap and extends to the last word.  glyphs is the line laid out on
// its own.
type lineBreak struct {
	startWord, gapIdx int
	glyphs            []font.Glyph
}

// totalAdvance returns the sum of the advances of glyphs.
func totalAdvance(glyphs []font.Glyph) float64 {
	var sum float64
	for _, g := range glyphs {
		sum += g.Advance
	}
	return sum
}

// breakLines greedily decides where to end each line of one paragraph.  Every
// line is measured by laying out its text on its own, exactly as it will be
// drawn, so kerning and ligatures across the removed gap text are accounted
// for.  A line ends at the last candidate gap whose line, including that
// gap's Replace text, fits into w.width, where the candidates considered are
// those up to the first word end at which the line without Replace text is
// already too wide.  If no candidate fits, the line extends to the end of the
// first word (the next gap without Replace text) and overflows; wrapping
// continues after it.
//
// To keep the cost near linear in the paragraph length, each word is laid
// out once and prefix sums of the advances propose a break by binary search.
// The proposal is then corrected using standalone layouts of the candidate
// lines: it moves left while the line is too wide and right while the next
// longer line still fits.  The result is the one an exhaustive scan would
// give, provided line width does not decrease when words are added; the
// number of standalone layouts per line is one plus the proposal's error.
//
// [wrap.Lines] and [wrap.Ranges] both call this, so they always agree on
// where lines break.  The paragraph must contain at least one word.
func (w *wrap) breakLines(F font.Layouter, ptSize float64, words []string, gaps []gap) []lineBreak {
	n := len(words)

	// prefix[i] is the estimated width of words[:i], each followed by its gap
	keepWidth := make(map[string]float64)
	prefix := make([]float64, n+1)
	for i, word := range words {
		prefix[i+1] = prefix[i] + totalAdvance(F.Layout(nil, ptSize, word).Seq)
		if i < n-1 && gaps[i].keep != "" {
			kw, ok := keepWidth[gaps[i].keep]
			if !ok {
				kw = totalAdvance(F.Layout(nil, ptSize, gaps[i].keep).Seq)
				keepWidth[gaps[i].keep] = kw
			}
			prefix[i+1] += kw
		}
	}

	var breaks []lineBreak
	for s := 0; ; {
		base := make(map[int][]font.Glyph) // line s..k without Replace text
		layoutBase := func(k int) []font.Glyph {
			g, ok := base[k]
			if !ok {
				g = F.Layout(nil, ptSize, joinWords(words[s:k+1], gaps[s:])).Seq
				base[k] = g
			}
			return g
		}
		// layoutCand returns the line s..k as it is drawn if it breaks at gap k
		layoutCand := func(k int) []font.Glyph {
			if k == n-1 || gaps[k].replace == "" {
				return layoutBase(k)
			}
			return F.Layout(nil, ptSize, joinWords(words[s:k+1], gaps[s:])+gaps[k].replace).Seq
		}

		// proposal from the prefix sums
		lo, hi := s, n-1
		for lo < hi {
			mid := (lo + hi + 1) / 2
			e := prefix[mid+1] - prefix[s]
			if mid < n-1 {
				e -= keepWidth[gaps[mid].keep]
			}
			if e <= w.width {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		k := lo

		for k > s && totalAdvance(layoutBase(k)) > w.width {
			k--
		}
		for k < n-1 && totalAdvance(layoutBase(k+1)) <= w.width {
			k++
		}
		var glyphs []font.Glyph
		for ; k >= s; k-- {
			glyphs = layoutCand(k)
			if totalAdvance(glyphs) <= w.width {
				break
			}
		}

		if k < s {
			// nothing fits: overflow to the end of the first word
			k = s
			for k < n-1 && gaps[k].replace != "" {
				k++
			}
			glyphs = layoutBase(k)
		}
		if k == n-1 {
			breaks = append(breaks, lineBreak{startWord: s, gapIdx: -1, glyphs: glyphs})
			return breaks
		}
		breaks = append(breaks, lineBreak{startWord: s, gapIdx: k, glyphs: glyphs})
		s = k + 1
	}
}

// joinWords joins words using the keep text of the gaps between them.
func joinWords(words []string, gaps []gap) string {
	var sb strings.Builder
	for i, word := range words {
		sb.WriteString(word)
		if i < len(words)-1 {
			sb.WriteString(gaps[i].keep)
		}
	}
	return sb.String()
}

// Lines arranges the text into lines.
// Breaks occur only at the break opportunities found by the wrapper's
// [LineBreaker] (which are spaces, for the default breaker).
// Lines are at most w.width wide, except when no candidate break lets a line
// end within that width -- e.g. a single word wider than w.width, or a word
// whose only break candidate's Replace text would push it over.  Such a line
// ends at the first following break opportunity without Replace text, so it
// overflows by at most one word (or word fragment); wrapping then continues
// after it, and later lines are again at most w.width wide.
func (w *wrap) Lines(F font.Layouter, ptSize float64) iter.Seq[*font.GlyphSeq] {
	return func(yield func(*font.GlyphSeq) bool) {
		for pIdx, words := range w.words {
			if len(words) == 0 {
				// empty paragraph, yield empty line
				if !yield(&font.GlyphSeq{}) {
					return
				}
				continue
			}
			gaps := w.gaps[pIdx]
			for _, lb := range w.breakLines(F, ptSize, words, gaps) {
				if !yield(&font.GlyphSeq{Seq: lb.glyphs}) {
					return
				}
			}
		}
	}
}

// LineRange is one line produced by [wrap.Ranges]: the byte range, into the
// text originally passed to [WrapWith], covered by the line, and whether the
// line ends with a hyphen ("-") inserted by the break rather than with a
// literal break already present in the text.
type LineRange struct {
	Start, End int
	Hyphen     bool
}

// Ranges reports the same line breaks as [wrap.Lines], as byte ranges into
// the text originally passed to [WrapWith] rather than as laid-out glyphs.
// In particular, overlong lines overflow as described there.
// It calls F the same way Lines does and so must choose exactly the same
// break points; a caller which needs both the glyphs and the ranges for the
// same text should call both, since the two do not share layout work.
//
// An empty paragraph (two consecutive newlines, or leading/trailing
// whitespace collapsed to nothing) is reported as a zero-length range at the
// byte offset where it occurred.
func (w *wrap) Ranges(F font.Layouter, ptSize float64) iter.Seq[LineRange] {
	return func(yield func(LineRange) bool) {
		for pIdx, words := range w.words {
			if len(words) == 0 {
				pos := w.base[pIdx]
				if !yield(LineRange{Start: pos, End: pos}) {
					return
				}
				continue
			}
			gaps := w.gaps[pIdx]
			spans := w.spans[pIdx]
			for _, lb := range w.breakLines(F, ptSize, words, gaps) {
				var lr LineRange
				if lb.gapIdx >= 0 {
					lr = LineRange{
						Start:  spans[lb.startWord].start,
						End:    spans[lb.gapIdx].end,
						Hyphen: gaps[lb.gapIdx].replace == "-",
					}
				} else {
					lr = LineRange{
						Start: spans[lb.startWord].start,
						End:   spans[len(words)-1].end,
					}
				}
				if !yield(lr) {
					return
				}
			}
		}
	}
}
