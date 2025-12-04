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
	"math"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
)

// PushGraphicsState saves the current graphics state.
//
// This implements the PDF graphics operator "q".
func (b *Builder) PushGraphicsState() {
	b.emit(content.OpPushGraphicsState)
}

// PopGraphicsState restores the previous graphics state.
//
// This implements the PDF graphics operator "Q".
func (b *Builder) PopGraphicsState() {
	b.emit(content.OpPopGraphicsState)
}

// Transform applies a transformation matrix to the coordinate system.
//
// This implements the PDF graphics operator "cm".
func (b *Builder) Transform(m matrix.Matrix) {
	b.emit(content.OpTransform,
		pdf.Number(m[0]), pdf.Number(m[1]),
		pdf.Number(m[2]), pdf.Number(m[3]),
		pdf.Number(m[4]), pdf.Number(m[5]))
}

// SetLineCap sets the line cap style.
//
// This implements the PDF graphics operator "J".
func (b *Builder) SetLineCap(cap graphics.LineCapStyle) {
	if cap > 2 {
		b.Err = fmt.Errorf("SetLineCap: invalid line cap style %d", cap)
		return
	}
	if b.isKnown(graphics.StateLineCap) && cap == b.State.Param.LineCap {
		return
	}
	b.State.Param.LineCap = cap
	b.State.MarkAsSet(graphics.StateLineCap)
	b.emit(content.OpSetLineCap, pdf.Integer(cap))
}

// SetLineJoin sets the line join style.
//
// This implements the PDF graphics operator "j".
func (b *Builder) SetLineJoin(join graphics.LineJoinStyle) {
	if join > 2 {
		b.Err = fmt.Errorf("SetLineJoin: invalid line join style %d", join)
		return
	}
	if b.isKnown(graphics.StateLineJoin) && join == b.State.Param.LineJoin {
		return
	}
	b.State.Param.LineJoin = join
	b.State.MarkAsSet(graphics.StateLineJoin)
	b.emit(content.OpSetLineJoin, pdf.Integer(join))
}

// SetMiterLimit sets the miter limit.
//
// This implements the PDF graphics operator "M".
func (b *Builder) SetMiterLimit(limit float64) {
	if limit < 1 {
		b.Err = fmt.Errorf("SetMiterLimit: invalid miter limit %f", limit)
		return
	}
	if b.isKnown(graphics.StateMiterLimit) && nearlyEqual(limit, b.State.Param.MiterLimit) {
		return
	}
	b.State.Param.MiterLimit = limit
	b.State.MarkAsSet(graphics.StateMiterLimit)
	b.emit(content.OpSetMiterLimit, pdf.Number(limit))
}

// SetLineDash sets the line dash pattern.
//
// This implements the PDF graphics operator "d".
func (b *Builder) SetLineDash(pattern []float64, phase float64) {
	if b.isKnown(graphics.StateLineDash) &&
		sliceNearlyEqual(pattern, b.State.Param.DashPattern) &&
		nearlyEqual(phase, b.State.Param.DashPhase) {
		return
	}

	b.State.Param.DashPattern = pattern
	b.State.Param.DashPhase = phase
	b.State.MarkAsSet(graphics.StateLineDash)

	arr := make(pdf.Array, len(pattern))
	for i, v := range pattern {
		arr[i] = pdf.Number(v)
	}
	b.emit(content.OpSetLineDash, arr, pdf.Number(phase))
}

// SetRenderingIntent sets the rendering intent.
//
// This implements the PDF graphics operator "ri".
func (b *Builder) SetRenderingIntent(intent graphics.RenderingIntent) {
	if b.isKnown(graphics.StateRenderingIntent) && intent == b.State.Param.RenderingIntent {
		return
	}
	b.State.Param.RenderingIntent = intent
	b.State.MarkAsSet(graphics.StateRenderingIntent)
	b.emit(content.OpSetRenderingIntent, pdf.Name(intent))
}

// SetFlatnessTolerance sets the flatness tolerance.
//
// This implements the PDF graphics operator "i".
func (b *Builder) SetFlatnessTolerance(flatness float64) {
	if flatness < 0 || flatness > 100 {
		b.Err = fmt.Errorf("SetFlatnessTolerance: invalid flatness tolerance %f", flatness)
		return
	}
	if b.isKnown(graphics.StateFlatnessTolerance) && nearlyEqual(flatness, b.State.Param.FlatnessTolerance) {
		return
	}
	b.State.Param.FlatnessTolerance = flatness
	b.State.MarkAsSet(graphics.StateFlatnessTolerance)
	b.emit(content.OpSetFlatnessTolerance, pdf.Number(flatness))
}

// SetLineWidth sets the line width.
//
// This implements the PDF graphics operator "w".
func (b *Builder) SetLineWidth(width float64) {
	if width < 0 {
		b.Err = fmt.Errorf("SetLineWidth: negative width %f", width)
		return
	}
	if b.isKnown(graphics.StateLineWidth) && nearlyEqual(width, b.State.Param.LineWidth) {
		return
	}
	b.State.Param.LineWidth = width
	b.State.MarkAsSet(graphics.StateLineWidth)
	b.emit(content.OpSetLineWidth, pdf.Number(width))
}

func nearlyEqual(a, b float64) bool {
	const ε = 1e-6
	return math.Abs(a-b) < ε
}

// SetExtGState sets selected graphics state parameters.
//
// This implements the PDF graphics operator "gs".
func (b *Builder) SetExtGState(gs *graphics.ExtGState) {
	if b.Err != nil {
		return
	}
	name := b.getExtGStateName(gs)

	// Apply the ExtGState to our State
	// Use a temporary graphics.State to leverage ExtGState.ApplyTo
	tmp := graphics.State{Parameters: &b.State.Param, Set: b.State.Known}
	gs.ApplyTo(&tmp)
	b.State.Known = tmp.Set

	b.emit(content.OpSetExtGState, name)
}

func sliceNearlyEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !nearlyEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}
