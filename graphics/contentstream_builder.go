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

package graphics

import (
	"fmt"
	"math"
	"strconv"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/color"
)

// ContentStreamBuilder provides methods to build a PDF content stream.
type ContentStreamBuilder struct {
	resources *pdf.Resources
	operators []Operator

	// Current graphics state for validation
	currentObject objectType
	State
	stack []State

	// Resource tracking
	resName map[catResBuilder]pdf.Name

	// Text buffer for glyph sequences
	glyphBuf *font.GlyphSeq

	// Nesting tracking
	nesting []pairType
	markedContent []*MarkedContent

	Err error
}

type catResBuilder struct {
	cat resourceCategory
	res any
}

// NewContentStreamBuilder creates a new builder for constructing content streams.
func NewContentStreamBuilder() *ContentStreamBuilder {
	return &ContentStreamBuilder{
		resources:     &pdf.Resources{},
		operators:     []Operator{},
		currentObject: objPage,
		State:         NewState(),
		resName:       make(map[catResBuilder]pdf.Name),
		glyphBuf:      &font.GlyphSeq{},
	}
}

// Build creates a ContentStream from the accumulated operators.
func (b *ContentStreamBuilder) Build() *ContentStream {
	return &ContentStream{
		Resources: b.resources,
		Operators: b.operators,
	}
}

// addOp adds an operator to the content stream.
func (b *ContentStreamBuilder) addOp(name pdf.Name, args ...pdf.Native) {
	if b.Err != nil {
		return
	}
	b.operators = append(b.operators, Operator{
		Name: name,
		Args: args,
	})
}

// isValid returns true if the current graphics object is one of the given types
// and if b.Err is nil. Otherwise it sets b.Err and returns false.
func (b *ContentStreamBuilder) isValid(cmd string, ss objectType) bool {
	if b.Err != nil {
		return false
	}

	if b.currentObject&ss != 0 {
		return true
	}

	b.Err = fmt.Errorf("unexpected state %q for %q", b.currentObject, cmd)
	return false
}

func (b *ContentStreamBuilder) coord(x float64) pdf.Real {
	// TODO: use the current transformation matrix to determine precision
	return pdf.Real(x)
}

func formatNum(x float64) string {
	return strconv.FormatFloat(x, 'f', -1, 64)
}

// Path construction operators

// MoveTo starts a new path at the given coordinates.
// This implements the PDF graphics operator "m".
func (b *ContentStreamBuilder) MoveTo(x, y float64) {
	if !b.isValid("MoveTo", objPage|objPath) {
		return
	}

	if b.currentObject == objPath && !b.ThisSubpathClosed {
		b.AllSubpathsClosed = false
	}

	b.currentObject = objPath
	b.StartX, b.StartY = x, y
	b.CurrentX, b.CurrentY = x, y
	b.ThisSubpathClosed = true

	b.addOp("m", pdf.Real(x), pdf.Real(y))
}

// LineTo appends a straight line segment to the current path.
// This implements the PDF graphics operator "l".
func (b *ContentStreamBuilder) LineTo(x, y float64) {
	if !b.isValid("LineTo", objPath) {
		return
	}

	b.CurrentX, b.CurrentY = x, y
	b.ThisSubpathClosed = false

	b.addOp("l", pdf.Real(x), pdf.Real(y))
}

// CurveTo appends a cubic Bezier curve to the current path.
// This implements the PDF graphics operators "c", "v", and "y".
func (b *ContentStreamBuilder) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	if !b.isValid("CurveTo", objPath) {
		return
	}

	x0, y0 := b.CurrentX, b.CurrentY
	b.CurrentX, b.CurrentY = x3, y3
	b.ThisSubpathClosed = false

	if nearlyEqual(x0, x1) && nearlyEqual(y0, y1) {
		b.addOp("v", pdf.Real(x2), pdf.Real(y2), pdf.Real(x3), pdf.Real(y3))
	} else if nearlyEqual(x2, x3) && nearlyEqual(y2, y3) {
		b.addOp("y", pdf.Real(x1), pdf.Real(y1), pdf.Real(x3), pdf.Real(y3))
	} else {
		b.addOp("c", pdf.Real(x1), pdf.Real(y1), pdf.Real(x2), pdf.Real(y2), pdf.Real(x3), pdf.Real(y3))
	}
}

// ClosePath closes the current subpath.
// This implements the PDF graphics operator "h".
func (b *ContentStreamBuilder) ClosePath() {
	if !b.isValid("ClosePath", objPath) {
		return
	}

	b.CurrentX, b.CurrentY = b.StartX, b.StartY
	b.ThisSubpathClosed = true

	b.addOp("h")
}

