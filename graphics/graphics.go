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
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/internal/float"
)

// Transform applies a transformation matrix to the coordinate system.
// This function modifies the current transformation matrix by multiplying the
// given matrix from the right.
//
// This implementes the "cm" PDF graphics operator.
func (p *Page) Transform(m Matrix) {
	if !p.valid("Transform", objPage, objText) {
		return
	}
	p.state.CTM = p.state.CTM.Mul(m)
	_, p.Err = fmt.Fprintln(p.Content,
		float.Format(m[0], 3), float.Format(m[1], 3),
		float.Format(m[2], 3), float.Format(m[3], 3),
		float.Format(m[4], 3), float.Format(m[5], 3), "cm")
}

// SetStrokeColor sets the stroke color in the graphics state.
// If col is nil, the stroke color is not changed.
func (p *Page) SetStrokeColor(col color.Color) {
	if !p.valid("SetStrokeColor", objPage, objText) {
		return
	}
	if p.isSet(StateStrokeColor) && col == p.state.StrokeColor {
		return
	}
	p.state.StrokeColor = col
	p.set |= StateStrokeColor
	p.Err = col.SetStroke(p.Content)
}

// SetFillColor sets the fill color in the graphics state.
// If col is nil, the fill color is not changed.
func (p *Page) SetFillColor(col color.Color) {
	if !p.valid("SetFillColor", objPage, objText) {
		return
	}
	if p.isSet(StateFillColor) && col == p.state.FillColor {
		return
	}
	p.state.FillColor = col
	p.set |= StateFillColor
	p.Err = col.SetFill(p.Content)
}

// SetLineWidth sets the line width.
func (p *Page) SetLineWidth(width float64) {
	if !p.valid("SetLineWidth", objPage, objText) {
		return
	}
	if p.isSet(StateLineWidth) && math.Abs(width-p.state.LineWidth) < ε {
		return
	}
	p.state.LineWidth = width
	p.set |= StateLineWidth
	_, p.Err = fmt.Fprintln(p.Content, p.coord(width), "w")
}

// SetLineCap sets the line cap style.
func (p *Page) SetLineCap(cap LineCapStyle) {
	if !p.valid("SetLineCap", objPage, objText) {
		return
	}
	if p.isSet(StateLineCap) && cap == p.state.LineCap {
		return
	}
	p.state.LineCap = cap
	p.set |= StateLineCap
	_, p.Err = fmt.Fprintln(p.Content, int(cap), "J")
}

// SetLineJoin sets the line join style.
func (p *Page) SetLineJoin(join LineJoinStyle) {
	if !p.valid("SetLineJoin", objPage, objText) {
		return
	}
	if p.isSet(StateLineJoin) && join == p.state.LineJoin {
		return
	}
	p.state.LineJoin = join
	p.set |= StateLineJoin
	_, p.Err = fmt.Fprintln(p.Content, int(join), "j")
}

// SetMiterLimit sets the miter limit.
func (p *Page) SetMiterLimit(limit float64) {
	if !p.valid("SetMiterLimit", objPage, objText) {
		return
	}
	if p.isSet(StateMiterLimit) && math.Abs(limit-p.state.MiterLimit) < ε {
		return
	}
	p.state.MiterLimit = limit
	p.set |= StateMiterLimit
	_, p.Err = fmt.Fprintln(p.Content, float.Format(limit, 3), "M")
}

// SetDashPattern sets the line dash pattern.
func (p *Page) SetDashPattern(phase float64, pattern ...float64) {
	if !p.valid("SetDashPattern", objPage, objText) {
		return
	}

	if p.isSet(StateDash) &&
		sliceNearlyEqual(pattern, p.state.DashPattern) &&
		math.Abs(phase-p.state.DashPhase) < ε {
		return
	}
	p.state.DashPattern = pattern
	p.state.DashPhase = phase
	p.set |= StateDash
	// TODO(voss): when we stroke lines, make sure to either update the
	// phase, or at least to unset the StateDash bit.

	_, p.Err = fmt.Fprint(p.Content, "[")
	if p.Err != nil {
		return
	}
	sep := ""
	for _, x := range pattern {
		_, p.Err = fmt.Fprint(p.Content, sep, float.Format(x, 3))
		if p.Err != nil {
			return
		}
		sep = " "
	}
	_, p.Err = fmt.Fprint(p.Content, "] ", float.Format(phase, 3), " d\n")
	if p.Err != nil {
		return
	}
}

