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
	"fmt"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
)

// Text Object Operators

// TextBegin starts a new text object.
//
// This implements the PDF graphics operator "BT".
func (b *Builder) TextBegin() {
	b.emit(content.OpTextBegin)
}

// TextEnd ends the current text object.
//
// This implements the PDF graphics operator "ET".
func (b *Builder) TextEnd() {
	b.emit(content.OpTextEnd)
}

// Text State Operators

// TextSetCharacterSpacing sets additional character spacing.
//
// This implements the PDF graphics operator "Tc".
func (b *Builder) TextSetCharacterSpacing(charSpacing float64) {
	if b.isSet(graphics.StateTextCharacterSpacing) &&
		nearlyEqual(charSpacing, b.State.GState.TextCharacterSpacing) {
		return
	}
	b.emit(content.OpTextSetCharacterSpacing, pdf.Number(charSpacing))
}

// TextSetWordSpacing sets additional word spacing.
//
// This implements the PDF graphics operator "Tw".
func (b *Builder) TextSetWordSpacing(wordSpacing float64) {
	if b.isSet(graphics.StateTextWordSpacing) &&
		nearlyEqual(wordSpacing, b.State.GState.TextWordSpacing) {
		return
	}
	b.emit(content.OpTextSetWordSpacing, pdf.Number(wordSpacing))
}

// TextSetHorizontalScaling sets the horizontal scaling.
// The value 1 corresponds to normal scaling.
//
// This implements the PDF graphics operator "Tz".
func (b *Builder) TextSetHorizontalScaling(scaling float64) {
	if b.isSet(graphics.StateTextHorizontalScaling) &&
		nearlyEqual(scaling, b.State.GState.TextHorizontalScaling) {
		return
	}
	// PDF operator expects percentage (100 = normal)
	b.emit(content.OpTextSetHorizontalScaling, pdf.Number(scaling*100))
}

// TextSetLeading sets the text leading.
//
// This implements the PDF graphics operator "TL".
func (b *Builder) TextSetLeading(leading float64) {
	if b.isSet(graphics.StateTextLeading) &&
		nearlyEqual(leading, b.State.GState.TextLeading) {
		return
	}
	b.emit(content.OpTextSetLeading, pdf.Number(leading))
}

// TextSetRenderingMode sets the text rendering mode.
//
// This implements the PDF graphics operator "Tr".
func (b *Builder) TextSetRenderingMode(mode graphics.TextRenderingMode) {
	if mode > 7 {
		b.Err = fmt.Errorf("TextSetRenderingMode: invalid mode %d", mode)
		return
	}
	if b.isSet(graphics.StateTextRenderingMode) &&
		mode == b.State.GState.TextRenderingMode {
		return
	}
	b.emit(content.OpTextSetRenderingMode, pdf.Integer(mode))
}

// TextSetRise sets the text rise.
//
// This implements the PDF graphics operator "Ts".
func (b *Builder) TextSetRise(rise float64) {
	if b.isSet(graphics.StateTextRise) &&
		nearlyEqual(rise, b.State.GState.TextRise) {
		return
	}
	b.emit(content.OpTextSetRise, pdf.Number(rise))
}

// TextSetFont sets the font and font size.
//
// This implements the PDF graphics operator "Tf".
func (b *Builder) TextSetFont(f font.Instance, size float64) {
	if b.Err != nil {
		return
	}

	if b.isSet(graphics.StateTextFont) &&
		b.State.GState.TextFont == f &&
		nearlyEqual(b.State.GState.TextFontSize, size) {
		return
	}

	name := b.getFontName(f)
	b.emit(content.OpTextSetFont, name, pdf.Number(size))
}

// SetFontNameInternal controls how the font is referred to in the content
// stream. Normally names are allocated automatically, and use of this
// function is not required.
func (b *Builder) SetFontNameInternal(f font.Instance, name pdf.Name) error {
	key := resKey{resFont, f}
	if _, exists := b.resName[key]; exists {
		return fmt.Errorf("font already has a name assigned")
	}
	if b.Resources.Font == nil {
		b.Resources.Font = make(map[pdf.Name]font.Instance)
	}
	if _, exists := b.Resources.Font[name]; exists {
		return fmt.Errorf("font name %q already in use", name)
	}
	b.Resources.Font[name] = f
	b.resName[key] = name
	return nil
}

// Text Positioning Operators

// TextFirstLine moves to the start of the next line of text.
// The new text position is (x, y), relative to the start of the current line
// (or to the current point if there is no current line).
//
// This implements the PDF graphics operator "Td".
func (b *Builder) TextFirstLine(x, y float64) {
	b.emit(content.OpTextMoveOffset, pdf.Number(x), pdf.Number(y))
}

// TextSecondLine moves to the point (dx, dy) relative to the start of the
// current line of text. The function also sets the leading to -dy.
// Usually, dy is negative.
//
// This implements the PDF graphics operator "TD".
func (b *Builder) TextSecondLine(dx, dy float64) {
	b.emit(content.OpTextMoveOffsetSetLeading, pdf.Number(dx), pdf.Number(dy))
}

// TextSetMatrix sets the text matrix and text line matrix.
//
// This implements the PDF graphics operator "Tm".
func (b *Builder) TextSetMatrix(m matrix.Matrix) {
	b.emit(content.OpTextSetMatrix,
		pdf.Number(m[0]), pdf.Number(m[1]),
		pdf.Number(m[2]), pdf.Number(m[3]),
		pdf.Number(m[4]), pdf.Number(m[5]))
}

// TextNextLine moves to the start of the next line.
//
// This implements the PDF graphics operator "T*".
func (b *Builder) TextNextLine() {
	b.emit(content.OpTextNextLine)
}

// Text Showing Operators

