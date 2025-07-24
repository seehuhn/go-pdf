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
	"math"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
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
				w.writeObjects(s, pdf.Operator("Tj"))
				out = out[:0]
				return
			}
		}

		w.writeObjects(out, pdf.Operator("TJ"))
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
			w.writeObjects(pdf.Number(w.State.TextRise), pdf.Operator("Ts"))
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
		prevLen := len(run)
		run, glyphWidth = E.(font.EmbeddedLayouter).AppendEncoded(run, g.GID, g.Text)
		isSpace := len(run) == prevLen+1 && run[prevLen] == ' '
		glyphWidth = glyphWidth*param.TextFontSize + param.TextCharacterSpacing
		if isSpace {
			glyphWidth += param.TextWordSpacing
		}

		xActual += glyphWidth * param.TextHorizontalScaling
		xWanted += g.Advance
	}
	xOffsetInt := pdf.Integer(math.Round((xWanted - xActual) / param.TextFontSize / param.TextHorizontalScaling * 1000))
	if xOffsetInt != 0 {
		if len(run) > 0 {
			out = append(out, run)
			run = nil
		}
		out = append(out, -xOffsetInt)
		xActual += float64(xOffsetInt) / 1000 * param.TextFontSize * param.TextHorizontalScaling
	}
	flush()
	w.TextMatrix = matrix.Translate(xActual, 0).Mul(w.TextMatrix)

	return xActual
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

	T := font.NewTypesetter(w.CurrentFont, w.TextFontSize)
	T.SetCharacterSpacing(w.TextCharacterSpacing)
	T.SetWordSpacing(w.TextWordSpacing)
	T.SetHorizontalScaling(w.TextHorizontalScaling)
	T.SetTextRise(w.TextRise)

	return T.Layout(seq, text)
}

// TextGetQuadPoints returns QuadPoints for a glyph sequence in default user
// space coordinates. Returns an array of 8 numbers representing one
// quadrilateral (x1 y1 x2 y2 x3 y3 x4 y4), where (x1,y1)--(x2,y2) is the
// bottom edge of the (possibly rotated) bounding box.
func (w *Writer) TextGetQuadPoints(seq *font.GlyphSeq) []float64 {
	if seq == nil || len(seq.Seq) == 0 {
		return nil
	}
	if err := w.mustBeSet(StateTextFont | StateTextMatrix); err != nil {
		return nil
	}

	// get bounding rectangle in PDF text space units
	f := w.CurrentFont
	geom := f.GetGeometry()
	size := w.TextFontSize

	var width, height, depth float64
	first := true
	for _, glyph := range seq.Seq {
		width += glyph.Advance

		bbox := &geom.GlyphExtents[glyph.GID]
		if bbox.IsZero() {
			continue
		}
		thisDepth := -(bbox.LLy*size/1000 + glyph.Rise)
		thisHeight := (bbox.URy*size/1000 + glyph.Rise)

		if thisDepth > depth || first {
			depth = thisDepth
		}
		if thisHeight > height || first {
			height = thisHeight
		}
		first = false
	}
	if first {
		return nil
	}
	// Build glyph rectangle in text space
	// Calculate the actual glyph bounds including left bearing
	skip := seq.Skip

	// Find the left-most and right-most glyph bounds
	var leftBearing, rightBearing float64
	first = true
	currentPos := skip
	for _, glyph := range seq.Seq {
		bbox := &geom.GlyphExtents[glyph.GID]
		if !bbox.IsZero() {
			glyphLeft := currentPos + bbox.LLx*size/1000
			glyphRight := currentPos + bbox.URx*size/1000
			if first {
				leftBearing = glyphLeft
				rightBearing = glyphRight
				first = false
			} else {
				if glyphLeft < leftBearing {
					leftBearing = glyphLeft
				}
				if glyphRight > rightBearing {
					rightBearing = glyphRight
				}
			}
		}
		currentPos += glyph.Advance
	}

	rectText := []float64{
		leftBearing, -depth, // bottom-left
		rightBearing, -depth, // bottom-right
		rightBearing, height, // top-right
		leftBearing, height, // top-left
	}

	// transform the bounding rectangle from text space to default user space
	M := w.TextMatrix.Mul(w.CTM)
	rectUser := make([]float64, 8)
	for i := range 4 {
		x, y := M.Apply(rectText[2*i], rectText[2*i+1])
		rectUser[2*i] = x
		rectUser[2*i+1] = y
	}

	return rectUser
}
