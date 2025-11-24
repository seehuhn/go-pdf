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
	"strings"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"
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

// builderGetResourceName returns a name which can be used to refer to a resource from
// within the content stream. The resource is added to the resource dictionary.
// Note: The actual embedding of the resource will happen when the content stream is written.
func builderGetResourceName(b *ContentStreamBuilder, cat resourceCategory, resource pdf.Embedder) (pdf.Name, error) {
	key := catResBuilder{cat, resource}
	v, ok := b.resName[key]
	if ok {
		return v, nil
	}

	dict := b.getCategoryDict(cat)
	name := b.generateName(cat, dict)
	// Store the Embedder directly. It will be embedded when the content stream is written.
	(*dict)[name] = resource.(pdf.Object)

	b.resName[key] = name
	return name, nil
}

func (b *ContentStreamBuilder) getCategoryDict(category resourceCategory) *pdf.Dict {
	var field *pdf.Dict
	switch category {
	case catFont:
		field = &b.resources.Font
	case catExtGState:
		field = &b.resources.ExtGState
	case catXObject:
		field = &b.resources.XObject
	case catColorSpace:
		field = &b.resources.ColorSpace
	case catPattern:
		field = &b.resources.Pattern
	case catShading:
		field = &b.resources.Shading
	case catProperties:
		field = &b.resources.Properties
	default:
		panic("invalid resource category")
	}

	if *field == nil {
		*field = pdf.Dict{}
	}

	return field
}

