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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/font"
)

// State represents the graphics state of a PDF processor.
type State struct {
	*Parameters
	Set StateBits
}

// Parameters collects all graphical parameters of the PDF processor.
//
// See section 8.4 of PDF 32000-1:2008.
type Parameters struct {
	// CTM is the "current transformation matrix", which maps positions from
	// user coordinates to device coordinates.
	// (default: device-dependent)
	CTM Matrix

	ClippingPath interface{} // TODO(voss): implement this

	StrokeColor color.Color
	FillColor   color.Color

	// Text State parameters:
	TextCharacterSpacing float64 // character spacing (T_c)
	TextWordSpacing      float64 // word spacing (T_w)
	TextHorizonalScaling float64 // horizonal scaling (T_h, normal sapcing = 100)
	TextLeading          float64 // leading (T_l)
	TextFont             font.NewFont
	TextFontSize         float64
	TextRenderingMode    TextRenderingMode
	TextRise             float64
	TextKnockout         bool

	// See https://github.com/pdf-association/pdf-issues/issues/368
	TextMatrix     Matrix // reset at the start of each text object
	TextLineMatrix Matrix // reset at the start of each text object

	LineWidth   float64
	LineCap     LineCapStyle
	LineJoin    LineJoinStyle
	MiterLimit  float64
	DashPattern []float64
	DashPhase   float64

	RenderingIntent pdf.Name

	// StrokeAdjustment is a flag specifying whether to compensate for possible
	// rasterization effects when stroking a path with a line width that is
	// small relative to the pixel resolution of the output device.
	StrokeAdjustment bool

	BlendMode              pdf.Object
	SoftMask               pdf.Object
	StrokeAlpha            float64
	FillAlpha              float64
	AlphaSourceFlag        bool
	BlackPointCompensation pdf.Name

	// The following parameters are device-dependent:

	OverprintStroke bool
	OverprintFill   bool // for PDF<1.3 this must equal OverprintStroke
	OverprintMode   int  // for PDF<1.3 this must be 0

	BlackGeneration   pdf.Object
	UndercolorRemoval pdf.Object
	TransferFunction  pdf.Object
	Halftone          pdf.Object
	HalftoneOriginX   float64 //  https://github.com/pdf-association/pdf-issues/issues/260
	HalftoneOriginY   float64

	// FlatnessTolerance is a positive number specifying the precision with which
	// curves are be rendered on the output device.  Smaller numbers give
	// smoother curves, but also increase the amount of computation needed
	// (default: 1).
	FlatnessTolerance float64

	// SmoothnessTolerance is a number in the range 0 to 1 specifying the
	// precision of smooth shading (default: device-dependent).
	SmoothnessTolerance float64
}

// StateBits is a bit mask for the fields of the State struct.
type StateBits uint64

// Possible values for StateBits.
const (
	// CTM is always set, so it is not included in the bit mask.
	// ClippingPath is always set, so it is not included in the bit mask.

	StateStrokeColor StateBits = 1 << iota
	StateFillColor

	StateTextCharacterSpacing
	StateTextWordSpacing
	StateTextHorizontalSpacing
	StateTextLeading
	StateTextFont // includes size
	StateTextRenderingMode
	StateTextRise
	StateTextKnockout

	StateTextMatrix
	StateTextLineMatrix

	StateLineWidth
	StateLineCap
	StateLineJoin
	StateMiterLimit
	StateDash // pattern and phase

	StateRenderingIntent
	StateStrokeAdjustment
	StateBlendMode
	StateSoftMask
	StateStrokeAlpha
	StateFillAlpha
	StateAlphaSourceFlag
	StateBlackPointCompensation

	StateOverprint
	StateOverprintMode
	StateBlackGeneration
	StateUndercolorRemoval
	StateTransferFunction
	StateHalftone
	StateHalftoneOrigin
	StateFlatnessTolerance
	StateSmoothnessTolerance

	stateFirstUnused
	AllStateBits = stateFirstUnused - 1
)

