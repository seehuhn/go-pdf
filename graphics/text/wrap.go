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
	"iter"
	"strings"
	"unicode"

	"seehuhn.de/go/pdf/font"
)

type wrap struct {
	width float64
	words [][]string
}

// Wrap creates a text wrapper that breaks text at the specified width.
// Whitespace at the beginning of the first text and the end of the last text is ignored.
// Newline characters in the input strings start a new line.
func Wrap(width float64, text ...string) *wrap {
	var allWords [][]string
	var currentParagraph []string

	for i, t := range text {
		if i == 0 {
			t = strings.TrimLeftFunc(t, unicode.IsSpace)
		}
		if i == len(text)-1 {
			t = strings.TrimRightFunc(t, unicode.IsSpace)
		}

		first := true
		for part := range strings.SplitSeq(t, "\n") {
			words := strings.Fields(part)
			if first {
				currentParagraph = append(currentParagraph, words...)
				first = false
			} else {
				allWords = append(allWords, currentParagraph)
				currentParagraph = words
			}
		}
	}
	if currentParagraph != nil {
		allWords = append(allWords, currentParagraph)
	}

	return &wrap{width, allWords}
}

// Lines arranges the text into lines.
// Breaks occur only at spaces (which are then removed).
// Lines are at most w.width wide, except when a single word is wider than w.width.
func (w *wrap) Lines(F font.Layouter, ptSize float64) iter.Seq[*font.GlyphSeq] {
	return func(yield func(*font.GlyphSeq) bool) {
		for _, paragraph := range w.words {
			if len(paragraph) == 0 {
				// empty paragraph, yield empty line
				if !yield(&font.GlyphSeq{}) {
					return
				}
				continue
			}

			// join words with spaces for this paragraph
			paragraphText := strings.Join(paragraph, " ")
			glyphs := F.Layout(nil, ptSize, paragraphText)

			startPos := 0
			breakPos := 0
			currentWidth := 0.0
			for i, g := range glyphs.Seq {
				if g.Text == " " {
					breakPos = i
				}
				if currentWidth+g.Advance > w.width && breakPos > startPos {
					// emit a line
					if !yield(&font.GlyphSeq{Seq: glyphs.Seq[startPos:breakPos]}) {
						return
					}
					startPos = breakPos + 1
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
