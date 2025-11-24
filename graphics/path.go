// seehuhn.de/go/pdf - a library for reading and writing PDF files
//go:build exclude
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

package graphics

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// handleMoveTo implements the m operator (begin new subpath)
func handleMoveTo(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	x := p.GetFloat()
	y := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPage && s.CurrentObject != ObjPath {
		return errors.New("m: invalid context")
	}

	// Starting a new path from page context
	if s.CurrentObject == ObjPage {
		s.Param.AllSubpathsClosed = true
	}

	// Finalize any existing open subpath
	if s.CurrentObject == ObjPath && !s.Param.ThisSubpathClosed {
		s.Param.AllSubpathsClosed = false
	}

	s.CurrentObject = ObjPath
	s.Param.StartX, s.Param.StartY = x, y
	s.Param.CurrentX, s.Param.CurrentY = x, y
	s.Param.ThisSubpathClosed = false

	return nil
}

// handleLineTo implements the l operator (append straight line)
func handleLineTo(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	x := p.GetFloat()
	y := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath {
		return errors.New("not in path context")
	}

	s.Param.CurrentX, s.Param.CurrentY = x, y
	return nil
}

// handleCurveTo implements the c operator (append Bezier curve)
func handleCurveTo(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	_ = p.GetFloat() // x1
	_ = p.GetFloat() // y1
	_ = p.GetFloat() // x2
	_ = p.GetFloat() // y2
	x3 := p.GetFloat()
	y3 := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath {
		return errors.New("not in path context")
	}

	s.Param.CurrentX, s.Param.CurrentY = x3, y3
	return nil
}

// handleCurveToV implements the v operator (Bezier curve, initial point replicated)
func handleCurveToV(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	_ = p.GetFloat() // x2
	_ = p.GetFloat() // y2
	x3 := p.GetFloat()
	y3 := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath {
		return errors.New("not in path context")
	}

	s.Param.CurrentX, s.Param.CurrentY = x3, y3
	return nil
}

// handleCurveToY implements the y operator (Bezier curve, final point replicated)
func handleCurveToY(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	_ = p.GetFloat() // x1
	_ = p.GetFloat() // y1
	x3 := p.GetFloat()
	y3 := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath {
		return errors.New("not in path context")
	}

	s.Param.CurrentX, s.Param.CurrentY = x3, y3
	return nil
}

// handleClosePath implements the h operator (close current subpath)
func handleClosePath(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath {
		return errors.New("not in path context")
	}

	s.Param.CurrentX = s.Param.StartX
	s.Param.CurrentY = s.Param.StartY
	s.Param.ThisSubpathClosed = true
	return nil
}

// handleRectangle implements the re operator (append rectangle)
func handleRectangle(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	x := p.GetFloat()
	y := p.GetFloat()
	_ = p.GetFloat() // width
	_ = p.GetFloat() // height
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPage && s.CurrentObject != ObjPath {
		return errors.New("re: invalid context")
	}

	// Starting a new path from page context
	if s.CurrentObject == ObjPage {
		s.Param.AllSubpathsClosed = true
	}

	// Finalize any existing open subpath
	if s.CurrentObject == ObjPath && !s.Param.ThisSubpathClosed {
		s.Param.AllSubpathsClosed = false
	}

	s.CurrentObject = ObjPath
	// Rectangle creates a closed subpath
	s.Param.StartX, s.Param.StartY = x, y
	s.Param.CurrentX, s.Param.CurrentY = x, y
	s.Param.ThisSubpathClosed = true

	return nil
}

