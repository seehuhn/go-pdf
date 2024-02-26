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
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

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

	// TODO(voss): disable ligatures if TextCharacterSpacing is non-zero
	gg := F.Layout(w.TextFontSize, s)

	// Apply PDF layout parameters
	for i, g := range gg.Seq {
		advance := g.Advance
		advance += w.TextCharacterSpacing
		if string(g.Text) == " " {
			advance += w.TextWordSpacing
		}
		gg.Seq[i].Advance = advance * w.TextHorizontalScaling
		gg.Seq[i].Rise = w.TextRise
	}

	return gg, nil
}

// TextShowGlyphs shows the PDF string s, taking kerning and text rise into
// account.
//
// This uses the "TJ", "Tj" and "Ts" PDF graphics operators.
func (w *Writer) TextShowGlyphs(seq *font.GlyphSeq) float64 {
	if !w.isValid("TextShowGlyphs", objText) {
		return 0
	}
	if err := w.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise); err != nil {
		w.Err = err
		return 0
	}

	left := seq.Skip
	gg := seq.Seq

	font := w.TextFont.(font.Layouter) // TODO(voss)

	var run pdf.String
	var out pdf.Array
	flush := func() {
		if len(run) > 0 {
			out = append(out, run)
			run = nil
		}
		if len(out) == 0 {
			return
		}
		if w.Err != nil {
			return
		}

		if len(out) == 1 {
			if s, ok := out[0].(pdf.String); ok {
				w.Err = s.PDF(w.Content)
				if w.Err != nil {
					return
				}
				_, w.Err = fmt.Fprintln(w.Content, " Tj")
				out = nil
				return
			}
		}

		// TODO(voss): omit excess spaces in the content stream
		w.Err = out.PDF(w.Content)
		if w.Err != nil {
			return
		}
		_, w.Err = fmt.Fprintln(w.Content, " TJ")
		out = nil
	}

	xActual := 0.0
	xWanted := left
	param := w.State
	if font.WritingMode() != 0 {
		panic("vertical writing mode not implemented")
	}
	for _, g := range gg {
		if w.State.Set&StateTextRise == 0 || math.Abs(g.Rise-w.State.TextRise) > 1e-6 {
			flush()
			w.State.TextRise = g.Rise
			if w.Err != nil {
				return 0
			}
			w.Err = pdf.Number(w.State.TextRise).PDF(w.Content) // TODO(voss): rounding?
			if w.Err != nil {
				return 0
			}
			_, w.Err = fmt.Fprintln(w.Content, " Ts")
		}

		xOffsetInt := pdf.Integer(math.Round((xWanted - xActual) / param.TextFontSize / param.TextHorizontalScaling * 1000))
		if xOffsetInt != 0 { // TODO(voss): only do this if the glyph is not blank
			if len(run) > 0 {
				out = append(out, run)
				run = nil
			}
			out = append(out, -xOffsetInt)
			xActual += float64(xOffsetInt) / 1000 * param.TextFontSize * param.TextHorizontalScaling
		}

		var glyphWidth float64
		var isSpace bool
		run, glyphWidth, isSpace = font.CodeAndWidth(run, g.GID, g.Text)
		glyphWidth = glyphWidth*param.TextFontSize + param.TextCharacterSpacing
		if isSpace {
			glyphWidth += param.TextWordSpacing
		}

		xActual += glyphWidth * param.TextHorizontalScaling
		xWanted += g.Advance
	}
	xOffsetInt := pdf.Integer(math.Round((xWanted - xActual) * 1000 / param.TextFontSize))
	if xOffsetInt != 0 {
		if len(run) > 0 {
			out = append(out, run)
			run = nil
		}
		out = append(out, -xOffsetInt)
		xActual += float64(xOffsetInt) / 1000 * param.TextFontSize
	}
	flush()
	w.TextMatrix = Translate(xActual, 0).Mul(w.TextMatrix)

	return xActual
}

// TextShowAligned draws a string and aligns it.
// The string is aligned in a space of the given width.
// q=0 means left alignment, q=1 means right alignment
// and q=0.5 means centering.
//
// TODO(voss): remove this function?
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
