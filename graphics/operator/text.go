package operator

import (
	"errors"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/resource"
)

// handleTextBegin implements the BT operator (begin text object)
func handleTextBegin(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objPage {
		return errors.New("BT: not in page context")
	}

	s.CurrentObject = objText
	s.Param.TextMatrix = matrix.Identity
	s.Param.TextLineMatrix = matrix.Identity
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextEnd implements the ET operator (end text object)
func handleTextEnd(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objText {
		return errors.New("not in text object")
	}

	s.CurrentObject = objPage
	s.Out &= ^graphics.StateTextMatrix

	return nil
}

// handleTextSetCharSpacing implements the Tc operator
func handleTextSetCharSpacing(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleTextSetWordSpacing(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleTextSetHorizontalScaling(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleTextSetLeading(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleTextSetFont(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	name := p.GetName()
	size := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if res.Font == nil {
		return errors.New("no font resources available")
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
func handleTextSetRenderingMode(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleTextSetRise(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleTextMoveOffset(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	tx := p.GetFloat()
	ty := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objText {
		return errors.New("not in text object")
	}

	// Translate text line matrix
	s.Param.TextLineMatrix = s.Param.TextLineMatrix.Mul(matrix.Matrix{1, 0, 0, 1, tx, ty})
	s.Param.TextMatrix = s.Param.TextLineMatrix
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextMoveOffsetSetLeading implements the TD operator
func handleTextMoveOffsetSetLeading(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	tx := p.GetFloat()
	ty := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objText {
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
func handleTextSetMatrix(s *State, args []pdf.Native, res *resource.Resource) error {
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

	if s.CurrentObject != objText {
		return errors.New("not in text object")
	}

	m := matrix.Matrix{a, b, c, d, e, f}
	s.Param.TextMatrix = m
	s.Param.TextLineMatrix = m
	s.markOut(graphics.StateTextMatrix)

	return nil
}

// handleTextNextLine implements the T* operator
func handleTextNextLine(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objText {
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
func handleTextShow(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetString() // text to show
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != objText {
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
func handleTextShowArray(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetArray() // array of strings and numbers
	if err := p.Check(); err != nil {
		return err
	}

	// Same dependencies as Tj
	return handleTextShow(s, []pdf.Native{pdf.String("")}, res)
}

// handleTextShowMoveNextLine implements the ' operator
func handleTextShowMoveNextLine(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	text := p.GetString()
	if err := p.Check(); err != nil {
		return err
	}

	// Equivalent to: T* Tj
	if err := handleTextNextLine(s, nil, res); err != nil {
		return err
	}
	return handleTextShow(s, []pdf.Native{text}, res)
}

// handleTextShowMoveNextLineSetSpacing implements the " operator
func handleTextShowMoveNextLineSetSpacing(s *State, args []pdf.Native, res *resource.Resource) error {
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

	return handleTextShowMoveNextLine(s, []pdf.Native{text}, res)
}