func (b *ContentStreamBuilder) generateName(category resourceCategory, dict *pdf.Dict) pdf.Name {
	var name pdf.Name

	prefix := getCategoryPrefix(category)
	numUsed := len(*dict)
	for k := numUsed + 1; ; k-- {
		name = prefix + pdf.Name(strconv.Itoa(k))
		if _, isUsed := (*dict)[name]; !isUsed {
			break
		}
	}

	return name
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

// SetRenderingIntent sets the rendering intent.
// This implements the PDF graphics operator "ri".
func (b *ContentStreamBuilder) SetRenderingIntent(intent RenderingIntent) {
	if !b.isValid("SetRenderingIntent", objPage|objPath) {
		return
	}

	b.RenderingIntent = intent
	b.Set |= StateRenderingIntent

	b.addOp("ri", pdf.Name(intent))
}

// SetFlatnessTolerance sets the flatness tolerance.
// This implements the PDF graphics operator "i".
func (b *ContentStreamBuilder) SetFlatnessTolerance(flatness float64) {
	if !b.isValid("SetFlatness", objPage|objPath) {
		return
	}

	b.FlatnessTolerance = flatness
	b.Set |= StateFlatnessTolerance

	b.addOp("i", pdf.Real(flatness))
}

// SetExtGState sets selected graphics state parameters.
// This implements the "gs" graphics operator.
func (b *ContentStreamBuilder) SetExtGState(s *ExtGState) {
	if !b.isValid("SetExtGState", objPage|objPath) {
		return
	}

	name, err := builderGetResourceName(b, catExtGState, s)
	if err != nil {
		b.Err = err
		return
	}

	s.ApplyTo(&b.State)

	b.addOp("gs", name)
}

// Helper functions

func nearlyEqualNum(a, b float64) bool {
	const epsilon = 1e-6
	return math.Abs(a-b) < epsilon
}

// Circle draws a circle at the given center and radius.
func (b *ContentStreamBuilder) Circle(x, y, radius float64) {
	if !b.isValid("Circle", objPage|objPath) {
		return
	}

	b.arc(x, y, radius, 0, 2*math.Pi, true)
	b.ClosePath()
}

// MoveToArc appends a circular arc to the current path,
// starting a new subpath.
//
// This is a convenience function, which uses MoveTo and CurveTo to draw the arc.
func (b *ContentStreamBuilder) MoveToArc(x, y, radius, startAngle, endAngle float64) {
	if !b.isValid("MoveToArc", objPage|objPath) {
		return
	}

	b.arc(x, y, radius, startAngle, endAngle, true)
}

// LineToArc appends a circular arc to the current subpath,
// connecting the previous point to the arc using a straight line.
//
// This is a convenience function, which uses LineTo and CurveTo to draw the arc.
func (b *ContentStreamBuilder) LineToArc(x, y, radius, startAngle, endAngle float64) {
	if !b.isValid("LineToArc", objPath) {
		return
	}

	b.arc(x, y, radius, startAngle, endAngle, false)
}

// arc appends a circular arc to the current path.
func (b *ContentStreamBuilder) arc(x, y, radius, startAngle, endAngle float64, move bool) {
	b.currentObject = objPath

	// rounding precision based on radius
	digits := max(1, 2-int(math.Round(math.Log10(radius))))

	// also see https://www.tinaja.com/glib/bezcirc2.pdf
	// from https://pomax.github.io/bezierinfo/ , section 42

	nSegment := int(math.Ceil(math.Abs(endAngle-startAngle) / (0.5 * math.Pi)))
	dPhi := (endAngle - startAngle) / float64(nSegment)
	k := 4.0 / 3.0 * radius * math.Tan(dPhi/4)

	phi := startAngle
	x0 := x + radius*math.Cos(phi)
	y0 := y + radius*math.Sin(phi)
	if move {
		b.MoveTo(pdf.Round(x0, digits), pdf.Round(y0, digits))
	} else {
		b.LineTo(pdf.Round(x0, digits), pdf.Round(y0, digits))
	}

	for range nSegment {
		x1 := x0 - k*math.Sin(phi)
		y1 := y0 + k*math.Cos(phi)
		phi += dPhi
		x3 := x + radius*math.Cos(phi)
		y3 := y + radius*math.Sin(phi)
		x2 := x3 + k*math.Sin(phi)
		y2 := y3 - k*math.Cos(phi)
		b.CurveTo(pdf.Round(x1, digits), pdf.Round(y1, digits), pdf.Round(x2, digits), pdf.Round(y2, digits), pdf.Round(x3, digits), pdf.Round(y3, digits))
		x0 = x3
		y0 = y3
	}
}

// SetStrokeColor sets the stroke color.
func (b *ContentStreamBuilder) SetStrokeColor(c color.Color) {
	if !b.isValid("SetStrokeColor", objPage|objPath|objText) {
		return
	}
	b.setColor(c, false)
}

// SetFillColor sets the fill color.
func (b *ContentStreamBuilder) SetFillColor(c color.Color) {
	if !b.isValid("SetFillColor", objPage|objPath|objText) {
		return
	}
	b.setColor(c, true)
}

// setColor sets either the stroke or fill color.
func (b *ContentStreamBuilder) setColor(c color.Color, fill bool) {
	var cur color.Color
	if fill {
		if b.isSet(StateFillColor) {
			cur = b.FillColor
		}
		b.FillColor = c
		b.Set |= StateFillColor
	} else {
		if b.isSet(StateStrokeColor) {
			cur = b.StrokeColor
		}
		b.StrokeColor = c
		b.Set |= StateStrokeColor
	}

	cs := c.ColorSpace()
	var needsColorSpace bool
	switch cs.Family() {
	case color.FamilyDeviceGray, color.FamilyDeviceRGB, color.FamilyDeviceCMYK:
		needsColorSpace = false
	default:
		needsColorSpace = cur == nil || cur.ColorSpace() != cs
	}

	if needsColorSpace {
		name, err := builderGetResourceName(b, catColorSpace, cs)
		if err != nil {
			b.Err = err
			return
		}

		var op pdf.Name = "CS"
		if fill {
			op = "cs"
		}
		b.addOp(op, name)
		if b.Err != nil {
			return
		}
		cur = cs.Default()
	}

	if cur != c {
		values, pattern, opStr := color.Operator(c)
		args := make([]pdf.Native, 0, len(values)+2)
		for _, val := range values {
			args = append(args, pdf.Real(val))
		}
		if pattern != nil {
			name, err := builderGetResourceName(b, catPattern, pattern)
			if err != nil {
				b.Err = err
				return
			}
			args = append(args, name)
		}
		var op pdf.Name
		if fill {
			op = pdf.Name(strings.ToLower(string(opStr)))
		} else {
			op = pdf.Name(opStr)
		}
		b.addOp(op, args...)
	}
}

// DrawShading paints the given shading, subject to the current clipping path.
// The current colour in the graphics state is neither used nor altered.
//
// All coordinates in the shading dictionary are interpreted relative to the
// current user space. The "Background" entry in the shading pattern (if any)
// is ignored.
//
// This implements the PDF graphics operator "sh".
func (b *ContentStreamBuilder) DrawShading(shading Shading) {
	if !b.isValid("DrawShading", objPage) {
		return
	}

	name, err := builderGetResourceName(b, catShading, shading)
	if err != nil {
		b.Err = err
		return
	}

	b.addOp("sh", name)
}

// Text operators

// TextBegin starts a new text object.
// This must be paired with TextEnd.
//
// This implements the PDF graphics operator "BT".
func (b *ContentStreamBuilder) TextBegin() {
	if !b.isValid("TextBegin", objPage) {
		return
	}
	b.currentObject = objText

	b.nesting = append(b.nesting, pairTypeBT)

	b.State.TextMatrix = matrix.Identity
	b.State.TextLineMatrix = matrix.Identity
	b.Set |= StateTextMatrix

	b.addOp("BT")
}

// TextEnd ends the current text object.
// This must be paired with TextBegin.
//
// This implements the PDF graphics operator "ET".
func (b *ContentStreamBuilder) TextEnd() {
	if !b.isValid("TextEnd", objText) {
		return
	}
	b.currentObject = objPage

	if len(b.nesting) == 0 || b.nesting[len(b.nesting)-1] != pairTypeBT {
		b.Err = fmt.Errorf("TextEnd: no matching TextBegin")
		return
	}
	b.nesting = b.nesting[:len(b.nesting)-1]

	b.Set &= ^StateTextMatrix

	b.addOp("ET")
}

// TextSetCharacterSpacing sets additional character spacing.
//
// This implements the PDF graphics operator "Tc".
func (b *ContentStreamBuilder) TextSetCharacterSpacing(charSpacing float64) {
	if !b.isValid("TextSetCharSpacing", objText|objPage) {
		return
	}

	b.State.TextCharacterSpacing = charSpacing
	b.Set |= StateTextCharacterSpacing

	b.addOp("Tc", pdf.Real(charSpacing))
}

// TextSetWordSpacing sets additional word spacing.
//
// This implements the PDF graphics operator "Tw".
func (b *ContentStreamBuilder) TextSetWordSpacing(wordSpacing float64) {
	if !b.isValid("TextSetWordSpacing", objText|objPage) {
		return
	}

	b.State.TextWordSpacing = wordSpacing
	b.Set |= StateTextWordSpacing

	b.addOp("Tw", pdf.Real(wordSpacing))
}

// TextSetHorizontalScaling sets the horizontal scaling.
// The effect of this is to strech/compress the text horizontally.
// The value 1 corresponds to normal scaling.
// Negative values correspond to horizontally mirrored text.
//
// This implements the PDF graphics operator "Tz".
func (b *ContentStreamBuilder) TextSetHorizontalScaling(scaling float64) {
	if !b.isValid("TextSetHorizontalScaling", objText|objPage) {
		return
	}

	b.State.TextHorizontalScaling = scaling
	b.Set |= StateTextHorizontalScaling

	b.addOp("Tz", pdf.Real(scaling*100))
}

// TextSetLeading sets the leading.
// The leading is the distance between the baselines of two consecutive lines of text.
// Positive values indicate that the next line of text is below the current line.
//
// This implements the PDF graphics operator "TL".
func (b *ContentStreamBuilder) TextSetLeading(leading float64) {
	if !b.isValid("TextSetLeading", objText|objPage) {
		return
	}

	b.State.TextLeading = leading
	b.Set |= StateTextLeading

	b.addOp("TL", pdf.Real(leading))
}

// TextSetFont sets the font and font size.
//
// This implements the PDF graphics operator "Tf".
func (b *ContentStreamBuilder) TextSetFont(F font.Instance, size float64) {
	if !b.isValid("TextSetFont", objText|objPage) {
		return
	}

	name, err := builderGetResourceName(b, catFont, F)
	if err != nil {
		b.Err = err
		return
	}

	b.State.TextFont = F
	b.State.TextFontSize = size
	b.State.Set |= StateTextFont

	b.addOp("Tf", name, pdf.Real(size))
}

// TextSetRenderingMode sets the text rendering mode.
//
// This implements the PDF graphics operator "Tr".
func (b *ContentStreamBuilder) TextSetRenderingMode(mode TextRenderingMode) {
	if !b.isValid("TextSetRenderingMode", objText|objPage) {
		return
	}

	b.State.TextRenderingMode = mode
	b.Set |= StateTextRenderingMode

	b.addOp("Tr", pdf.Integer(mode))
}

// TextSetRise sets the text rise.
// Positive values move the text up.
//
// This implements the PDF graphics operator "Ts".
func (b *ContentStreamBuilder) TextSetRise(rise float64) {
	if !b.isValid("TextSetRise", objText|objPage) {
		return
	}

	b.State.TextRise = rise
	b.Set |= StateTextRise

	b.addOp("Ts", pdf.Real(rise))
}

// TextFirstLine moves to the start of the next line of text.
// The new text position is (x, y), relative to the start of the current line
// (or to the current point if there is no current line).
//
// This implements the PDF graphics operator "Td".
func (b *ContentStreamBuilder) TextFirstLine(x, y float64) {
	if !b.isValid("TextFirstLine", objText) {
		return
	}

	b.TextLineMatrix = matrix.Translate(x, y).Mul(b.TextLineMatrix)
	b.TextMatrix = b.TextLineMatrix

	b.addOp("Td", pdf.Real(x), pdf.Real(y))
}

// TextSecondLine moves to the point (dx, dy) relative to the start of the
// current line of text.   The function also sets the leading to -dy.
// Usually, dy is negative.
//
// This implements the PDF graphics operator "TD".
func (b *ContentStreamBuilder) TextSecondLine(dx, dy float64) {
	if !b.isValid("TextSecondLine", objText) {
		return
	}

	b.TextLineMatrix = matrix.Translate(dx, dy).Mul(b.TextLineMatrix)
	b.TextMatrix = b.TextLineMatrix
	b.TextLeading = -dy
	b.Set |= StateTextLeading

	b.addOp("TD", pdf.Real(dx), pdf.Real(dy))
}

// TextSetMatrix replaces the current text matrix and line matrix with M.
//
// This implements the PDF graphics operator "Tm".
func (b *ContentStreamBuilder) TextSetMatrix(M matrix.Matrix) {
	if !b.isValid("TextSetMatrix", objText) {
		return
	}

	b.TextMatrix = M
	b.TextLineMatrix = M
	b.Set |= StateTextMatrix

	b.addOp("Tm",
		pdf.Real(M[0]), pdf.Real(M[1]),
		pdf.Real(M[2]), pdf.Real(M[3]),
		pdf.Real(M[4]), pdf.Real(M[5]))
}

// TextNextLine moves to the start of the next line of text.
//
// This implements the PDF graphics operator "T*".
func (b *ContentStreamBuilder) TextNextLine() {
	if !b.isValid("TextNewLine", objText) {
		return
	}
	if err := b.mustBeSet(StateTextMatrix | StateTextLeading); err != nil {
		b.Err = err
		return
	}

	b.TextLineMatrix = matrix.Translate(0, -b.TextLeading).Mul(b.TextLineMatrix)
	b.TextMatrix = b.TextLineMatrix

	b.addOp("T*")
}

// TextShowRaw shows an already encoded text in the PDF file.
//
// This implements the PDF graphics operator "Tj".
func (b *ContentStreamBuilder) TextShowRaw(s pdf.String) {
	if !b.isValid("TextShowRaw", objText) {
		return
	}
	if err := b.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise | StateTextWordSpacing | StateTextCharacterSpacing); err != nil {
		b.Err = err
		return
	}

	b.updateTextPosition(s)

	b.addOp("Tj", s)
}

