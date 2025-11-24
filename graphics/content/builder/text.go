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
	if b.State.Out&graphics.StateTextCharacterSpacing != 0 &&
		nearlyEqual(charSpacing, b.State.Param.TextCharacterSpacing) {
		return
	}
	b.emit(content.OpTextSetCharacterSpacing, pdf.Number(charSpacing))
}

// TextSetWordSpacing sets additional word spacing.
//
// This implements the PDF graphics operator "Tw".
func (b *Builder) TextSetWordSpacing(wordSpacing float64) {
	if b.State.Out&graphics.StateTextWordSpacing != 0 &&
		nearlyEqual(wordSpacing, b.State.Param.TextWordSpacing) {
		return
	}
	b.emit(content.OpTextSetWordSpacing, pdf.Number(wordSpacing))
}

// TextSetHorizontalScaling sets the horizontal scaling.
// The value 1 corresponds to normal scaling.
//
// This implements the PDF graphics operator "Tz".
func (b *Builder) TextSetHorizontalScaling(scaling float64) {
	if b.State.Out&graphics.StateTextHorizontalScaling != 0 &&
		nearlyEqual(scaling, b.State.Param.TextHorizontalScaling) {
		return
	}
	// PDF operator expects percentage (100 = normal)
	b.emit(content.OpTextSetHorizontalScaling, pdf.Number(scaling*100))
}

// TextSetLeading sets the text leading.
//
// This implements the PDF graphics operator "TL".
func (b *Builder) TextSetLeading(leading float64) {
	if b.State.Out&graphics.StateTextLeading != 0 &&
		nearlyEqual(leading, b.State.Param.TextLeading) {
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
	if b.State.Out&graphics.StateTextRenderingMode != 0 &&
		mode == b.State.Param.TextRenderingMode {
		return
	}
	b.emit(content.OpTextSetRenderingMode, pdf.Integer(mode))
}

// TextSetRise sets the text rise.
//
// This implements the PDF graphics operator "Ts".
func (b *Builder) TextSetRise(rise float64) {
	if b.State.Out&graphics.StateTextRise != 0 &&
		nearlyEqual(rise, b.State.Param.TextRise) {
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

	// look up or allocate resource name
	key := resKey{"F", f}
	name, ok := b.resName[key]
	if !ok {
		if b.Resources.Font == nil {
			b.Resources.Font = make(map[pdf.Name]font.Instance)
		}
		name = allocateName("F", b.Resources.Font)
		b.Resources.Font[name] = f
		b.resName[key] = name
	}

	if b.State.Out&graphics.StateTextFont != 0 &&
		b.State.Param.TextFont == f &&
		nearlyEqual(b.State.Param.TextFontSize, size) {
		return
	}

	b.emit(content.OpTextSetFont, name, pdf.Number(size))
}

// SetFontNameInternal controls how the font is referred to in the content
// stream. Normally names are allocated automatically, and use of this
// function is not required.
func (b *Builder) SetFontNameInternal(f font.Instance, name pdf.Name) error {
	key := resKey{"F", f}
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
	b.emit(content.OpTextShow, s)
}

// TextShowNextLineRaw starts a new line and then shows an already encoded text
// in the PDF file. This has the same effect as [Builder.TextNextLine] followed
// by [Builder.TextShowRaw].
//
// This implements the PDF graphics operator "'".
func (b *Builder) TextShowNextLineRaw(s pdf.String) {
	b.emit(content.OpTextShowMoveNextLine, s)
}

// TextShowSpacedRaw adjusts word and character spacing and then shows an
// already encoded text in the PDF file. This has the same effect as
// [Builder.TextSetWordSpacing] and [Builder.TextSetCharacterSpacing], followed
// by [Builder.TextShowRaw].
//
// This implements the PDF graphics operator '"'.
func (b *Builder) TextShowSpacedRaw(wordSpacing, charSpacing float64, s pdf.String) {
	b.emit(content.OpTextShowMoveNextLineSetSpacing,
		pdf.Number(wordSpacing), pdf.Number(charSpacing), s)
}

// TextShowKernedRaw shows an already encoded text in the PDF file, using
// kerning information provided to adjust glyph spacing.
//
// The arguments must be of type [pdf.String], [pdf.Real], [pdf.Integer] or
// [pdf.Number].
//
// This implements the PDF graphics operator "TJ".
func (b *Builder) TextShowKernedRaw(args ...pdf.Object) {
	b.emit(content.OpTextShowArray, pdf.Array(args))
}