// handleStroke implements the S operator (stroke path)
func handleStroke(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath && s.CurrentObject != ObjClippingPath {
		return errors.New("not in path context")
	}

	// Finalize current subpath
	if !s.Param.ThisSubpathClosed {
		s.Param.AllSubpathsClosed = false
	}

	// Mark dependencies
	s.markIn(StateLineWidth | StateLineJoin |
		StateLineDash | StateStrokeColor)

	// Conditional dependency on LineCap
	if !s.Param.AllSubpathsClosed || len(s.Param.DashPattern) > 0 {
		s.markIn(StateLineCap)
	}

	// Reset path
	s.CurrentObject = ObjPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleCloseAndStroke implements the s operator (close and stroke path)
func handleCloseAndStroke(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath && s.CurrentObject != ObjClippingPath {
		return errors.New("not in path context")
	}

	// Close current subpath
	s.Param.CurrentX = s.Param.StartX
	s.Param.CurrentY = s.Param.StartY
	s.Param.ThisSubpathClosed = true

	// Mark dependencies (same as S)
	s.markIn(StateLineWidth | StateLineJoin |
		StateLineDash | StateStrokeColor)

	// Conditional dependency on LineCap
	if !s.Param.AllSubpathsClosed || len(s.Param.DashPattern) > 0 {
		s.markIn(StateLineCap)
	}

	s.CurrentObject = ObjPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleFill implements the f operator (fill path using nonzero winding rule)
func handleFill(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath && s.CurrentObject != ObjClippingPath {
		return errors.New("not in path context")
	}

	s.markIn(StateFillColor)
	s.CurrentObject = ObjPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleFillCompat implements the F operator (deprecated alias for f)
func handleFillCompat(s *State, args []pdf.Native, res interface{}) error {
	return handleFill(s, args, res)
}

// handleFillEvenOdd implements the f* operator (fill using even-odd rule)
func handleFillEvenOdd(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath && s.CurrentObject != ObjClippingPath {
		return errors.New("not in path context")
	}

	s.markIn(StateFillColor)
	s.CurrentObject = ObjPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleFillAndStroke implements the B operator (fill and stroke, nonzero)
func handleFillAndStroke(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath && s.CurrentObject != ObjClippingPath {
		return errors.New("not in path context")
	}

	if !s.Param.ThisSubpathClosed {
		s.Param.AllSubpathsClosed = false
	}

	s.markIn(StateFillColor | StateLineWidth |
		StateLineJoin | StateLineDash | StateStrokeColor)

	if !s.Param.AllSubpathsClosed || len(s.Param.DashPattern) > 0 {
		s.markIn(StateLineCap)
	}

	s.CurrentObject = ObjPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleFillAndStrokeEvenOdd implements the B* operator
func handleFillAndStrokeEvenOdd(s *State, args []pdf.Native, res interface{}) error {
	return handleFillAndStroke(s, args, res)
}

// handleCloseFillAndStroke implements the b operator
func handleCloseFillAndStroke(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath && s.CurrentObject != ObjClippingPath {
		return errors.New("not in path context")
	}

	s.Param.CurrentX = s.Param.StartX
	s.Param.CurrentY = s.Param.StartY
	s.Param.ThisSubpathClosed = true

	s.markIn(StateFillColor | StateLineWidth |
		StateLineJoin | StateLineDash | StateStrokeColor)

	if !s.Param.AllSubpathsClosed || len(s.Param.DashPattern) > 0 {
		s.markIn(StateLineCap)
	}

	s.CurrentObject = ObjPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleCloseFillAndStrokeEvenOdd implements the b* operator
func handleCloseFillAndStrokeEvenOdd(s *State, args []pdf.Native, res interface{}) error {
	return handleCloseFillAndStroke(s, args, res)
}

// handleEndPath implements the n operator (end path without painting)
func handleEndPath(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath && s.CurrentObject != ObjClippingPath {
		return errors.New("not in path context")
	}

	s.CurrentObject = ObjPage
	s.Param.AllSubpathsClosed = true

	return nil
}

// handleClip implements the W operator (set clipping path, nonzero)
func handleClip(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath {
		return errors.New("not in path context")
	}

	s.CurrentObject = ObjClippingPath
	return nil
}

// handleClipEvenOdd implements the W* operator (set clipping path, even-odd)
func handleClipEvenOdd(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if s.CurrentObject != ObjPath {
		return errors.New("not in path context")
	}

	s.CurrentObject = ObjClippingPath
	return nil
}
