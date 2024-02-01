// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package graphics

import (
	"errors"

	"seehuhn.de/go/pdf/font"
)

// TextLayout returns the glyph sequence for a string.
// The function panics if no font is set.
func (w *Writer) TextLayout(s string) (*font.GlyphSeq, error) {
	if err := w.mustBeSet(StateTextFont); err != nil {
		return nil, err
	}
	F, ok := w.State.TextFont.(font.Layouter)
	if !ok {
		return nil, errors.New("font does not support layouting")
	}
	// TODO(voss): use character spacing, word spacing, horizontal scaling
	return F.Layout(w.TextFontSize, s), nil
}

// TextShow draws a string.
func (w *Writer) TextShow(s string) float64 {
	if !w.isValid("TextShow", objText) {
		return 0
	}
	if !w.isSet(StateTextFont) {
		w.Err = errors.New("no font set")
		return 0
	}
	gg, err := w.TextLayout(s)
	if err != nil {
		w.Err = err
		return 0
	}
	return w.TextShowGlyphs(gg)
}

// TextShowAligned draws a string and aligns it.
// The string is aligned in a space of the given width.
// q=0 means left alignment, q=1 means right alignment
// and q=0.5 means centering.
func (w *Writer) TextShowAligned(s string, width, q float64) {
	if !w.isValid("TextShowAligned", objText) {
		return
	}
	gg, err := w.TextLayout(s)
	if err != nil {
		w.Err = err
		return
	}
	gg.Align(width, q)
	w.TextShowGlyphs(gg)
}
