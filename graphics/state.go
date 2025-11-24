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

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
)

// handlePushState implements the q operator (save graphics state)
func handlePushState(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	s.stack = append(s.stack, savedState{
		param: s.Param.Clone(),
		out:   s.Out,
	})
	return nil
}

// handlePopState implements the Q operator (restore graphics state)
func handlePopState(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}

	if len(s.stack) == 0 {
		return errors.New("no saved state to restore")
	}

	saved := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]

	s.Param = *saved.param
	s.Out = saved.out

	return nil
}

// handleConcatMatrix implements the cm operator (modify CTM)
func handleConcatMatrix(s *State, args []pdf.Native, res interface{}) error {
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

	m := matrix.Matrix{a, b, c, d, e, f}
	s.Param.CTM = s.Param.CTM.Mul(m)
	return nil
}

// handleSetLineWidth implements the w operator
func handleSetLineWidth(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	width := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.LineWidth = width
	s.markOut(StateLineWidth)
	return nil
}

// handleSetLineCap implements the J operator
func handleSetLineCap(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	cap := p.GetInt()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.LineCap = LineCapStyle(cap)
	s.markOut(StateLineCap)
	return nil
}

// handleSetLineJoin implements the j operator
func handleSetLineJoin(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	join := p.GetInt()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.LineJoin = LineJoinStyle(join)
	s.markOut(StateLineJoin)
	return nil
}

// handleSetMiterLimit implements the M operator
func handleSetMiterLimit(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	limit := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.MiterLimit = limit
	s.markOut(StateMiterLimit)
	return nil
}

// handleSetLineDash implements the d operator
func handleSetLineDash(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	arr := p.GetArray()
	phase := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	// convert array to []float64
	pattern := make([]float64, len(arr))
	for i, val := range arr {
		switch v := val.(type) {
		case pdf.Real:
			pattern[i] = float64(v)
		case pdf.Integer:
			pattern[i] = float64(v)
		default:
			return errors.New("dash array must contain numbers")
		}
	}

	s.Param.DashPattern = pattern
	s.Param.DashPhase = phase
	s.markOut(StateLineDash)
	return nil
}

// handleSetRenderingIntent implements the ri operator
func handleSetRenderingIntent(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	intent := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.RenderingIntent = RenderingIntent(intent)
	s.markOut(StateRenderingIntent)
	return nil
}

// handleSetFlatness implements the i operator
func handleSetFlatness(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	flatness := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.FlatnessTolerance = flatness
	s.markOut(StateFlatnessTolerance)
	return nil
}

// handleSetExtGState implements the gs operator
func handleSetExtGState(s *State, args []pdf.Native, res interface{}) error {
	p := argParser{args: args}
	name := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	gs, ok := res.ExtGState[name]
	if !ok {
		return errors.New("ExtGState not found")
	}

	// apply ExtGState parameters to current state
	set := gs.Set
	if set&StateLineWidth != 0 {
		s.Param.LineWidth = gs.LineWidth
		s.markOut(StateLineWidth)
	}
	if set&StateLineCap != 0 {
		s.Param.LineCap = gs.LineCap
		s.markOut(StateLineCap)
	}
	if set&StateLineJoin != 0 {
		s.Param.LineJoin = gs.LineJoin
		s.markOut(StateLineJoin)
	}
	if set&StateMiterLimit != 0 {
		s.Param.MiterLimit = gs.MiterLimit
		s.markOut(StateMiterLimit)
	}
	if set&StateLineDash != 0 {
		s.Param.DashPattern = gs.DashPattern
		s.Param.DashPhase = gs.DashPhase
		s.markOut(StateLineDash)
	}
	if set&StateRenderingIntent != 0 {
		s.Param.RenderingIntent = gs.RenderingIntent
		s.markOut(StateRenderingIntent)
	}
	if set&StateStrokeAdjustment != 0 {
		s.Param.StrokeAdjustment = gs.StrokeAdjustment
		s.markOut(StateStrokeAdjustment)
	}

	return nil
}