// TextShowNextLineRaw start a new line and then shows an already encoded text
// in the PDF file.  This has the same effect as TextNextLine followed
// by TextShowRaw.
//
// This implements the PDF graphics operator "'".
func (b *ContentStreamBuilder) TextShowNextLineRaw(s pdf.String) {
	if !b.isValid("TextShowNextLineRaw", objText) {
		return
	}
	if err := b.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise | StateTextWordSpacing | StateTextCharacterSpacing | StateTextLeading); err != nil {
		b.Err = err
		return
	}

	b.TextLineMatrix = matrix.Translate(0, -b.TextLeading).Mul(b.TextLineMatrix)
	b.TextMatrix = b.TextLineMatrix

	b.updateTextPosition(s)

	b.addOp("'", s)
}

// TextShowSpacedRaw adjusts word and character spacing and then shows an
// already encoded text in the PDF file.  This has the same effect as
// TextSetWordSpacing and TextSetCharacterSpacing, followed
// by TextShowRaw.
//
// This implements the PDF graphics operator '"'.
func (b *ContentStreamBuilder) TextShowSpacedRaw(wordSpacing, charSpacing float64, s pdf.String) {
	if !b.isValid("TextShowSpacedRaw", objText) {
		return
	}
	if err := b.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise); err != nil {
		b.Err = err
		return
	}

	b.State.TextWordSpacing = wordSpacing
	b.State.TextCharacterSpacing = charSpacing
	b.Set |= StateTextWordSpacing | StateTextCharacterSpacing
	b.updateTextPosition(s)

	b.addOp(`"`, pdf.Real(wordSpacing), pdf.Real(charSpacing), s)
}

