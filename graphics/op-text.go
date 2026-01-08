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

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/state"
)

// This file implements all text-related PDF operators.  The operators
// implemented here are defined in tables 103, 105, 106 107 of ISO
// 32000-2:2020.

// TextBegin starts a new text object.
// This must be paired with [Writer.TextEnd].
//
// This implements the PDF graphics operator "BT".
func (w *Writer) TextBegin() {
	if !w.isValid("TextBegin", objPage) {
		return
	}
	w.currentObject = objText

	w.nesting = append(w.nesting, pairTypeBT)

	w.State.TextMatrix = matrix.Identity
	w.State.TextLineMatrix = matrix.Identity
	w.Set |= state.TextMatrix

	_, w.Err = fmt.Fprintln(w.Content, "BT")
}

// TextEnd ends the current text object.
// This must be paired with [Writer.TextBegin].
//
// This implements the PDF graphics operator "ET".
func (w *Writer) TextEnd() {
	if !w.isValid("TextEnd", objText) {
		return
	}
	w.currentObject = objPage

	if len(w.nesting) == 0 || w.nesting[len(w.nesting)-1] != pairTypeBT {
		w.Err = errors.New("TextEnd: no matching TextBegin")
		return
	}
	w.nesting = w.nesting[:len(w.nesting)-1]

	w.Set &= ^state.TextMatrix

	_, w.Err = fmt.Fprintln(w.Content, "ET")
}

// TextSetCharacterSpacing sets additional character spacing.
//
// This implements the PDF graphics operator "Tc".
func (w *Writer) TextSetCharacterSpacing(charSpacing float64) {
	if !w.isValid("TextSetCharSpacing", objText|objPage) {
		return
	}
	if w.isSet(state.TextCharacterSpacing) && nearlyEqual(charSpacing, w.State.TextCharacterSpacing) {
		return
	}

	w.State.TextCharacterSpacing = charSpacing
	w.Set |= state.TextCharacterSpacing

	_, w.Err = fmt.Fprintln(w.Content, w.coord(charSpacing), "Tc")
}

// TextSetWordSpacing sets additional word spacing.
//
// This implements the PDF graphics operator "Tw".
func (w *Writer) TextSetWordSpacing(wordSpacing float64) {
	if !w.isValid("TextSetWordSpacing", objText|objPage) {
		return
	}
	if w.isSet(state.TextWordSpacing) && nearlyEqual(wordSpacing, w.State.TextWordSpacing) {
		return
	}

	w.State.TextWordSpacing = wordSpacing
	w.Set |= state.TextWordSpacing

	_, w.Err = fmt.Fprintln(w.Content, w.coord(wordSpacing), "Tw")
}

// TextSetHorizontalScaling sets the horizontal scaling.
// The effect of this is to strech/compress the text horizontally.
// The value 1 corresponds to normal scaling.
// Negative values correspond to horizontally mirrored text.
//
// This implements the PDF graphics operator "Tz".
func (w *Writer) TextSetHorizontalScaling(scaling float64) {
	if !w.isValid("TextSetHorizontalScaling", objText|objPage) {
		return
	}
	if w.isSet(state.TextHorizontalScaling) && nearlyEqual(scaling, w.State.TextHorizontalScaling) {
		return
	}

	w.State.TextHorizontalScaling = scaling
	w.Set |= state.TextHorizontalScaling

	_, w.Err = fmt.Fprintln(w.Content, w.coord(scaling*100), "Tz")
}

// TextSetLeading sets the leading.
// The leading is the distance between the baselines of two consecutive lines of text.
// Positive values indicate that the next line of text is below the current line.
//
// This implements the PDF graphics operator "TL".
func (w *Writer) TextSetLeading(leading float64) {
	if !w.isValid("TextSetLeading", objText|objPage) {
		return
	}
	if w.isSet(state.TextLeading) && nearlyEqual(leading, w.State.TextLeading) {
		return
	}

	w.State.TextLeading = leading
	w.Set |= state.TextLeading

	_, w.Err = fmt.Fprintln(w.Content, w.coord(leading), "TL")
}

