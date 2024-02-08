// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

// This file implements all text-related PDF operators.  The operators
// implemented here are defined in tables 103, 105, 106 107 of ISO
// 32000-2:2020.

// TextStart starts a new text object.
//
// This implements the PDF graphics operator "BT".
func (w *Writer) TextStart() {
	if !w.isValid("TextStart", objPage) {
		return
	}
	w.currentObject = objText

	w.nesting = append(w.nesting, pairTypeBT)

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
	w.currentObject = objPage

	if len(w.nesting) == 0 || w.nesting[len(w.nesting)-1] != pairTypeBT {
		w.Err = errors.New("TextEnd: no matching TextStart")
		return
	}
	w.nesting = w.nesting[:len(w.nesting)-1]

	w.Set &= ^StateTextMatrix

	_, w.Err = fmt.Fprintln(w.Content, "ET")
}

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
// The value 100 corresponds to the normal scaling.
//
// This implementes the PDF graphics operator "Tz".
func (w *Writer) TextSetHorizontalScaling(scaling float64) {
	if !w.isValid("TextSetHorizontalScaling", objText|objPage) {
		return
	}
	scaling /= 100
	if w.isSet(StateTextHorizontalScaling) && nearlyEqual(scaling, w.State.TextHorizontalScaling) {
		return
	}

	w.State.TextHorizontalScaling = scaling
	w.Set |= StateTextHorizontalScaling

	_, w.Err = fmt.Fprintln(w.Content, w.coord(scaling*100), "Tz")
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
func (w *Writer) TextSetFont(font font.Embedded, size float64) {
	if !w.isValid("TextSetFont", objText|objPage) {
		return
	}
	if w.isSet(StateTextFont) && w.State.TextFont == font && nearlyEqual(w.State.TextFontSize, size) {
		return
	}

	if _, ok := font.PDFObject().(pdf.Reference); !ok {
		// TODO(voss): can this happen?  do we need this check?
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
	if err := w.mustBeSet(StateTextMatrix | StateTextLeading); err != nil {
		w.Err = err
		return
	}

	w.TextLineMatrix = Translate(0, -w.TextLeading).Mul(w.TextLineMatrix)
	w.TextMatrix = w.TextLineMatrix

	_, w.Err = fmt.Fprintln(w.Content, "T*")
}

// TextShowRaw shows an already encoded text in the PDF file.
//
// This implements the PDF graphics operator "Tj".
func (w *Writer) TextShowRaw(s pdf.String) {
	if !w.isValid("TextShowRaw", objText) {
		return
	}
	if err := w.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise); err != nil {
		w.Err = err
		return
	}

	F := w.State.TextFont
	wmode := F.WritingMode()
	F.ForeachWidth(s, func(width float64, is_space bool) {
		width = width*w.TextFontSize + w.TextCharacterSpacing
		if is_space {
			width += w.TextWordSpacing
		}
		switch wmode {
		case 0: // horizontal
			w.TextMatrix = Translate(width*w.TextHorizontalScaling, 0).Mul(w.TextMatrix)
		case 1: // vertical
			w.TextMatrix = Translate(0, width).Mul(w.TextMatrix)
		}
	})

	w.Err = s.PDF(w.Content)
	if w.Err != nil {
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, " Tj")
}

// TextShowNextLineRaw start a new line and then shows an already encoded text
// in the PDF file.  This has the same effect as [Writer.TextNextLine] followed
// by [Writer.TextShowRaw].
//
// This implements the PDF graphics operator "'".
func (w *Writer) TextShowNextLineRaw(s pdf.String) {
	if !w.isValid("TextShowNextLineRaw", objText) {
		return
	}
	if err := w.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise | StateTextLeading); err != nil {
		w.Err = err
		return
	}

	F := w.State.TextFont
	wmode := F.WritingMode()
	F.ForeachWidth(s, func(width float64, is_space bool) {
		width = width*w.TextFontSize + w.TextCharacterSpacing
		if is_space {
			width += w.TextWordSpacing
		}
		switch wmode {
		case 0: // horizontal
			w.TextMatrix = Translate(width*w.TextHorizontalScaling, 0).Mul(w.TextMatrix)
		case 1: // vertical
			w.TextMatrix = Translate(0, width).Mul(w.TextMatrix)
		}
	})

	w.Err = s.PDF(w.Content)
	if w.Err != nil {
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, " '")
}

// TextShowSpacedRaw adjusts word and character spacing and then shows an
// already encoded text in the PDF file. This has the same effect as
// [Writer.TextSetWordSpacing] and [Writer.TextSetCharacterSpacing], followed
// by [Writer.TextShowRaw].
//
// This implements the PDF graphics operator '"'.
func (w *Writer) TextShowSpacedRaw(wordSpacing, charSpacing float64, s pdf.String) {
	if !w.isValid("TextShowSpacedRaw", objText) {
		return
	}
	if err := w.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise); err != nil {
		w.Err = err
		return
	}

	F := w.State.TextFont
	wmode := F.WritingMode()
	F.ForeachWidth(s, func(width float64, is_space bool) {
		width = width*w.TextFontSize + w.TextCharacterSpacing
		if is_space {
			width += w.TextWordSpacing
		}
		switch wmode {
		case 0: // horizontal
			w.TextMatrix = Translate(width*w.TextHorizontalScaling, 0).Mul(w.TextMatrix)
		case 1: // vertical
			w.TextMatrix = Translate(0, width).Mul(w.TextMatrix)
		}
	})

	_, w.Err = fmt.Fprint(w.Content, w.coord(wordSpacing), w.coord(charSpacing), " ")
	if w.Err != nil {
		return
	}
	w.Err = s.PDF(w.Content)
	if w.Err != nil {
		return
	}
	_, w.Err = fmt.Fprintln(w.Content, " \"")
}

// TextShowKernedRaw shows an already encoded text in the PDF file, using
// kerning information provided to adjust glyph spacing.
//
// This implements the PDF graphics operator "TJ".
func (w *Writer) TextShowKernedRaw(args ...pdf.Object) {
	if !w.isValid("TextShowKernedRaw", objText) {
		return
	}
	if err := w.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise); err != nil {
		w.Err = err
		return
	}

	// TODO(voss): implement this
	panic("not implemented")
}