// TextShowKernedRaw shows an already encoded text in the PDF file, using
// kerning information provided to adjust glyph spacing.
//
// The arguments must be of type pdf.String, pdf.Real, pdf.Integer or
// pdf.Number.
//
// This implements the PDF graphics operator "TJ".
func (b *ContentStreamBuilder) TextShowKernedRaw(args ...pdf.Object) {
	if !b.isValid("TextShowKernedRaw", objText) {
		return
	}
	if err := b.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise | StateTextWordSpacing | StateTextCharacterSpacing); err != nil {
		b.Err = err
		return
	}

	var a pdf.Array
	wMode := b.State.TextFont.WritingMode()
	for _, arg := range args {
		var delta float64
		switch arg := arg.(type) {
		case pdf.String:
			b.updateTextPosition(arg)
			if b.Err != nil {
				return
			}
		case pdf.Real:
			delta = float64(arg)
		case pdf.Integer:
			delta = float64(arg)
		default:
			b.Err = fmt.Errorf("TextShowKernedRaw: invalid argument type %T", arg)
			return
		}
		if delta != 0 {
			delta *= -b.State.TextFontSize / 1000
			if wMode == 0 {
				b.TextMatrix = matrix.Translate(delta*b.State.TextHorizontalScaling, 0).Mul(b.TextMatrix)
			} else {
				b.TextMatrix = matrix.Translate(0, delta).Mul(b.TextMatrix)
			}
		}
		a = append(a, arg)
	}

	b.addOp("TJ", a)
}

