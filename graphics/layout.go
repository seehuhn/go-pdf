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
	"seehuhn.de/go/pdf/graphics/matrix"
)

// This function contains convenience methods for drawing text.
// These functions first convert Go strings to PDF strings and then call the
// functions from "op-text.go".

// TextShow draws a string.
func (w *Writer) TextShow(s string) float64 {
	if !w.isValid("TextShow", objText) {
		return 0
	}

	w.glyphBuf.Reset()
	gg := w.TextLayout(w.glyphBuf, s)
	if gg == nil {
		w.Err = errors.New("font does not support layouting")
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
	gg := w.TextLayout(nil, s)
	if gg == nil {
		w.Err = errors.New("font does not support layouting")
		return
	}
	gg.Align(width, q)
	w.TextShowGlyphs(gg)
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

	E := w.TextFont

	left := seq.Skip
	gg := seq.Seq

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
				out = out[:0]
				return
			}
		}

		// TODO(voss): omit excess spaces in the content stream
		w.Err = out.PDF(w.Content)
		if w.Err != nil {
			return
		}
		_, w.Err = fmt.Fprintln(w.Content, " TJ")
		out = out[:0]
	}

	xActual := 0.0
	xWanted := left
	param := w.State
	if E.WritingMode() != 0 {
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
		run, glyphWidth, isSpace = E.CodeAndWidth(run, g.GID, g.Text)
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
	w.TextMatrix = matrix.Translate(xActual, 0).Mul(w.TextMatrix)

	return xActual
}

type Layouter struct {
	Font     font.Layouter
	Geometry *font.Geometry
	FontSize float64

	CharacterSpacing  float64
	WordSpacing       float64
	HorizontalScaling float64
	TextRise          float64
}

func (w *Writer) NewLayouter() *Layouter {
	l := &Layouter{
		HorizontalScaling: 1,
	}

	if w.CurrentFont != nil {
		l.SetFont(w.CurrentFont, w.TextFontSize)
	}
	if w.isSet(StateTextCharacterSpacing) {
		l.CharacterSpacing = w.TextCharacterSpacing
	}
	if w.isSet(StateTextWordSpacing) {
		l.WordSpacing = w.TextWordSpacing
	}
	if w.isSet(StateTextHorizontalScaling) {
		l.HorizontalScaling = w.TextHorizontalScaling
	}
	if w.isSet(StateTextRise) {
		l.TextRise = w.TextRise
	}

	return l
}

func (l *Layouter) SetFont(F font.Layouter, size float64) {
	l.Font = F
	l.Geometry = F.GetGeometry()
	l.FontSize = size
}

func (l *Layouter) Layout(seq *font.GlyphSeq, text string) *font.GlyphSeq {
	if l.CharacterSpacing == 0 {
		l.Font.Layout(seq, l.FontSize, text)
	} else { // disable ligatures
		for _, r := range text {
			l.Font.Layout(seq, l.FontSize, string(r))
		}
	}

	// Apply PDF layout parameters
	for i := len(seq.Seq) - len(text); i < len(seq.Seq); i++ {
		advance := seq.Seq[i].Advance
		advance += l.CharacterSpacing
		if string(seq.Seq[i].Text) == " " {
			advance += l.WordSpacing
		}
		seq.Seq[i].Advance = advance * l.HorizontalScaling
		seq.Seq[i].Rise = l.TextRise
	}

	return seq
}

// TextLayout appends a string to a GlyphSeq, using the text parameters from
// the writer's graphics state.  If seq is nil, a new GlyphSeq is allocated.  The
// resulting GlyphSeq is returned.
//
// If no font is set, or if the current font does not implement
// [font.Layouter], the function returns nil.  If seq is not nil (and there is
// no error), the return value is guaranteed to be equal to seq.
func (w *Writer) TextLayout(seq *font.GlyphSeq, text string) *font.GlyphSeq {
	F := w.CurrentFont
	if F == nil {
		return nil
	}

	var characterSpacing, wordSpacing, horizontalScaling, textRise float64
	if w.isSet(StateTextCharacterSpacing) {
		characterSpacing = w.TextCharacterSpacing
	}
	if w.isSet(StateTextWordSpacing) {
		wordSpacing = w.TextWordSpacing
	}
	if w.isSet(StateTextHorizontalScaling) {
		horizontalScaling = w.TextHorizontalScaling
	} else {
		horizontalScaling = 1
	}
	if w.isSet(StateTextRise) {
		textRise = w.TextRise
	}

	if seq == nil {
		seq = &font.GlyphSeq{}
	}
	base := len(seq.Seq)

	if characterSpacing == 0 {
		F.Layout(seq, w.TextFontSize, text)
	} else {
		// disable ligatures
		for _, r := range text {
			F.Layout(seq, w.TextFontSize, string(r))
		}
	}

	// Apply PDF layout parameters
	for i := base; i < len(seq.Seq); i++ {
		advance := seq.Seq[i].Advance
		advance += characterSpacing
		if string(seq.Seq[i].Text) == " " {
			advance += wordSpacing
		}
		seq.Seq[i].Advance = advance * horizontalScaling
		seq.Seq[i].Rise = textRise
	}

	return seq
}