// Rectangle appends a rectangle to the current path as a closed subpath.
// This implements the PDF graphics operator "re".
func (b *ContentStreamBuilder) Rectangle(x, y, width, height float64) {
	if !b.isValid("Rectangle", objPage|objPath) {
		return
	}

	if b.currentObject == objPath && !b.ThisSubpathClosed {
		b.AllSubpathsClosed = false
	}

	b.currentObject = objPath
	b.StartX, b.StartY = x, y
	b.CurrentX, b.CurrentY = x, y
	b.ThisSubpathClosed = true

	b.addOp("re", pdf.Real(x), pdf.Real(y), pdf.Real(width), pdf.Real(height))
}

// Path painting operators

// Stroke strokes the current path.
// This implements the PDF graphics operator "S".
func (b *ContentStreamBuilder) Stroke() {
	if !b.isValid("Stroke", objPath|objClippingPath) {
		return
	}

	if !b.ThisSubpathClosed {
		b.AllSubpathsClosed = false
	}

	b.currentObject = objPage
	b.addOp("S")
}

// CloseAndStroke closes and strokes the current path.
// This implements the PDF graphics operator "s".
func (b *ContentStreamBuilder) CloseAndStroke() {
	if !b.isValid("CloseAndStroke", objPath|objClippingPath) {
		return
	}

	b.CurrentX, b.CurrentY = b.StartX, b.StartY
	b.ThisSubpathClosed = true
	b.currentObject = objPage

	b.addOp("s")
}

// Fill fills the current path using the nonzero winding number rule.
// This implements the PDF graphics operator "f".
func (b *ContentStreamBuilder) Fill() {
	if !b.isValid("Fill", objPath|objClippingPath) {
		return
	}

	b.currentObject = objPage
	b.addOp("f")
}

// FillEvenOdd fills the current path using the even-odd rule.
// This implements the PDF graphics operator "f*".
func (b *ContentStreamBuilder) FillEvenOdd() {
	if !b.isValid("FillEvenOdd", objPath|objClippingPath) {
		return
	}

	b.currentObject = objPage
	b.addOp("f*")
}

// FillAndStroke fills and then strokes the current path.
// This implements the PDF graphics operator "B".
func (b *ContentStreamBuilder) FillAndStroke() {
	if !b.isValid("FillAndStroke", objPath|objClippingPath) {
		return
	}

	if !b.ThisSubpathClosed {
		b.AllSubpathsClosed = false
	}

	b.currentObject = objPage
	b.addOp("B")
}

// FillAndStrokeEvenOdd fills and strokes using the even-odd rule.
// This implements the PDF graphics operator "B*".
func (b *ContentStreamBuilder) FillAndStrokeEvenOdd() {
	if !b.isValid("FillAndStrokeEvenOdd", objPath|objClippingPath) {
		return
	}

	if !b.ThisSubpathClosed {
		b.AllSubpathsClosed = false
	}

	b.currentObject = objPage
	b.addOp("B*")
}

// CloseFillAndStroke closes, fills, and strokes the current path.
// This implements the PDF graphics operator "b".
func (b *ContentStreamBuilder) CloseFillAndStroke() {
	if !b.isValid("CloseFillAndStroke", objPath|objClippingPath) {
		return
	}

	b.CurrentX, b.CurrentY = b.StartX, b.StartY
	b.ThisSubpathClosed = true
	b.currentObject = objPage

	b.addOp("b")
}

// CloseFillAndStrokeEvenOdd closes, fills, and strokes using the even-odd rule.
// This implements the PDF graphics operator "b*".
func (b *ContentStreamBuilder) CloseFillAndStrokeEvenOdd() {
	if !b.isValid("CloseFillAndStrokeEvenOdd", objPath|objClippingPath) {
		return
	}

	b.CurrentX, b.CurrentY = b.StartX, b.StartY
	b.ThisSubpathClosed = true
	b.currentObject = objPage

	b.addOp("b*")
}

// EndPath ends the path without filling or stroking.
// This implements the PDF graphics operator "n".
func (b *ContentStreamBuilder) EndPath() {
	if !b.isValid("EndPath", objPath|objClippingPath) {
		return
	}

	b.currentObject = objPage
	b.addOp("n")
}

// Clipping path operators

// ClipNonZero intersects the current clipping path with the current path using the nonzero winding rule.
// This implements the PDF graphics operator "W".
func (b *ContentStreamBuilder) ClipNonZero() {
	if !b.isValid("ClipNonZero", objPath) {
		return
	}

	b.currentObject = objClippingPath
	b.addOp("W")
}

// ClipEvenOdd intersects the current clipping path with the current path using the even-odd rule.
// This implements the PDF graphics operator "W*".
func (b *ContentStreamBuilder) ClipEvenOdd() {
	if !b.isValid("ClipEvenOdd", objPath) {
		return
	}

	b.currentObject = objClippingPath
	b.addOp("W*")
}

// Graphics state operators

// PushGraphicsState saves the current graphics state.
// This implements the PDF graphics operator "q".
func (b *ContentStreamBuilder) PushGraphicsState() {
	if !b.isValid("PushGraphicsState", objPage|objPath|objClippingPath) {
		return
	}

	b.stack = append(b.stack, b.State)
	b.nesting = append(b.nesting, pairTypeQ)

	b.addOp("q")
}