func (b *ContentStreamBuilder) updateTextPosition(s pdf.String) {
	wmode := b.TextFont.WritingMode()
	for info := range b.TextFont.Codes(s) {
		width := info.Width*b.TextFontSize + b.TextCharacterSpacing
		if info.UseWordSpacing {
			width += b.TextWordSpacing
		}
		if wmode == font.Horizontal {
			width *= b.TextHorizontalScaling
		}

		switch wmode {
		case font.Horizontal:
			b.TextMatrix = matrix.Translate(width, 0).Mul(b.TextMatrix)
		case font.Vertical:
			b.TextMatrix = matrix.Translate(0, width).Mul(b.TextMatrix)
		}
	}
}

// XObject operator

// DrawXObject draws a PDF XObject on the page.
//
// This implements the PDF graphics operator "Do".
func (b *ContentStreamBuilder) DrawXObject(obj XObject) {
	if !b.isValid("DrawXObject", objPage) {
		return
	}

	name, err := builderGetResourceName(b, catXObject, obj)
	if err != nil {
		b.Err = err
		return
	}

	b.addOp("Do", name)
}

// Marked content operators

// MarkedContentPoint adds a marked-content point to the content stream.
//
// This implements the PDF graphics operators "MP" (without properties)
// and "DP" (with properties).
func (b *ContentStreamBuilder) MarkedContentPoint(mc *MarkedContent) {
	if !b.isValid("MarkedContentPoint", objPage|objText) {
		return
	}

	if mc.Properties == nil {
		b.addOp("MP", mc.Tag)
		return
	}

	if mc.Inline {
		if !mc.Properties.IsDirect() {
			b.Err = ErrNotDirect
			return
		}
		// For inline properties, create a Dict from the property list
		// This requires converting the property list to a dict
		dict := pdf.Dict{}
		for _, key := range mc.Properties.Keys() {
			val, err := mc.Properties.Get(key)
			if err == nil {
				dict[key] = val
			}
		}
		b.addOp("DP", mc.Tag, dict)
	} else {
		name, err := builderGetResourceName(b, catProperties, mc.Properties)
		if err != nil {
			b.Err = err
			return
		}
		b.addOp("DP", mc.Tag, name)
	}
}

