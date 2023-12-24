// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// TextSetCharacterSpacing sets the character spacing.
//
// This implementes the PDF graphics operator "Tc".
func (w *Writer) TextSetCharacterSpacing(spacing float64) {
	if !w.isValid("TextSetCharSpacing", objText|objPage) {
		return
	}
	if w.isSet(StateTextCharacterSpacing) && nearlyEqual(spacing, w.State.TextCharacterSpacing) {
		return
	}
	w.State.TextCharacterSpacing = spacing
	w.Set |= StateTextCharacterSpacing
	_, w.Err = fmt.Fprintln(w.Content, w.coord(spacing), "Tc")
}

// TextSetWordSpacing sets the word spacing.
//
// This implementes the PDF graphics operator "Tw".
func (w *Writer) TextSetWordSpacing(spacing float64) {
	if !w.isValid("TextSetWordSpacing", objText|objPage) {
		return
	}
	if w.isSet(StateTextWordSpacing) && nearlyEqual(spacing, w.State.TextWordSpacing) {
		return
	}
	w.State.TextWordSpacing = spacing
	w.Set |= StateTextWordSpacing
	_, w.Err = fmt.Fprintln(w.Content, w.coord(spacing), "Tw")
}

// TextSetHorizontalScaling sets the horizontal scaling.
//
// This implementes the PDF graphics operator "Tz".
func (w *Writer) TextSetHorizontalScaling(scaling float64) {
	if !w.isValid("TextSetHorizontalScaling", objText|objPage) {
		return
	}
	if w.isSet(StateTextHorizontalSpacing) && nearlyEqual(scaling, w.State.TextHorizonalScaling) {
		return
	}
	w.State.TextHorizonalScaling = scaling
	w.Set |= StateTextHorizontalSpacing
	_, w.Err = fmt.Fprintln(w.Content, w.coord(scaling), "Tz")
}

// TextSetLeading sets the leading.
//
// This implementes the PDF graphics operator "TL".
func (w *Writer) TextSetLeading(leading float64) {
	if !w.isValid("TextSetLeading", objText|objPage) {
		return
	}
	if w.isSet(StateTextLeading) && nearlyEqual(leading, w.State.TextLeading) {
		return
	}
	w.State.TextLeading = leading
	w.Set |= StateTextLeading
	_, w.Err = fmt.Fprintln(w.Content, w.coord(leading), "TL")
}

// TextSetFont sets the font and font size.
//
// This implements the PDF graphics operator "Tf".
func (w *Writer) TextSetFont(font font.NewFont, size float64) {
	if !w.isValid("TextSetFont", objText|objPage) {
		return
	}
	if w.isSet(StateTextFont) && w.State.TextFont == font && nearlyEqual(w.State.TextFontSize, size) {
		return
	}
	if _, ok := font.PDFObject().(pdf.Reference); !ok {
		panic("font is not an indirect object")
	}
	w.State.TextFont = font
	w.State.TextFontSize = size
	w.State.Set |= StateTextFont
	name := w.getResourceName(catFont, font)
	err := name.PDF(w.Content)
	if err != nil {
		w.Err = err
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, "", size, "Tf")
}

// TextSetRenderingMode sets the text rendering mode.
//
// This implements the PDF graphics operator "Tr".
func (w *Writer) TextSetRenderingMode(mode TextRenderingMode) {
	if !w.isValid("TextSetRenderingMode", objText|objPage) {
		return
	}
	if w.isSet(StateTextRenderingMode) && w.State.TextRenderingMode == mode {
		return
	}
	w.State.TextRenderingMode = mode
	w.Set |= StateTextRenderingMode
	_, w.Err = fmt.Fprintln(w.Content, mode, "Tr")
}

// TextSetRise sets the text rise.
//
// This implements the PDF graphics operator "Ts".
func (w *Writer) TextSetRise(rise float64) {
	if !w.isValid("TextSetRise", objText|objPage) {
		return
	}
	if w.isSet(StateTextRise) && nearlyEqual(rise, w.State.TextRise) {
		return
	}
	w.State.TextRise = rise
	w.Set |= StateTextRise
	_, w.Err = fmt.Fprintln(w.Content, w.coord(rise), "Ts")
}