// TextShowRaw shows an already encoded text in the PDF file.
//
// This implements the PDF graphics operator "Tj".
func (b *Builder) TextShowRaw(s pdf.String) {
	b.updateTextPosition(s)
	// Clone the string to avoid aliasing if caller reuses the slice
	b.emit(content.OpTextShow, cloneString(s))
}

// TextShowNextLineRaw starts a new line and then shows an already encoded text
// in the PDF file. This has the same effect as [Builder.TextNextLine] followed
// by [Builder.TextShowRaw].
//
// This implements the PDF graphics operator "'".
func (b *Builder) TextShowNextLineRaw(s pdf.String) {
	// Clone the string to avoid aliasing if caller reuses the slice
	// emit() handles line matrix update via applyOperatorToParams
	b.emit(content.OpTextShowMoveNextLine, cloneString(s))
	// Then advance for glyph widths
	b.updateTextPosition(s)
}

// TextShowSpacedRaw adjusts word and character spacing and then shows an
// already encoded text in the PDF file. This has the same effect as
// [Builder.TextSetWordSpacing] and [Builder.TextSetCharacterSpacing], followed
// by [Builder.TextShowRaw].
//
// This implements the PDF graphics operator '"'.
func (b *Builder) TextShowSpacedRaw(wordSpacing, charSpacing float64, s pdf.String) {
	// Clone the string to avoid aliasing if caller reuses the slice
	// emit() handles spacing updates and line matrix update via applyOperatorToParams
	b.emit(content.OpTextShowMoveNextLineSetSpacing,
		pdf.Number(wordSpacing), pdf.Number(charSpacing), cloneString(s))
	// Then advance for glyph widths (using the new spacing values)
	b.updateTextPosition(s)
}

// TextShowKernedRaw shows an already encoded text in the PDF file, using
// kerning information provided to adjust glyph spacing.
//
// The arguments must be of type [pdf.String], [pdf.Real], [pdf.Integer] or
// [pdf.Number].
//
// This implements the PDF graphics operator "TJ".
func (b *Builder) TextShowKernedRaw(args ...pdf.Object) {
	if b.Err != nil {
		return
	}
	wMode := font.Horizontal
	if b.State.GState.TextFont != nil {
		wMode = b.State.GState.TextFont.WritingMode()
	}
	// Clone strings to avoid aliasing if caller reuses slices
	clonedArgs := make(pdf.Array, len(args))
	for i, arg := range args {
		switch arg := arg.(type) {
		case pdf.String:
			b.updateTextPosition(arg)
			clonedArgs[i] = cloneString(arg)
		case pdf.Real:
			b.applyTextKern(float64(arg), wMode)
			clonedArgs[i] = arg
		case pdf.Integer:
			b.applyTextKern(float64(arg), wMode)
			clonedArgs[i] = arg
		case pdf.Number:
			b.applyTextKern(float64(arg), wMode)
			clonedArgs[i] = arg
		default:
			b.Err = fmt.Errorf("TextShowKernedRaw: invalid argument type %T", arg)
			return
		}
	}
	b.emit(content.OpTextShowArray, clonedArgs)
}

// applyTextKern applies a kerning adjustment to the text matrix.
func (b *Builder) applyTextKern(delta float64, wMode font.WritingMode) {
	if delta == 0 {
		return
	}
	delta *= -b.State.GState.TextFontSize / 1000
	if wMode == font.Horizontal {
		b.State.GState.TextMatrix = matrix.Translate(delta*b.State.GState.TextHorizontalScaling, 0).Mul(b.State.GState.TextMatrix)
	} else {
		b.State.GState.TextMatrix = matrix.Translate(0, delta).Mul(b.State.GState.TextMatrix)
	}
}

// updateTextPosition advances the text matrix based on the glyphs in the string.
func (b *Builder) updateTextPosition(s pdf.String) {
	if b.State.GState.TextFont == nil {
		return
	}

	wmode := b.State.GState.TextFont.WritingMode()
	for info := range b.State.GState.TextFont.Codes(s) {
		width := info.Width*b.State.GState.TextFontSize + b.State.GState.TextCharacterSpacing
		if info.UseWordSpacing {
			width += b.State.GState.TextWordSpacing
		}
		if wmode == font.Horizontal {
			width *= b.State.GState.TextHorizontalScaling
		}

		switch wmode {
		case font.Horizontal:
			b.State.GState.TextMatrix = matrix.Translate(width, 0).Mul(b.State.GState.TextMatrix)
		case font.Vertical:
			b.State.GState.TextMatrix = matrix.Translate(0, width).Mul(b.State.GState.TextMatrix)
		}
	}
}

// GetTextPositionUser returns the current text position in user coordinates.
func (b *Builder) GetTextPositionUser() (float64, float64) {
	p := b.State.GState
	M := matrix.Matrix{
		p.TextFontSize * p.TextHorizontalScaling, 0,
		0, p.TextFontSize,
		0, p.TextRise,
	}
	M = M.Mul(p.TextMatrix)
	return M[4], M[5]
}

// GetTextPositionDevice returns the current text position in device coordinates.
func (b *Builder) GetTextPositionDevice() (float64, float64) {
	p := b.State.GState
	M := matrix.Matrix{
		p.TextFontSize * p.TextHorizontalScaling, 0,
		0, p.TextFontSize,
		0, p.TextRise,
	}
	M = M.Mul(p.TextMatrix)
	M = M.Mul(p.CTM)
	return M[4], M[5]
}

// cloneString creates a copy of a pdf.String to avoid aliasing issues
// when callers reuse the same slice backing array.
func cloneString(s pdf.String) pdf.String {
	if s == nil {
		return nil
	}
	clone := make(pdf.String, len(s))
	copy(clone, s)
	return clone
}
