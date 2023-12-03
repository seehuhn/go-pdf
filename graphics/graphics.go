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

	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/internal/float"
)

// Transform applies a transformation matrix to the coordinate system.
// This function modifies the current transformation matrix by multiplying the
// given matrix from the right.
//
// This implementes the "cm" PDF graphics operator.
func (p *Writer) Transform(m Matrix) {
	if !p.valid("Transform", objPage, objText) {
		return
	}
	p.CTM = p.CTM.Mul(m)
	_, p.Err = fmt.Fprintln(p.Content,
		float.Format(m[0], 3), float.Format(m[1], 3),
		float.Format(m[2], 3), float.Format(m[3], 3),
		float.Format(m[4], 3), float.Format(m[5], 3), "cm")
}

// SetStrokeColor sets the stroke color in the graphics state.
// If col is nil, the stroke color is not changed.
func (p *Writer) SetStrokeColor(col color.Color) {
	if !p.valid("SetStrokeColor", objPage, objText) {
		return
	}
	if p.isSet(StateStrokeColor) && col == p.StrokeColor {
		return
	}
	p.StrokeColor = col
	p.Set |= StateStrokeColor
	p.Err = col.SetStroke(p.Content)
}

// SetFillColor sets the fill color in the graphics state.
// If col is nil, the fill color is not changed.
func (p *Writer) SetFillColor(col color.Color) {
	if !p.valid("SetFillColor", objPage, objText) {
		return
	}
	if p.isSet(StateFillColor) && col == p.FillColor {
		return
	}
	p.FillColor = col
	p.Set |= StateFillColor
	p.Err = col.SetFill(p.Content)
}

// SetLineWidth sets the line width.
func (p *Writer) SetLineWidth(width float64) {
	if !p.valid("SetLineWidth", objPage, objText) {
		return
	}
	if p.isSet(StateLineWidth) && nearlyEqual(width, p.LineWidth) {
		return
	}
	p.LineWidth = width
	p.Set |= StateLineWidth
	_, p.Err = fmt.Fprintln(p.Content, p.coord(width), "w")
}

// SetLineCap sets the line cap style.
func (p *Writer) SetLineCap(cap LineCapStyle) {
	if !p.valid("SetLineCap", objPage, objText) {
		return
	}
	if p.isSet(StateLineCap) && cap == p.LineCap {
		return
	}
	p.LineCap = cap
	p.Set |= StateLineCap
	_, p.Err = fmt.Fprintln(p.Content, int(cap), "J")
}

// SetLineJoin sets the line join style.
func (p *Writer) SetLineJoin(join LineJoinStyle) {
	if !p.valid("SetLineJoin", objPage, objText) {
		return
	}
	if p.isSet(StateLineJoin) && join == p.LineJoin {
		return
	}
	p.LineJoin = join
	p.Set |= StateLineJoin
	_, p.Err = fmt.Fprintln(p.Content, int(join), "j")
}

// SetMiterLimit sets the miter limit.
func (p *Writer) SetMiterLimit(limit float64) {
	if !p.valid("SetMiterLimit", objPage, objText) {
		return
	}
	if p.isSet(StateMiterLimit) && nearlyEqual(limit, p.MiterLimit) {
		return
	}
	p.MiterLimit = limit
	p.Set |= StateMiterLimit
	_, p.Err = fmt.Fprintln(p.Content, float.Format(limit, 3), "M")
}

// SetDashPattern sets the line dash pattern.
func (p *Writer) SetDashPattern(phase float64, pattern ...float64) {
	if !p.valid("SetDashPattern", objPage, objText) {
		return
	}

	if p.isSet(StateDash) &&
		sliceNearlyEqual(pattern, p.DashPattern) &&
		nearlyEqual(phase, p.DashPhase) {
		return
	}
	p.DashPattern = pattern
	p.DashPhase = phase
	p.Set |= StateDash

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

func nearlyEqual(a, b float64) bool {
	const ε = 1e-6
	return math.Abs(a-b) < ε
}

func sliceNearlyEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, x := range a {
		if nearlyEqual(x, b[i]) {
			return false
		}
	}
	return true
}