// TextSetFont sets the font and font size.
//
// This implements the PDF graphics operator "Tf".
func (w *Writer) TextSetFont(F font.Instance, size float64) {
	if !w.isValid("TextSetFont", objText|objPage) {
		return
	}

	name, err := writerGetResourceName(w, catFont, F)
	if err != nil {
		w.Err = err
		return
	}

	if w.isSet(state.TextFont) && w.State.TextFont == F && nearlyEqual(w.State.TextFontSize, size) {
		return
	}

	w.State.TextFont = F
	w.State.TextFontSize = size
	w.State.Set |= state.TextFont

	w.writeObjects(name, pdf.Number(size), pdf.Operator("Tf"))
}

// TextSetRenderingMode sets the text rendering mode.
//
// This implements the PDF graphics operator "Tr".
func (w *Writer) TextSetRenderingMode(mode TextRenderingMode) {
	if !w.isValid("TextSetRenderingMode", objText|objPage) {
		return
	}
	if w.isSet(state.TextRenderingMode) && w.State.TextRenderingMode == mode {
		return
	}

	w.State.TextRenderingMode = mode
	w.Set |= state.TextRenderingMode

	_, w.Err = fmt.Fprintln(w.Content, mode, "Tr")
}

// TextSetRise sets the text rise.
// Positive values move the text up.
//
// This implements the PDF graphics operator "Ts".
func (w *Writer) TextSetRise(rise float64) {
	if !w.isValid("TextSetRise", objText|objPage) {
		return
	}
	if w.isSet(state.TextRise) && nearlyEqual(rise, w.State.TextRise) {
		return
	}

	w.State.TextRise = rise
	w.Set |= state.TextRise

	_, w.Err = fmt.Fprintln(w.Content, w.coord(rise), "Ts")
}

// TextFirstLine moves to the start of the next line of text.
// The new text position is (x, y), relative to the start of the current line
// (or to the current point if there is no current line).
//
// This implements the PDF graphics operator "Td".
func (w *Writer) TextFirstLine(x, y float64) {
	if !w.isValid("TextFirstLine", objText) {
		return
	}

	w.TextLineMatrix = matrix.Translate(x, y).Mul(w.TextLineMatrix)
	w.TextMatrix = w.TextLineMatrix

	_, w.Err = fmt.Fprintln(w.Content, w.coord(x), w.coord(y), "Td")
}

// TextSecondLine moves to the point (dx, dy) relative to the start of the
// current line of text.   The function also sets the leading to -dy.
// Usually, dy is negative.
//
// This implements the PDF graphics operator "TD".
func (w *Writer) TextSecondLine(dx, dy float64) {
	if !w.isValid("TextSecondLine", objText) {
		return
	}

	w.TextLineMatrix = matrix.Translate(dx, dy).Mul(w.TextLineMatrix)
	w.TextMatrix = w.TextLineMatrix
	w.TextLeading = -dy
	w.Set |= state.TextLeading

	_, w.Err = fmt.Fprintln(w.Content, w.coord(dx), w.coord(dy), "TD")
}

// TextSetMatrix replaces the current text matrix and line matrix with M.
//
// This implements the PDF graphics operator "Tm".
func (w *Writer) TextSetMatrix(M matrix.Matrix) {
	if !w.isValid("TextSetMatrix", objText) {
		return
	}

	w.TextMatrix = M
	w.TextLineMatrix = M
	w.Set |= state.TextMatrix

	_, w.Err = fmt.Fprintln(w.Content, w.coord(M[0]), w.coord(M[1]), w.coord(M[2]), w.coord(M[3]), w.coord(M[4]), w.coord(M[5]), "Tm")
}

