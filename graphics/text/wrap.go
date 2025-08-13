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

	"seehuhn.de/go/pdf/font"
)

type wrap struct {
	width float64
	text  string
}

func Wrap(width float64, text ...string) *wrap {
	var words []string
	for _, t := range text {
		words = append(words, strings.Fields(t)...)
	}
	return &wrap{width, strings.Join(words, " ")}
}

// Lines arranges the text into lines.
// Breaks occur only at spaces (which are then removed).
// Lines are at most w.width wide, except when a single word is wider than w.width.
func (w *wrap) Lines(F font.Layouter, ptSize float64) iter.Seq[*font.GlyphSeq] {
	return func(yield func(*font.GlyphSeq) bool) {
		glyphs := F.Layout(nil, ptSize, w.text)

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
				currentWidth = 0
			}
			currentWidth += g.Advance
		}
		if startPos < len(glyphs.Seq) {
			yield(&font.GlyphSeq{Seq: glyphs.Seq[startPos:]})
		}
	}
}