// MarkedContentStart begins a marked-content sequence.  The sequence is
// terminated by a call to MarkedContentEnd.
//
// This implements the PDF graphics operators "BMC" and "BDC".
func (b *ContentStreamBuilder) MarkedContentStart(mc *MarkedContent) {
	if !b.isValid("MarkedContentStart", objPage|objText) {
		return
	}

	b.nesting = append(b.nesting, pairTypeBMC)
	b.markedContent = append(b.markedContent, mc)

	if mc.Properties == nil {
		b.addOp("BMC", mc.Tag)
		return
	}

	if mc.Inline {
		if !mc.Properties.IsDirect() {
			b.Err = ErrNotDirect
			return
		}
		// For inline properties, create a Dict from the property list
		dict := pdf.Dict{}
		for _, key := range mc.Properties.Keys() {
			val, err := mc.Properties.Get(key)
			if err == nil {
				dict[key] = val
			}
		}
		b.addOp("BDC", mc.Tag, dict)
	} else {
		name, err := builderGetResourceName(b, catProperties, mc.Properties)
		if err != nil {
			b.Err = err
			return
		}
		b.addOp("BDC", mc.Tag, name)
	}
}

// MarkedContentEnd ends a marked-content sequence.
// This must be matched with a preceding call to MarkedContentStart.
func (b *ContentStreamBuilder) MarkedContentEnd() {
	if len(b.nesting) == 0 || b.nesting[len(b.nesting)-1] != pairTypeBMC {
		b.Err = fmt.Errorf("MarkedContentEnd: no matching MarkedContentStart")
		return
	}

	b.nesting = b.nesting[:len(b.nesting)-1]
	b.markedContent = b.markedContent[:len(b.markedContent)-1]

	b.addOp("EMC")
}

