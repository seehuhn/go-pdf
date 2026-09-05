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

package fallback

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	gtext "seehuhn.de/go/pdf/graphics/text"
)

// Line is one line of text as [Style.Lines] would lay it out: a byte range
// into the string passed to Lines, plus whether the line ends with a hyphen
// ("-") inserted by the line breaker rather than with a break already
// present in the text.
type Line struct {
	Start, End int
	Hyphen     bool
}

// Lines reports how [Generator.AddAppearance] breaks text into lines for the
// content of a FreeText annotation, using the style's font (NewContentFont,
// or Helvetica where that is nil), the fixed font size used for FreeText
// content, and the style's LineBreaker (or [gtext.WhitespaceBreaker] where
// that is nil).
//
// Width is the space available for the text, in the same units and with the
// same meaning as the clipWidth computed for a FreeText appearance: the
// annotation's inner rectangle, narrowed by its border width and content
// padding on both sides.  A caller which asks about a FreeText annotation
// must derive width the same way to get the same lines back.
//
// The returned lines cover the whole of text, in order, with no gaps or
// overlaps: bytes consumed by whitespace at a line break, or replaced by a
// hyphen, belong to neither neighbouring line.
//
// An error is returned only if NewContentFont fails; the default font never
// does.
func (s *Style) Lines(text string, width float64) ([]Line, error) {
	F, err := s.linesFont()
	if err != nil {
		return nil, err
	}

	breaker := gtext.LineBreaker(gtext.WhitespaceBreaker{})
	if s.LineBreaker != nil {
		breaker = s.LineBreaker
	}

	wrapper := gtext.WrapWith(breaker, width, text)
	var lines []Line
	for r := range wrapper.Ranges(F, freeTextFontSize) {
		lines = append(lines, Line{Start: r.Start, End: r.End, Hyphen: r.Hyphen})
	}
	return lines, nil
}

// linesFont returns the font [Style.Lines] uses to lay out FreeText content.
// Unlike [Generator.ContentFont], it is not tied to a document: [Style.Lines]
// only measures text and does not embed the font in a file.
func (s *Style) linesFont() (font.Layouter, error) {
	if s.NewContentFont != nil {
		return s.NewContentFont()
	}
	return font.Must(standard.Helvetica.New()), nil
}
