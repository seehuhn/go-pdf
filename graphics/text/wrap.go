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

// WhitespaceBreaker breaks after runs of whitespace and at newlines.  It is
// the default line breaker used by [Wrap].
type WhitespaceBreaker struct{}

// Opportunities implements the [LineBreaker] interface.
func (WhitespaceBreaker) Opportunities(s string) []BreakOpportunity {
	var res []BreakOpportunity
	start := -1
	for i, r := range s {
		if unicode.IsSpace(r) {
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
// Newline characters in the input strings start a new line.
func Wrap(width float64, text ...string) *wrap {
	return WrapWith(WhitespaceBreaker{}, width, text...)
}

// WrapWith creates a text wrapper that breaks text at the specified width,
// using b to find break opportunities within each line of input.
// Whitespace at the beginning of the first text and the end of the last text is ignored.
// Newline characters in the input strings start a new line, regardless of b.
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
			trimmed := strings.TrimLeftFunc(t, unicode.IsSpace)
			trimStart = len(t) - len(trimmed)
			t = trimmed
		}
		if i == len(text)-1 {
			t = strings.TrimRightFunc(t, unicode.IsSpace)
		}

		first := true
		localOffset := 0
		for part := range strings.SplitSeq(t, "\n") {
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
			localOffset += len(part) + 1
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

	if words[0] == "" {
		words = words[1:]
		gaps = gaps[1:]
		spans = spans[1:]
	}
	if len(words) > 0 && words[len(words)-1] == "" {
		words = words[:len(words)-1]
		gaps = gaps[:len(gaps)-1]
		spans = spans[:len(spans)-1]
	}
	if len(words) == 0 {
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

// Lines arranges the text into lines.
// Breaks occur only at the break opportunities found by the wrapper's
// [LineBreaker] (which are spaces, for the default breaker).
// Lines are at most w.width wide, except when a single word is wider than w.width.
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

			var sb strings.Builder
			gapStart := make([]int, len(gaps))
			gapEnd := make([]int, len(gaps))
			for i, word := range words {
				sb.WriteString(word)
				if i < len(gaps) {
					gapStart[i] = sb.Len()
					sb.WriteString(gaps[i].keep)
					gapEnd[i] = sb.Len()
				}
			}
			paragraphText := sb.String()
			glyphs := F.Layout(nil, ptSize, paragraphText)

			// breakPos[k] is the glyph index at which a line ending at gap k
			// stops (exclusive); nextStart[k] is where the following line
			// starts.
			breakPos := make([]int, len(gaps))
			nextStart := make([]int, len(gaps))
			bi, ni := 0, 0
			pos := 0
			for i := 0; i <= len(glyphs.Seq); i++ {
				for bi < len(gaps) && pos >= gapStart[bi] {
					breakPos[bi] = i
					bi++
				}
				for ni < len(gaps) && pos >= gapEnd[ni] {
					nextStart[ni] = i
					ni++
				}
				if i < len(glyphs.Seq) {
					pos += len(glyphs.Seq[i].Text)
				}
			}

			startPos := 0
			useCand := -1
			nextCand := 0
			currentWidth := 0.0
			for i, g := range glyphs.Seq {
				for nextCand < len(gaps) && breakPos[nextCand] <= i {
					useCand = nextCand
					nextCand++
				}
				if currentWidth+g.Advance > w.width && useCand >= 0 && breakPos[useCand] > startPos {
					seq := glyphs.Seq[startPos:breakPos[useCand]]
					if replace := gaps[useCand].replace; replace != "" {
						extra := F.Layout(nil, ptSize, replace)
						combined := make([]font.Glyph, 0, len(seq)+len(extra.Seq))
						combined = append(combined, seq...)
						combined = append(combined, extra.Seq...)
						seq = combined
					}
					if !yield(&font.GlyphSeq{Seq: seq}) {
						return
					}
					startPos = nextStart[useCand]
					useCand = -1
					// recalculate width for the new line (glyphs from startPos to i)
					currentWidth = 0
					for j := startPos; j <= i; j++ {
						currentWidth += glyphs.Seq[j].Advance
					}
				} else {
					currentWidth += g.Advance
				}
			}
			// emit remaining text in this paragraph
			if startPos < len(glyphs.Seq) {
				if !yield(&font.GlyphSeq{Seq: glyphs.Seq[startPos:]}) {
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

			var sb strings.Builder
			gapStart := make([]int, len(gaps))
			gapEnd := make([]int, len(gaps))
			for i, word := range words {
				sb.WriteString(word)
				if i < len(gaps) {
					gapStart[i] = sb.Len()
					sb.WriteString(gaps[i].keep)
					gapEnd[i] = sb.Len()
				}
			}
			paragraphText := sb.String()
			glyphs := F.Layout(nil, ptSize, paragraphText)

			// breakPos[k] is the glyph index at which a line ending at gap k
			// stops (exclusive); nextStart[k] is where the following line
			// starts.
			breakPos := make([]int, len(gaps))
			nextStart := make([]int, len(gaps))
			bi, ni := 0, 0
			pos := 0
			for i := 0; i <= len(glyphs.Seq); i++ {
				for bi < len(gaps) && pos >= gapStart[bi] {
					breakPos[bi] = i
					bi++
				}
				for ni < len(gaps) && pos >= gapEnd[ni] {
					nextStart[ni] = i
					ni++
				}
				if i < len(glyphs.Seq) {
					pos += len(glyphs.Seq[i].Text)
				}
			}

			startPos := 0
			startWord := 0
			useCand := -1
			nextCand := 0
			currentWidth := 0.0
			for i, g := range glyphs.Seq {
				for nextCand < len(gaps) && breakPos[nextCand] <= i {
					useCand = nextCand
					nextCand++
				}
				if currentWidth+g.Advance > w.width && useCand >= 0 && breakPos[useCand] > startPos {
					lr := LineRange{
						Start:  spans[startWord].start,
						End:    spans[useCand].end,
						Hyphen: gaps[useCand].replace == "-",
					}
					if !yield(lr) {
						return
					}
					startWord = useCand + 1
					startPos = nextStart[useCand]
					useCand = -1
					// recalculate width for the new line (glyphs from startPos to i)
					currentWidth = 0
					for j := startPos; j <= i; j++ {
						currentWidth += glyphs.Seq[j].Advance
					}
				} else {
					currentWidth += g.Advance
				}
			}
			// emit remaining text in this paragraph
			if startPos < len(glyphs.Seq) {
				lr := LineRange{
					Start: spans[startWord].start,
					End:   spans[len(words)-1].end,
				}
				if !yield(lr) {
					return
				}
			}
		}
	}
}
