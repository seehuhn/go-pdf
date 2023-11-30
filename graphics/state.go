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
)

type State struct {
	*Parameters
	Set StateBits
}

func (s State) Clone() State {
	return State{
		Parameters: s.Parameters.Clone(),
		Set:        s.Set,
	}
}

// Parameters collects all graphical parameters of the PDF processor.
//
// See section 8.4 of PDF 32000-1:2008.
type Parameters struct {
	// CTM is the "current transformation matrix", which maps positions from
	// user coordinates to device coordinates.
	// (default: device-dependent)
	CTM Matrix

	// TODO(voss): clipping path

	StrokeColor color.Color
	FillColor   color.Color

	// Text State parameters:
	Tc           float64 // character spacing
	Tw           float64 // word spacing
	Th           float64 // horizonal scaling
	Tl           float64 // leading
	Font         Resource
	FontSize     float64
	Tmode        int
	TextRise     float64
	TextKnockout bool

	// TODO(voss): revisit the following two parameters once
	// https://github.com/pdf-association/pdf-issues/issues/368 is resolved.
	Tm  Matrix // reset at the start of each text object
	Tlm Matrix // reset at the start of each text object

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

	BlendMode              pdf.Name // TODO(voss): can be an array for PDF<2.0
	SoftMask               pdf.Dict // TODO(voss): can be name
	StrokeAlpha            float64
	FillAlpha              float64
	AlphaSourceFlag        bool
	BlackPointCompensation pdf.Name

	// The following parameters are device-dependent:

	OverprintStroke bool
	OverprintFill   bool // for PDF<1.3 this must equal OverprintStroke
	OverprintMode   int  // for PDF<1.3 this must be 0

	// TODO(voss): black generation
	// TODO(voss): undercolor removal
	// TODO(voss): transfer function
	// TODO(voss): halftone
	// TODO(voss): halftone origin https://github.com/pdf-association/pdf-issues/issues/260

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
	StateCTM StateBits = 1 << iota
	StateStrokeColor
	StateFillColor
	StateTm
	StateTlm
	StateTc
	StateTw
	StateTh
	StateTl
	StateFont // includes size
	StateTmode
	StateTextRise
	StateTextKnockout
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
	StateFlatnessTolerance
	StateSmoothnessTolerance

	stateFirstUnused
	AllStateBits = stateFirstUnused - 1
)

// NewState returns a new graphics state with default values,
// and a bit mask indicating which fields are set to their default values.
func NewState() State {
	param := &Parameters{}

	param.CTM = IdentityMatrix
	param.FillColor = color.Gray(0)
	param.StrokeColor = color.Gray(0)

	param.Tc = 0
	param.Tw = 0
	param.Th = 1
	param.Tl = 0
	// no default for Font
	// no default for FontSize
	param.Tmode = 0
	param.TextRise = 0
	param.TextKnockout = true

	param.LineWidth = 1
	param.LineCap = LineCapButt
	param.LineJoin = LineJoinMiter
	param.MiterLimit = 10
	param.RenderingIntent = pdf.Name("RelativeColorimetric")
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

	param.FlatnessTolerance = 1
	// res.SmoothnessTolerance has a device-dependent default

	// TODO(voss): should the list of exceptions include the CTM?
	isSet := AllStateBits & ^(StateFont | StateSmoothnessTolerance)

	return State{param, isSet}
}