// SetExtGState sets selected graphics state parameters.
// The argument dictName must be the name of a graphics state dictionary
// that has been defined using the [Page.AddExtGState] method.
func (p *Page) SetExtGState(dictName pdf.Name) {
	// TODO(voss): keep track of the graphics state

	if !p.valid("SetGraphicsState", objPage, objText) {
		return
	}

	err := dictName.PDF(p.Content)
	if err != nil {
		p.Err = err
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, " gs")
}

// AddExtGState adds a new graphics state dictionary.
func (p *Page) AddExtGState(name pdf.Name, dict pdf.Dict) {
	// TODO(voss): keep track of the graphics state

	if p.Resources.ExtGState == nil {
		p.Resources.ExtGState = pdf.Dict{}
	}
	p.Resources.ExtGState[name] = dict
}

// See table 57 in ISO 32000-2:2020.
func ExtGStateDict(s *State, set StateBits) pdf.Dict {
	res := pdf.Dict{}
	if set&StateLineWidth != 0 {
		res["LW"] = pdf.Number(s.LineWidth)
	}
	if set&StateLineCap != 0 {
		res["LC"] = pdf.Integer(s.LineCap)
	}
	if set&StateLineJoin != 0 {
		res["LJ"] = pdf.Integer(s.LineJoin)
	}
	if set&StateMiterLimit != 0 {
		res["ML"] = pdf.Number(s.MiterLimit)
	}
	if set&StateDash != 0 {
		pat := make(pdf.Array, len(s.DashPattern))
		for i, x := range s.DashPattern {
			pat[i] = pdf.Number(x)
		}
		res["D"] = pdf.Array{
			pat,
			pdf.Number(s.DashPhase),
		}
	}
	if set&StateRenderingIntent != 0 {
		res["RI"] = s.RenderingIntent
	}
	if set&StateOverprint != 0 {
		res["OP"] = pdf.Boolean(s.OverprintStroke)
		if s.OverprintFill != s.OverprintStroke {
			res["op"] = pdf.Boolean(s.OverprintFill)
		}
	}
	if set&StateOverprintMode != 0 {
		res["OPM"] = pdf.Integer(s.OverprintMode)
	}
	if set&StateFont != 0 {
		res["Font"] = pdf.Array{
			s.Font.Reference(),
			pdf.Number(s.FontSize),
		}
	}

	// TODO(voss): black generation
	// TODO(voss): undercolor removal
	// TODO(voss): transfer function
	// TODO(voss): halftone

	if set&StateFlatnessTolerance != 0 {
		res["FL"] = pdf.Number(s.FlatnessTolerance)
	}
	if set&StateSmoothnessTolerance != 0 {
		res["SM"] = pdf.Number(s.SmoothnessTolerance)
	}
	if set&StateStrokeAdjustment != 0 {
		res["SA"] = pdf.Boolean(s.StrokeAdjustment)
	}
	if set&StateBlendMode != 0 {
		res["BM"] = s.BlendMode
	}
	if set&StateSoftMask != 0 {
		res["SMask"] = s.SoftMask
	}
	if set&StateStrokeAlpha != 0 {
		res["CA"] = pdf.Number(s.StrokeAlpha)
	}
	if set&StateFillAlpha != 0 {
		res["ca"] = pdf.Number(s.FillAlpha)
	}
	if set&StateAlphaSourceFlag != 0 {
		res["AIS"] = pdf.Boolean(s.AlphaSourceFlag)
	}

	return res
}

func sliceNearlyEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		if math.Abs(x-b[i]) > ε {
			return false
		}
	}
	return true
}

const ε = 1e-6
