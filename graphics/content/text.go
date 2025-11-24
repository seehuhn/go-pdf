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

package content

import (
	"errors"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// handleTextBegin implements the BT operator (begin text object)
func handleTextBegin(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPage {
		return errors.New("BT: not in page context")
	}

	s.CurrentObject = ObjText
	s.Param.TextMatrix = matrix.Identity
	s.Param.TextLineMatrix = matrix.Identity
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextEnd implements the ET operator (end text object)
func handleTextEnd(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjText {
		return errors.New("not in text object")
	}

	s.CurrentObject = ObjPage
	s.Out &= ^graphics.StateTextMatrix

	return nil
}

// handleTextSetCharSpacing implements the Tc operator
func handleTextSetCharSpacing(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	spacing := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.TextCharacterSpacing = spacing
	s.markOut(graphics.StateTextCharacterSpacing)
	return nil
}

// handleTextSetWordSpacing implements the Tw operator
func handleTextSetWordSpacing(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	spacing := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.TextWordSpacing = spacing
	s.markOut(graphics.StateTextWordSpacing)
	return nil
}

// handleTextSetHorizontalScaling implements the Tz operator
func handleTextSetHorizontalScaling(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	scale := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.TextHorizontalScaling = scale / 100.0 // PDF uses percentage
	s.markOut(graphics.StateTextHorizontalScaling)
	return nil
}

// handleTextSetLeading implements the TL operator
func handleTextSetLeading(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	leading := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.TextLeading = leading
	s.markOut(graphics.StateTextLeading)
	return nil
}

// handleTextSetFont implements the Tf operator
func handleTextSetFont(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	name := p.GetName()
	size := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	fontInstance, ok := res.Font[name]
	if !ok {
		return errors.New("font not found")
	}

	s.Param.TextFont = fontInstance
	s.Param.TextFontSize = size
	s.markOut(graphics.StateTextFont)
	return nil
}

// handleTextSetRenderingMode implements the Tr operator
func handleTextSetRenderingMode(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	mode := p.GetInt()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.TextRenderingMode = graphics.TextRenderingMode(mode)
	s.markOut(graphics.StateTextRenderingMode)
	return nil
}

// handleTextSetRise implements the Ts operator
func handleTextSetRise(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	rise := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.TextRise = rise
	s.markOut(graphics.StateTextRise)
	return nil
}

// handleTextMoveOffset implements the Td operator
func handleTextMoveOffset(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	tx := p.GetFloat()
	ty := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjText {
		return errors.New("not in text object")
	}

	// Translate text line matrix
	s.Param.TextLineMatrix = s.Param.TextLineMatrix.Mul(matrix.Matrix{1, 0, 0, 1, tx, ty})
	s.Param.TextMatrix = s.Param.TextLineMatrix
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextMoveOffsetSetLeading implements the TD operator
func handleTextMoveOffsetSetLeading(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	tx := p.GetFloat()
	ty := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjText {
		return errors.New("not in text object")
	}

	// Set leading
	s.Param.TextLeading = -ty
	s.markOut(graphics.StateTextLeading)

	// Move text position
	s.Param.TextLineMatrix = s.Param.TextLineMatrix.Mul(matrix.Matrix{1, 0, 0, 1, tx, ty})
	s.Param.TextMatrix = s.Param.TextLineMatrix
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextSetMatrix implements the Tm operator
func handleTextSetMatrix(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	a := p.GetFloat()
	b := p.GetFloat()
	c := p.GetFloat()
	d := p.GetFloat()
	e := p.GetFloat()
	f := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjText {
		return errors.New("not in text object")
	}

	m := matrix.Matrix{a, b, c, d, e, f}
	s.Param.TextMatrix = m
	s.Param.TextLineMatrix = m
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextNextLine implements the T* operator
func handleTextNextLine(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjText {
		return errors.New("not in text object")
	}

	// Mark dependency on TextLeading
	s.markIn(graphics.StateTextLeading)

	// Move to next line
	leading := s.Param.TextLeading
	s.Param.TextLineMatrix = s.Param.TextLineMatrix.Mul(matrix.Matrix{1, 0, 0, 1, 0, -leading})
	s.Param.TextMatrix = s.Param.TextLineMatrix
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextShow implements the Tj operator
func handleTextShow(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	_ = p.GetString() // text to show
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjText {
		return errors.New("not in text object")
	}

	s.markIn(graphics.StateTextFont | graphics.StateTextMatrix)

	// Dependencies based on rendering mode
	mode := s.Param.TextRenderingMode
	if mode == graphics.TextRenderingModeFill ||
		mode == graphics.TextRenderingModeFillStroke ||
		mode == graphics.TextRenderingModeFillClip ||
		mode == graphics.TextRenderingModeFillStrokeClip {
		s.markIn(graphics.StateFillColor)
	}
	if mode == graphics.TextRenderingModeStroke ||
		mode == graphics.TextRenderingModeFillStroke ||
		mode == graphics.TextRenderingModeStrokeClip ||
		mode == graphics.TextRenderingModeFillStrokeClip {
		s.markIn(graphics.StateStrokeColor | graphics.StateLineWidth |
			graphics.StateLineJoin | graphics.StateLineCap)
	}

	s.markOut(graphics.StateTextMatrix)
	return nil
}

// handleTextShowArray implements the TJ operator
func handleTextShowArray(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	_ = p.GetArray() // array of strings and numbers
	if err := p.Check(); err != nil {
		return err
	}

	// Same dependencies as Tj
	return handleTextShow(s, []pdf.Object{pdf.String("")}, res)
}

// handleTextShowMoveNextLine implements the ' operator
func handleTextShowMoveNextLine(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	text := p.GetString()
	if err := p.Check(); err != nil {
		return err
	}

	// Equivalent to: T* Tj
	if err := handleTextNextLine(s, nil, res); err != nil {
		return err
	}
	return handleTextShow(s, []pdf.Object{text}, res)
}

// handleTextShowMoveNextLineSetSpacing implements the " operator
func handleTextShowMoveNextLineSetSpacing(s *GraphicsState, args []pdf.Object, res *Resources) error {
	p := argParser{args: args}
	aw := p.GetFloat()
	ac := p.GetFloat()
	text := p.GetString()
	if err := p.Check(); err != nil {
		return err
	}

	// Equivalent to: aw Tw ac Tc string '
	s.Param.TextWordSpacing = aw
	s.markOut(graphics.StateTextWordSpacing)
	s.Param.TextCharacterSpacing = ac
	s.markOut(graphics.StateTextCharacterSpacing)

	return handleTextShowMoveNextLine(s, []pdf.Object{text}, res)
}