const (
	// initializedStateBits lists the parameters which are initialized to
	// their default values in [NewState].
	initializedStateBits = StateStrokeColor | StateFillColor | StateTextCharacterSpacing |
		StateTextWordSpacing | StateTextHorizontalSpacing | StateTextLeading | StateTextRenderingMode | StateTextRise |
		StateTextKnockout | StateLineWidth | StateLineCap | StateLineJoin |
		StateMiterLimit | StateDash | StateRenderingIntent |
		StateStrokeAdjustment | StateBlendMode | StateSoftMask |
		StateStrokeAlpha | StateFillAlpha | StateAlphaSourceFlag |
		StateBlackPointCompensation | StateOverprint | StateOverprintMode |
		StateFlatnessTolerance

	// extStateBits lists the parameters which can be encoded in an ExtGState
	// resource.
	extStateBits = StateTextFont | StateTextKnockout | StateLineWidth |
		StateLineCap | StateLineJoin | StateMiterLimit | StateDash |
		StateRenderingIntent | StateStrokeAdjustment | StateBlendMode |
		StateSoftMask | StateStrokeAlpha | StateFillAlpha |
		StateAlphaSourceFlag | StateBlackPointCompensation | StateOverprint |
		StateOverprintMode | StateBlackGeneration | StateUndercolorRemoval |
		StateTransferFunction | StateHalftone | StateHalftoneOrigin |
		StateFlatnessTolerance | StateSmoothnessTolerance
)

// NewState returns a new graphics state with default values,
// and a bit mask indicating which fields are set to their default values.
func NewState() State {
	param := &Parameters{}

	param.CTM = IdentityMatrix
	param.StrokeColor = color.Gray(0)
	param.FillColor = color.Gray(0)

	param.TextCharacterSpacing = 0
	param.TextWordSpacing = 0
	param.TextHorizonalScaling = 1
	param.TextLeading = 0
	// no default for Font
	// no default for FontSize
	param.TextRenderingMode = 0
	param.TextRise = 0
	param.TextKnockout = true

	// Tm and Tlm are reset at the start of each text object

	param.LineWidth = 1
	param.LineCap = LineCapButt
	param.LineJoin = LineJoinMiter
	param.MiterLimit = 10
	param.DashPattern = []float64{}
	param.DashPhase = 0

	param.RenderingIntent = RenderingIntentRelativeColorimetric
	param.StrokeAdjustment = false
	param.BlendMode = pdf.Name("Normal")
	param.SoftMask = nil
	param.StrokeAlpha = 1
	param.FillAlpha = 1
	param.AlphaSourceFlag = false
	param.BlackPointCompensation = pdf.Name("Default")

	param.OverprintStroke = false
	param.OverprintFill = false
	param.OverprintMode = 0

	// param.BlackGeneration = nil   // defaul: device dependent
	// param.UndercolorRemoval = nil // defaul: device dependent
	// param.TransferFunction = nil  // defaul: device dependent
	// param.Halftone = nil          // defaul: device dependent
	// param.HalftoneOriginX = 0     // defaul: device dependent
	// param.HalftoneOriginY = 0     // defaul: device dependent

	param.FlatnessTolerance = 1
	// param.SmoothnessTolerance = 0 // defaul: device dependent

	isSet := initializedStateBits

	return State{param, isSet}
}

// Clone returns a shallow copy of the GraphicsState.
func (s *Parameters) Clone() *Parameters {
	res := *s
	return &res
}

// Matrix contains a PDF transformation matrix.
// The elements are stored in the same order as for the "cm" operator.
//
// If M = [a b c d e f] is a Matrix, then M corresponds to the following
// 3x3 matrix:
//
//	/ a b 0 \
//	| c d 0 |
//	\ e f 1 /
//
// A vector (x, y, 1) is transformed by M into
//
//	(x y 1) * M = (a*x+c*y+e, b*x+d*y+f, 1)
type Matrix [6]float64