// TextStart starts a new text object.
//
// This implements the PDF graphics operator "BT".
func (w *Writer) TextStart() {
	if !w.isValid("TextStart", objPage) {
		return
	}
	w.nesting = append(w.nesting, pairTypeBT)

	w.currentObject = objText
	w.State.TextMatrix = IdentityMatrix
	w.State.TextLineMatrix = IdentityMatrix
	w.Set |= StateTextMatrix
	_, w.Err = fmt.Fprintln(w.Content, "BT")
}

// TextEnd ends the current text object.
//
// This implements the PDF graphics operator "ET".
func (w *Writer) TextEnd() {
	if !w.isValid("TextEnd", objText) {
		return
	}
	if len(w.nesting) == 0 || w.nesting[len(w.nesting)-1] != pairTypeBT {
		w.Err = errors.New("TextEnd without TextStart")
		return
	}
	w.nesting = w.nesting[:len(w.nesting)-1]
	w.Set &= ^StateTextMatrix

	w.currentObject = objPage
	_, w.Err = fmt.Fprintln(w.Content, "ET")
}

// TextFirstLine moves to the start of the next line of text.
//
// This implements the PDF graphics operator "Td".
func (w *Writer) TextFirstLine(dx, dy float64) {
	if !w.isValid("TextFirstLine", objText) {
		return
	}
	w.TextLineMatrix = Translate(dx, dy).Mul(w.TextLineMatrix)
	w.TextMatrix = w.TextLineMatrix
	_, w.Err = fmt.Fprintln(w.Content, w.coord(dx), w.coord(dy), "Td")
}

// TextSecondLine moves to the start of the next line of text and sets
// the leading.  Usually, dy is negative.
//
// This implements the PDF graphics operator "TD".
func (w *Writer) TextSecondLine(dx, dy float64) {
	if !w.isValid("TextSecondLine", objText) {
		return
	}
	w.TextLineMatrix = Translate(dx, dy).Mul(w.TextLineMatrix)
	w.TextMatrix = w.TextLineMatrix
	w.TextLeading = -dy
	w.Set |= StateTextLeading
	_, w.Err = fmt.Fprintln(w.Content, w.coord(dx), w.coord(dy), "TD")
}

// TextSetMatrix replaces the current text matrix and line matrix with M.
//
// This implements the PDF graphics operator "Tm".
func (w *Writer) TextSetMatrix(M Matrix) {
	if !w.isValid("TextSetMatrix", objText) {
		return
	}
	w.TextMatrix = M
	w.TextLineMatrix = M
	w.Set |= StateTextMatrix
	_, w.Err = fmt.Fprintln(w.Content, w.coord(M[0]), w.coord(M[1]), w.coord(M[2]), w.coord(M[3]), w.coord(M[4]), w.coord(M[5]), "Tm")
}

// TextNextLine moves to the start of the next line of text.
//
// This implements the PDF graphics operator "T*".
func (w *Writer) TextNextLine() {
	if !w.isValid("TextNewLine", objText) {
		return
	}
	w.TextLineMatrix = Translate(0, -w.TextLeading).Mul(w.TextLineMatrix)
	w.TextMatrix = w.TextLineMatrix
	_, w.Err = fmt.Fprintln(w.Content, "T*")
}

// TextShowRaw shows the PDF string s.
//
// This implements the PDF graphics operator "Tj".
func (w *Writer) TextShowRaw(s pdf.String) {
	if !w.isValid("TextShowRaw", objText) {
		return
	}
	err := s.PDF(w.Content)
	if err != nil {
		w.Err = err
		return
	}

	var width float64
	codes := s
	for len(codes) > 0 {
		c, n := w.TextFont.Decode(codes)
		if c >= 0 {
			// TODO(voss): add the glyph width
			// width += w.TextFont.GlyphWidth(c)
			if n == 1 && codes[0] == 0x20 {
				width += w.State.TextWordSpacing
			}
		}
		width += w.State.TextCharacterSpacing
		codes = codes[n:]
	}

	switch w.TextFont.WritingMode() {
	case 0: // horizontal
		w.TextMatrix = Translate(width, 0).Mul(w.TextMatrix)
	case 1: // vertical
		w.TextMatrix = Translate(0, width).Mul(w.TextMatrix)
	}

	_, w.Err = fmt.Fprintln(w.Content, " Tj")
}