// PopGraphicsState restores the most recently saved graphics state.
// This implements the PDF graphics operator "Q".
func (b *ContentStreamBuilder) PopGraphicsState() {
	if !b.isValid("PopGraphicsState", objPage|objPath|objClippingPath) {
		return
	}

	if len(b.stack) == 0 {
		b.Err = fmt.Errorf("PopGraphicsState: no saved state")
		return
	}

	if len(b.nesting) == 0 || b.nesting[len(b.nesting)-1] != pairTypeQ {
		b.Err = fmt.Errorf("PopGraphicsState: mismatched nesting")
		return
	}

	b.State = b.stack[len(b.stack)-1]
	b.stack = b.stack[:len(b.stack)-1]
	b.nesting = b.nesting[:len(b.nesting)-1]

	b.addOp("Q")
}

// Transform applies a transformation to the current transformation matrix.
// This implements the PDF graphics operator "cm".
func (b *ContentStreamBuilder) Transform(m matrix.Matrix) {
	if !b.isValid("Transform", objPage|objPath) {
		return
	}

	b.CTM = m.Mul(b.CTM)

	b.addOp("cm",
		pdf.Real(m[0]), pdf.Real(m[1]),
		pdf.Real(m[2]), pdf.Real(m[3]),
		pdf.Real(m[4]), pdf.Real(m[5]))
}

// SetLineWidth sets the line width.
// This implements the PDF graphics operator "w".
func (b *ContentStreamBuilder) SetLineWidth(width float64) {
	if !b.isValid("SetLineWidth", objPage|objPath) {
		return
	}

	b.LineWidth = width
	b.Set |= StateLineWidth

	b.addOp("w", pdf.Real(width))
}

// SetLineCap sets the line cap style.
// This implements the PDF graphics operator "J".
func (b *ContentStreamBuilder) SetLineCap(style LineCapStyle) {
	if !b.isValid("SetLineCap", objPage|objPath) {
		return
	}

	b.LineCap = style
	b.Set |= StateLineCap

	b.addOp("J", pdf.Integer(style))
}

// SetLineJoin sets the line join style.
// This implements the PDF graphics operator "j".
func (b *ContentStreamBuilder) SetLineJoin(style LineJoinStyle) {
	if !b.isValid("SetLineJoin", objPage|objPath) {
		return
	}

	b.LineJoin = style
	b.Set |= StateLineJoin

	b.addOp("j", pdf.Integer(style))
}

// SetMiterLimit sets the miter limit.
// This implements the PDF graphics operator "M".
func (b *ContentStreamBuilder) SetMiterLimit(limit float64) {
	if !b.isValid("SetMiterLimit", objPage|objPath) {
		return
	}

	b.MiterLimit = limit
	b.Set |= StateMiterLimit

	b.addOp("M", pdf.Real(limit))
}

// SetLineDash sets the line dash pattern.
// This implements the PDF graphics operator "d".
func (b *ContentStreamBuilder) SetLineDash(pattern []float64, phase float64) {
	if !b.isValid("SetLineDash", objPage|objPath) {
		return
	}

	b.DashPattern = pattern
	b.DashPhase = phase
	b.Set |= StateLineDash

	arr := make(pdf.Array, len(pattern))
	for i, v := range pattern {
		arr[i] = pdf.Real(v)
	}

	b.addOp("d", arr, pdf.Real(phase))
}

// Helper functions

func nearlyEqualNum(a, b float64) bool {
	const epsilon = 1e-6
	return math.Abs(a-b) < epsilon
}

// Circle draws a circle at the given center and radius.
func (b *ContentStreamBuilder) Circle(x, y, radius float64) {
	const k = 0.5522847498307935 // (4/3) * tan(π/8)
	kr := k * radius

	b.MoveTo(x+radius, y)
	b.CurveTo(x+radius, y+kr, x+kr, y+radius, x, y+radius)
	b.CurveTo(x-kr, y+radius, x-radius, y+kr, x-radius, y)
	b.CurveTo(x-radius, y-kr, x-kr, y-radius, x, y-radius)
	b.CurveTo(x+kr, y-radius, x+radius, y-kr, x+radius, y)
}

// SetStrokeColor sets the stroke color.
func (b *ContentStreamBuilder) SetStrokeColor(c color.Color) {
	b.setColor(c, false)
}

// SetFillColor sets the fill color.
func (b *ContentStreamBuilder) SetFillColor(c color.Color) {
	b.setColor(c, true)
}

// setColor sets either the stroke or fill color.
func (b *ContentStreamBuilder) setColor(c color.Color, fill bool) {
	if !b.isValid("setColor", objPage|objPath|objText) {
		return
	}

	// TODO: Implement color space handling and resource management
	// For now, just handle simple device colors

	if fill {
		b.FillColor = c
		b.Set |= StateFillColor
	} else {
		b.StrokeColor = c
		b.Set |= StateStrokeColor
	}

	// TODO: Add appropriate color operators based on color type
}