// Update copies selected fields from another State into s.
func (s *State) Update(other State) {
	s.Set |= other.Set & AllStateBits
	if other.Set&StateCTM != 0 {
		s.Parameters.CTM = other.Parameters.CTM
	}
	if other.Set&StateStrokeColor != 0 {
		s.Parameters.StrokeColor = other.Parameters.StrokeColor
	}
	if other.Set&StateFillColor != 0 {
		s.Parameters.FillColor = other.Parameters.FillColor
	}
	if other.Set&StateTm != 0 {
		s.Parameters.Tm = other.Parameters.Tm
	}
	if other.Set&StateTlm != 0 {
		s.Parameters.Tlm = other.Parameters.Tlm
	}
	if other.Set&StateTc != 0 {
		s.Parameters.Tc = other.Parameters.Tc
	}
	if other.Set&StateTw != 0 {
		s.Parameters.Tw = other.Parameters.Tw
	}
	if other.Set&StateTh != 0 {
		s.Parameters.Th = other.Parameters.Th
	}
	if other.Set&StateTl != 0 {
		s.Parameters.Tl = other.Parameters.Tl
	}
	if other.Set&StateFont != 0 {
		s.Parameters.Font = other.Parameters.Font
		s.Parameters.FontSize = other.Parameters.FontSize
	}
	if other.Set&StateTmode != 0 {
		s.Parameters.Tmode = other.Parameters.Tmode
	}
	if other.Set&StateTextRise != 0 {
		s.Parameters.TextRise = other.Parameters.TextRise
	}
	if other.Set&StateTextKnockout != 0 {
		s.Parameters.TextKnockout = other.Parameters.TextKnockout
	}
	if other.Set&StateLineWidth != 0 {
		s.Parameters.LineWidth = other.Parameters.LineWidth
	}
	if other.Set&StateLineCap != 0 {
		s.Parameters.LineCap = other.Parameters.LineCap
	}
	if other.Set&StateLineJoin != 0 {
		s.Parameters.LineJoin = other.Parameters.LineJoin
	}
	if other.Set&StateMiterLimit != 0 {
		s.Parameters.MiterLimit = other.Parameters.MiterLimit
	}
	if other.Set&StateDash != 0 {
		s.Parameters.DashPattern = other.Parameters.DashPattern // TODO(voss): make a copy?
		s.Parameters.DashPhase = other.Parameters.DashPhase
	}
	if other.Set&StateRenderingIntent != 0 {
		s.Parameters.RenderingIntent = other.Parameters.RenderingIntent
	}
	if other.Set&StateStrokeAdjustment != 0 {
		s.Parameters.StrokeAdjustment = other.Parameters.StrokeAdjustment
	}
	if other.Set&StateBlendMode != 0 {
		s.Parameters.BlendMode = other.Parameters.BlendMode
	}
	if other.Set&StateSoftMask != 0 {
		s.Parameters.SoftMask = other.Parameters.SoftMask
	}
	if other.Set&StateStrokeAlpha != 0 {
		s.Parameters.StrokeAlpha = other.Parameters.StrokeAlpha
	}
	if other.Set&StateFillAlpha != 0 {
		s.Parameters.FillAlpha = other.Parameters.FillAlpha
	}
	if other.Set&StateAlphaSourceFlag != 0 {
		s.Parameters.AlphaSourceFlag = other.Parameters.AlphaSourceFlag
	}
	if other.Set&StateBlackPointCompensation != 0 {
		s.Parameters.BlackPointCompensation = other.Parameters.BlackPointCompensation
	}
	if other.Set&StateOverprint != 0 {
		s.Parameters.OverprintStroke = other.Parameters.OverprintStroke
		s.Parameters.OverprintFill = other.Parameters.OverprintFill
	}
	if other.Set&StateOverprintMode != 0 {
		s.Parameters.OverprintMode = other.Parameters.OverprintMode
	}
	if other.Set&StateFlatnessTolerance != 0 {
		s.Parameters.FlatnessTolerance = other.Parameters.FlatnessTolerance
	}
	if other.Set&StateSmoothnessTolerance != 0 {
		s.Parameters.SmoothnessTolerance = other.Parameters.SmoothnessTolerance
	}
}

// Clone returns a shallow copy of the GraphicsState.
func (s *Parameters) Clone() *Parameters {
	res := *s
	return &res
}

// Matrix contains a PDF transformation matrix.
// The elements are stored in the same order as for the "cm" operator.
//
// If M = [a b c d e f] is a matrix, then M corresponds to the following
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