// TextNextLine moves to the start of the next line of text.
//
// This implements the PDF graphics operator "T*".
func (w *Writer) TextNextLine() {
	if !w.isValid("TextNewLine", objText) {
		return
	}
	if err := w.mustBeSet(state.TextMatrix | state.TextLeading); err != nil {
		w.Err = err
		return
	}

	w.TextLineMatrix = matrix.Translate(0, -w.TextLeading).Mul(w.TextLineMatrix)
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
	if err := w.mustBeSet(state.TextFont | state.TextMatrix | state.TextHorizontalScaling | state.TextRise | state.TextWordSpacing | state.TextCharacterSpacing); err != nil {
		w.Err = err
		return
	}

	w.updateTextPosition(s)

	w.writeObjects(s, pdf.Operator("Tj"))
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
	if err := w.mustBeSet(state.TextFont | state.TextMatrix | state.TextHorizontalScaling | state.TextRise | state.TextWordSpacing | state.TextCharacterSpacing | state.TextLeading); err != nil {
		w.Err = err
		return
	}

	w.TextLineMatrix = matrix.Translate(0, -w.TextLeading).Mul(w.TextLineMatrix)
	w.TextMatrix = w.TextLineMatrix

	w.updateTextPosition(s)

	w.writeObjects(s, pdf.Operator("'"))
}

// TextShowSpacedRaw adjusts word and character spacing and then shows an
// already encoded text in the PDF file.  This has the same effect as
// [Writer.TextSetWordSpacing] and [Writer.TextSetCharacterSpacing], followed
// by [Writer.TextShowRaw].
//
// This implements the PDF graphics operator '"'.
func (w *Writer) TextShowSpacedRaw(wordSpacing, charSpacing float64, s pdf.String) {
	if !w.isValid("TextShowSpacedRaw", objText) {
		return
	}
	if err := w.mustBeSet(state.TextFont | state.TextMatrix | state.TextHorizontalScaling | state.TextRise); err != nil {
		w.Err = err
		return
	}

	w.State.TextWordSpacing = wordSpacing
	w.State.TextCharacterSpacing = charSpacing
	w.Set |= state.TextWordSpacing | state.TextCharacterSpacing
	w.updateTextPosition(s)

	w.writeObjects(pdf.Number(wordSpacing), pdf.Number(charSpacing), s, pdf.Operator(`"`))
}

// TextShowKernedRaw shows an already encoded text in the PDF file, using
// kerning information provided to adjust glyph spacing.
//
// The arguments must be of type [pdf.String], [pdf.Real], [pdf.Integer] or
// [pdf.Number].
//
// This implements the PDF graphics operator "TJ".
func (w *Writer) TextShowKernedRaw(args ...pdf.Object) {
	if !w.isValid("TextShowKernedRaw", objText) {
		return
	}
	if err := w.mustBeSet(state.TextFont | state.TextMatrix | state.TextHorizontalScaling | state.TextRise | state.TextWordSpacing | state.TextCharacterSpacing); err != nil {
		w.Err = err
		return
	}

	var a pdf.Array
	wMode := w.State.TextFont.WritingMode()
	for _, arg := range args {
		var delta float64
		switch arg := arg.AsPDF(w.opt).(type) {
		case pdf.String:
			w.updateTextPosition(arg)
			if w.Err != nil {
				return
			}
		case pdf.Real:
			delta = float64(arg)
		case pdf.Integer:
			delta = float64(arg)
		default:
			w.Err = fmt.Errorf("TextShowKernedRaw: invalid argument type %T", arg)
			return
		}
		if delta != 0 { // TODO(voss): move outisde the loop?
			delta *= -w.State.TextFontSize / 1000
			if wMode == 0 {
				w.TextMatrix = matrix.Translate(delta*w.State.TextHorizontalScaling, 0).Mul(w.TextMatrix)
			} else {
				w.TextMatrix = matrix.Translate(0, delta).Mul(w.TextMatrix)
			}
		}
		a = append(a, arg)
	}

	w.writeObjects(a, pdf.Operator("TJ"))
}

func (w *Writer) updateTextPosition(s pdf.String) {
	// TODO(voss): can this be merged with the corresponding code in
	// reader/reader.go and layout.go?

	wmode := w.TextFont.WritingMode()
	for info := range w.TextFont.Codes(s) {
		width := info.Width*w.TextFontSize + w.TextCharacterSpacing
		if info.UseWordSpacing {
			width += w.TextWordSpacing
		}
		if wmode == font.Horizontal {
			width *= w.TextHorizontalScaling
		}

		switch wmode {
		case font.Horizontal:
			w.TextMatrix = matrix.Translate(width, 0).Mul(w.TextMatrix)
		case font.Vertical:
			w.TextMatrix = matrix.Translate(0, width).Mul(w.TextMatrix)
		}
	}
}