// Text layout convenience methods

// TextShow draws a string.
func (b *ContentStreamBuilder) TextShow(s string) float64 {
	if !b.isValid("TextShow", objText) {
		return 0
	}

	b.glyphBuf.Reset()
	gg := b.TextLayout(b.glyphBuf, s)
	if gg == nil {
		b.Err = fmt.Errorf("font does not support layouting")
		return 0
	}

	return b.TextShowGlyphs(gg)
}

// TextShowAligned draws a string and aligns it.
// The string is aligned in a space of the given width.
// q=0 means left alignment, q=1 means right alignment
// and q=0.5 means centering.
func (b *ContentStreamBuilder) TextShowAligned(s string, width, q float64) {
	if !b.isValid("TextShowAligned", objText) {
		return
	}
	gg := b.TextLayout(nil, s)
	if gg == nil {
		b.Err = fmt.Errorf("font does not support layouting")
		return
	}
	gg.Align(width, q)
	b.TextShowGlyphs(gg)
}

// TextShowGlyphs shows the PDF string s, taking kerning and text rise into
// account.
//
// This uses the "TJ", "Tj" and "Ts" PDF graphics operators.
func (b *ContentStreamBuilder) TextShowGlyphs(seq *font.GlyphSeq) float64 {
	if !b.isValid("TextShowGlyphs", objText) {
		return 0
	}
	if err := b.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise); err != nil {
		b.Err = err
		return 0
	}

	E := b.TextFont
	layouter, ok := E.(font.Layouter)
	if !ok {
		panic("font does not implement Layouter")
	}

	left := seq.Skip
	gg := seq.Seq

	var run pdf.String
	var out pdf.Array
	flush := func() {
		if len(run) > 0 {
			out = append(out, run)
			run = nil
		}
		if len(out) == 0 {
			return
		}
		if b.Err != nil {
			return
		}

		if len(out) == 1 {
			if s, ok := out[0].(pdf.String); ok {
				b.addOp("Tj", s)
				out = out[:0]
				return
			}
		}

		b.addOp("TJ", out)
		out = out[:0]
	}

	xActual := 0.0
	xWanted := left
	param := b.State
	if E.WritingMode() != 0 {
		panic("vertical writing mode not implemented")
	}
	codec := layouter.Codec()
	for _, g := range gg {
		if b.State.Set&StateTextRise == 0 || math.Abs(g.Rise-b.State.TextRise) > 1e-6 {
			flush()
			b.State.TextRise = g.Rise
			if b.Err != nil {
				return 0
			}
			b.addOp("Ts", pdf.Real(b.State.TextRise))
		}

		xOffsetInt := pdf.Integer(math.Round((xWanted - xActual) / param.TextFontSize / param.TextHorizontalScaling * 1000))
		if xOffsetInt != 0 && !layouter.IsBlank(g.GID) {
			if len(run) > 0 {
				out = append(out, run)
				run = nil
			}
			out = append(out, -xOffsetInt)
			xActual += float64(xOffsetInt) / 1000 * param.TextFontSize * param.TextHorizontalScaling
		}

		xWanted += g.Advance

		prevLen := len(run)
		charCode, ok := layouter.Encode(g.GID, g.Text)
		if !ok {
			continue // Skip glyphs that can't be encoded
		}
		run = codec.AppendCode(run, charCode)
		for info := range layouter.Codes(run[prevLen:]) {
			glyphWidth := info.Width*param.TextFontSize + param.TextCharacterSpacing
			if info.UseWordSpacing {
				glyphWidth += param.TextWordSpacing
			}
			xActual += glyphWidth * param.TextHorizontalScaling
		}
	}
	xOffsetInt := pdf.Integer(math.Round((xWanted - xActual) / param.TextFontSize / param.TextHorizontalScaling * 1000))
	if xOffsetInt != 0 {
		if len(run) > 0 {
			out = append(out, run)
			run = nil
		}
		out = append(out, -xOffsetInt)
		xActual += float64(xOffsetInt) / 1000 * param.TextFontSize * param.TextHorizontalScaling
	}
	flush()
	b.TextMatrix = matrix.Translate(xActual, 0).Mul(b.TextMatrix)

	return xActual
}