// Translate moves the origin of the coordinate system.
//
// Drawing the unit square [0, 1] x [0, 1] after applying this transformation
// is equivalent to drawing the rectangle [dx, dx+1] x [dy, dy+1] in the
// original coordinate system.
func Translate(dx, dy float64) Matrix {
	return Matrix{1, 0, 0, 1, dx, dy}
}

// Scale scales the coordinate system.
//
// Drawing the unit square [0, 1] x [0, 1] after applying this transformation
// is equivalent to drawing the rectangle [0, xScale] x [0, yScale] in the
// original coordinate system.
func Scale(xScale, yScale float64) Matrix {
	return Matrix{xScale, 0, 0, yScale, 0, 0}
}

// TranslateAndScale moves and scales the coordinate system.
//
// Drawing the unit square [0, 1] x [0, 1] after applying this transformation
// is equivalent to drawing the rectangle [dx, dx+xScale] x [dy, dy+yScale] in
// the original coordinate system.
//
// This is equivalent to first applying Translate(dx, dy) and then
// Scale(xScale, yScale).
func TranslateAndScale(dx, dy, xScale, yScale float64) Matrix {
	return Matrix{xScale, 0, 0, yScale, dx, dy}
}

// Apply applies the transformation matrix to the given vector.
func (A Matrix) Apply(x, y float64) (float64, float64) {
	return A[0]*x + A[2]*y + A[4], A[1]*x + A[3]*y + A[5]
}

// Mul multiplies two transformation matrices and returns the result.
func (A Matrix) Mul(B Matrix) Matrix {
	// / A0 A1 0 \  / B0 B1 0 \   / A0*B0+A1*B2    A0*B1+A1*B3    0 \
	// | A2 A3 0 |  | B2 B3 0 | = | A2*B0+A3*B2    A2*B1+A3*B3    0 |
	// \ A4 A5 1 /  \ B4 B5 1 /   \ A4*B0+A5*B2+B4 A4*B1+A5*B3+B5 1 /
	return Matrix{
		A[0]*B[0] + A[1]*B[2],
		A[0]*B[1] + A[1]*B[3],
		A[2]*B[0] + A[3]*B[2],
		A[2]*B[1] + A[3]*B[3],
		A[4]*B[0] + A[5]*B[2] + B[4],
		A[4]*B[1] + A[5]*B[3] + B[5],
	}
}

// IdentityMatrix is the identity transformation.
var IdentityMatrix = Matrix{1, 0, 0, 1, 0, 0}

// TextRenderingMode is the rendering mode for text.
type TextRenderingMode uint8

// Possible values for TextRenderingMode.
// See section 9.3.6 of ISO 32000-2:2020.
const (
	TextRenderingModeFill TextRenderingMode = iota
	TextRenderingModeStroke
	TextRenderingModeFillStroke
	TextRenderingModeInvisible
	TextRenderingModeFillClip
	TextRenderingModeStrokeClip
	TextRenderingModeFillStrokeClip
	TextRenderingModeClip
)

// LineCapStyle is the style of the end of a line.
type LineCapStyle uint8

// Possible values for LineCapStyle.
// See section 8.4.3.3 of PDF 32000-1:2008.
const (
	LineCapButt   LineCapStyle = 0
	LineCapRound  LineCapStyle = 1
	LineCapSquare LineCapStyle = 2
)

// LineJoinStyle is the style of the corner of a line.
type LineJoinStyle uint8

// Possible values for LineJoinStyle.
const (
	LineJoinMiter LineJoinStyle = 0
	LineJoinRound LineJoinStyle = 1
	LineJoinBevel LineJoinStyle = 2
)

// The PDF standard rendering intents.
//
// See section 8.6.5.8 of ISO 32000-2:2020.
const (
	RenderingIntentAbsoluteColorimetric pdf.Name = "AbsoluteColorimetric"
	RenderingIntentRelativeColorimetric pdf.Name = "RelativeColorimetric"
	RenderingIntentSaturation           pdf.Name = "Saturation"
	RenderingIntentPerceptual           pdf.Name = "Perceptual"
)
