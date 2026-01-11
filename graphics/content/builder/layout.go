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

package builder

import (
	"errors"
	"math"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/state"
)

// TextShow draws a string. Returns the advance width.
func (b *Builder) TextShow(s string) float64 {
	if b.Err != nil {
		return 0
	}

	if b.glyphBuf == nil {
		b.glyphBuf = &font.GlyphSeq{}
	}
	b.glyphBuf.Reset()
	gg := b.TextLayout(b.glyphBuf, s)
	if b.Err != nil {
		return 0
	}

	return b.TextShowGlyphs(gg)
}

// TextShowAligned draws a string and aligns it.
// The string is aligned in a space of the given width.
// q=0 means left alignment, q=1 means right alignment
// and q=0.5 means centering.
func (b *Builder) TextShowAligned(s string, width, q float64) {
	if b.Err != nil {
		return
	}

	gg := b.TextLayout(nil, s)
	if b.Err != nil {
		return
	}
	gg.Align(width, q)
	b.TextShowGlyphs(gg)
}

// TextShowGlyphs shows a glyph sequence, taking kerning and text rise into
// account. Returns the advance width.
//
// This uses the "TJ", "Tj" and "Ts" PDF graphics operators.
func (b *Builder) TextShowGlyphs(seq *font.GlyphSeq) float64 {
	if b.Err != nil {
		return 0
	}

	if !b.isUsable(state.TextFont | state.TextMatrix |
		state.TextHorizontalScaling | state.TextRise) {
		b.Err = errors.New("required text state not set")
		return 0
	}

	E := b.State.GState.TextFont
	layouter, ok := E.(font.Layouter)
	if !ok {
		b.Err = errors.New("font does not implement Layouter")
		return 0
	}

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
		if b.Err != nil {
			return
		}

		if len(out) == 1 {
			if s, ok := out[0].(pdf.String); ok {
				b.emit(content.OpTextShow, s)
				out = out[:0]
				return
			}
		}

		b.emit(content.OpTextShowArray, out)
		out = out[:0]
	}

	xActual := 0.0
	xWanted := left
	param := b.State.GState
	if E.WritingMode() != 0 {
		panic("vertical writing mode not implemented")
	}
	codec := layouter.Codec()
	for _, g := range gg {
		if !b.isUsable(state.TextRise) || math.Abs(g.Rise-param.TextRise) > 1e-6 {
			flush()
			b.TextSetRise(g.Rise)
			if b.Err != nil {
				return 0
			}
		}

		xOffsetInt := pdf.Integer(math.Round((xWanted - xActual) / param.TextFontSize / param.TextHorizontalScaling * 1000))
		if xOffsetInt != 0 && !layouter.IsBlank(g.GID) {
			if len(run) > 0 {
				out = append(out, run)
				run = nil
			}
			out = append(out, -xOffsetInt)
			xActual += float64(xOffsetInt) / 1000 * param.TextFontSize * param.TextHorizontalScaling
		}

		xWanted += g.Advance

		prevLen := len(run)
		charCode, ok := layouter.Encode(g.GID, g.Text)
		if !ok {
			continue // skip glyphs that can't be encoded
		}
		run = codec.AppendCode(run, charCode)
		for info := range layouter.Codes(run[prevLen:]) {
			glyphWidth := info.Width*param.TextFontSize + param.TextCharacterSpacing
			if info.UseWordSpacing {
				glyphWidth += param.TextWordSpacing
			}
			xActual += glyphWidth * param.TextHorizontalScaling
		}
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
	b.State.GState.TextMatrix = matrix.Translate(xActual, 0).Mul(b.State.GState.TextMatrix)

	return xActual
}

// TextLayout appends a string to a GlyphSeq, using the text parameters from
// the builder's graphics state. If seq is nil, a new GlyphSeq is allocated.
// The resulting GlyphSeq is returned.
//
// If no font is set, or if the current font does not implement
// [font.Layouter], an error is set and an empty GlyphSeq is returned.
func (b *Builder) TextLayout(seq *font.GlyphSeq, text string) *font.GlyphSeq {
	if seq == nil {
		seq = &font.GlyphSeq{}
	}

	if b.Err != nil {
		return seq
	}

	layouter, ok := b.State.GState.TextFont.(font.Layouter)
	if !ok {
		b.Err = errors.New("no font set, or font does not support layout")
		return seq
	}

	param := b.State.GState
	T := font.NewTypesetter(layouter, param.TextFontSize)
	T.SetCharacterSpacing(param.TextCharacterSpacing)
	T.SetWordSpacing(param.TextWordSpacing)
	T.SetHorizontalScaling(param.TextHorizontalScaling)
	T.SetTextRise(param.TextRise)

	return T.Layout(seq, text)
}

// TextGetQuadPoints returns QuadPoints for a glyph sequence in default user
// space coordinates. Returns 4 Vec2 points representing one quadrilateral,
// where the first two points form the bottom edge of the (possibly rotated)
// bounding box.
func (b *Builder) TextGetQuadPoints(seq *font.GlyphSeq, padding float64) []vec.Vec2 {
	// TODO(voss): make sure this is correct for vertical writing mode.

	if seq == nil || len(seq.Seq) == 0 {
		return nil
	}
	if !b.isUsable(state.TextFont | state.TextMatrix) {
		return nil
	}

	// get bounding rectangle in PDF text space units
	f, ok := b.State.GState.TextFont.(font.Layouter)
	if !ok {
		return nil
	}
	geom := f.GetGeometry()
	size := b.State.GState.TextFontSize

	height := geom.Ascent * size
	depth := -geom.Descent * size
	var leftBearing, rightBearing float64

	first := true
	currentPos := seq.Skip
	for _, glyph := range seq.Seq {
		bbox := &geom.GlyphExtents[glyph.GID]
		if !bbox.IsZero() {
			glyphDepth := -(bbox.LLy*size + glyph.Rise)
			glyphHeight := bbox.URy*size + glyph.Rise
			glyphLeft := currentPos + bbox.LLx*size
			glyphRight := currentPos + bbox.URx*size

			if glyphDepth > depth {
				depth = glyphDepth
			}
			if glyphHeight > height {
				height = glyphHeight
			}
			if glyphLeft < leftBearing || first {
				leftBearing = glyphLeft
			}
			if glyphRight > rightBearing || first {
				rightBearing = glyphRight
			}

			first = false
		}
		currentPos += glyph.Advance
	}
	if first {
		return nil
	}

	leftBearing -= padding
	rightBearing += padding
	height += padding
	depth += padding

	rectText := []float64{
		leftBearing, -depth, // bottom-left
		rightBearing, -depth, // bottom-right
		rightBearing, height, // top-right
		leftBearing, height, // top-left
	}

	// transform the bounding rectangle from text space to default user space
	M := b.State.GState.TextMatrix.Mul(b.State.GState.CTM)
	rectUser := make([]vec.Vec2, 4)
	for i := range 4 {
		x, y := M.Apply(rectText[2*i], rectText[2*i+1])
		rectUser[i] = vec.Vec2{X: x, Y: y}
	}

	return rectUser
}