// TextLayout appends a string to a GlyphSeq, using the text parameters from
// the builder's graphics state.  If seq is nil, a new GlyphSeq is allocated.  The
// resulting GlyphSeq is returned.
//
// If no font is set, or if the current font does not implement
// font.Layouter, the function returns nil.  If seq is not nil (and there is
// no error), the return value is guaranteed to be equal to seq.
func (b *ContentStreamBuilder) TextLayout(seq *font.GlyphSeq, text string) *font.GlyphSeq {
	layouter, ok := b.TextFont.(font.Layouter)

	if b.Err != nil || !ok {
		return seq
	}

	T := font.NewTypesetter(layouter, b.TextFontSize)
	T.SetCharacterSpacing(b.TextCharacterSpacing)
	T.SetWordSpacing(b.TextWordSpacing)
	T.SetHorizontalScaling(b.TextHorizontalScaling)
	T.SetTextRise(b.TextRise)

	return T.Layout(seq, text)
}

// TextGetQuadPoints returns QuadPoints for a glyph sequence in default user
// space coordinates. Returns 4 Vec2 points representing one quadrilateral,
// where the first two points form the bottom edge of the (possibly rotated) bounding box.
func (b *ContentStreamBuilder) TextGetQuadPoints(seq *font.GlyphSeq, padding float64) []vec.Vec2 {
	// TODO(voss): Make sure this is correct for vertical writing mode.

	if seq == nil || len(seq.Seq) == 0 {
		return nil
	}
	if err := b.mustBeSet(StateTextFont | StateTextMatrix); err != nil {
		return nil
	}

	// get bounding rectangle in PDF text space units
	f, ok := b.TextFont.(font.Layouter)
	if !ok {
		return nil
	}
	geom := f.GetGeometry()
	size := b.TextFontSize

	height := geom.Ascent * size
	depth := -geom.Descent * size
	var leftBearing, rightBearing float64

	first := true
	currentPos := seq.Skip
	for _, glyph := range seq.Seq {
		bbox := &geom.GlyphExtents[glyph.GID]
		if !bbox.IsZero() {
			glyphDepth := -(bbox.LLy*size/1000 + glyph.Rise)
			glyphHeight := (bbox.URy*size/1000 + glyph.Rise)
			glyphLeft := currentPos + bbox.LLx*size/1000
			glyphRight := currentPos + bbox.URx*size/1000

			if glyphDepth > depth {
				depth = glyphDepth
			}
			if glyphHeight > height {
				height = glyphHeight
			}
			if glyphLeft < leftBearing || first {
				leftBearing = glyphLeft
			}
			if glyphRight > rightBearing || first {
				rightBearing = glyphRight
			}

			first = false
		}
		currentPos += glyph.Advance
	}
	if first {
		return nil
	}

	leftBearing -= padding
	rightBearing += padding
	height += padding
	depth += padding

	rectText := []float64{
		leftBearing, -depth, // bottom-left
		rightBearing, -depth, // bottom-right
		rightBearing, height, // top-right
		leftBearing, height, // top-left
	}

	// transform the bounding rectangle from text space to default user space
	M := b.TextMatrix.Mul(b.CTM)
	rectUser := make([]vec.Vec2, 4)
	for i := range 4 {
		x, y := M.Apply(rectText[2*i], rectText[2*i+1])
		rectUser[i] = vec.Vec2{X: x, Y: y}
	}

	return rectUser
}
