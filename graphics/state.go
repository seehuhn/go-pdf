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
	"slices"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/blend"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/halftone"
	"seehuhn.de/go/pdf/graphics/state"
	"seehuhn.de/go/pdf/graphics/transfer"
)

// Parameters collects all graphical parameters of the PDF processor.
//
// See section 8.4 of PDF 32000-1:2008.
type Parameters struct {
	// CTM is the "current transformation matrix", which maps positions from
	// user coordinates to device coordinates.
	// (default: device-dependent)
	CTM matrix.Matrix

	StartX, StartY     float64 // the starting point of the current path
	CurrentX, CurrentY float64 // the "current point"
	AllSubpathsClosed  bool    // all subpaths of the current path are closed
	ThisSubpathClosed  bool    // the current subpath is closed

	StrokeColor color.Color
	FillColor   color.Color

	// Text State parameters:
	TextCharacterSpacing  float64 // character spacing (T_c)
	TextWordSpacing       float64 // word spacing (T_w)
	TextHorizontalScaling float64 // horizonal scaling (T_h, normal spacing = 1)
	TextLeading           float64 // leading (T_l)
	TextFont              font.Instance
	TextFontSize          float64
	TextRenderingMode     TextRenderingMode
	TextRise              float64
	TextKnockout          bool

	// See https://github.com/pdf-association/pdf-issues/issues/368
	TextMatrix     matrix.Matrix // reset at the start of each text object
	TextLineMatrix matrix.Matrix // reset at the start of each text object

	LineWidth   float64
	LineCap     LineCapStyle
	LineJoin    LineJoinStyle
	MiterLimit  float64
	DashPattern []float64
	DashPhase   float64

	RenderingIntent RenderingIntent

	// StrokeAdjustment is a flag specifying whether to compensate for possible
	// rasterization effects when stroking a path with a line width that is
	// small relative to the pixel resolution of the output device.
	StrokeAdjustment bool

	BlendMode              blend.Mode
	SoftMask               pdf.Object
	StrokeAlpha            float64
	FillAlpha              float64
	AlphaSourceFlag        bool
	BlackPointCompensation pdf.Name

	// The following parameters are device-dependent:

	OverprintStroke bool
	OverprintFill   bool // for PDF<1.3 this must equal OverprintStroke
	OverprintMode   int  // for PDF<1.3 this must be 0

	// BlackGeneration specifies the black generation function to be used for
	// color conversion from DeviceRGB to DeviceCMYK.  The value nil represents
	// the device-specific default function.
	BlackGeneration pdf.Function

	// UndercolorRemoval specifies the undercolor removal function to be used
	// for color conversion from DeviceRGB to DeviceCMYK.  The value nil
	// represents the device-specific default function.
	UndercolorRemoval pdf.Function

	// TransferFunction represents the transfer functions for the individual
	// color components.
	TransferFunction transfer.Functions

	// Halftone specifies the halftone screen to be used.
	// The value nil represents the device-dependent default halftone.
	Halftone halftone.Halftone

	HalftoneOriginX float64 //  https://github.com/pdf-association/pdf-issues/issues/260
	HalftoneOriginY float64

	// FlatnessTolerance is a positive number specifying the precision with
	// which curves are be rendered on the output device.  Smaller numbers give
	// smoother curves, but also increase the amount of computation needed
	// (default: 1).
	FlatnessTolerance float64

	// SmoothnessTolerance is a number in the range 0 to 1 specifying the
	// precision of smooth shading (default: device-dependent).
	SmoothnessTolerance float64
}

// Clone returns a shallow copy of the parameter vector.
func (p *Parameters) Clone() *Parameters {
	res := *p
	return &res
}

// State represents the graphics state of a PDF processor.
type State struct {
	*Parameters
	Set state.Bits
}

// NewState returns a new graphics state with default values,
// and a bit mask indicating which fields are set to their default values.
func NewState() State {
	param := &Parameters{}

	param.CTM = matrix.Identity

	param.AllSubpathsClosed = true
	param.ThisSubpathClosed = true

	param.StrokeColor = color.Black
	param.FillColor = color.Black

	param.TextCharacterSpacing = 0
	param.TextWordSpacing = 0
	param.TextHorizontalScaling = 1
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

	param.RenderingIntent = RelativeColorimetric
	param.StrokeAdjustment = false
	param.BlendMode = blend.Mode{blend.ModeNormal}
	param.SoftMask = nil
	param.StrokeAlpha = 1
	param.FillAlpha = 1
	param.AlphaSourceFlag = false
	param.BlackPointCompensation = pdf.Name("Default")

	param.OverprintStroke = false
	param.OverprintFill = false
	param.OverprintMode = 0

	param.BlackGeneration = nil   // defaul: device dependent
	param.UndercolorRemoval = nil // defaul: device dependent
	param.TransferFunction = transfer.Functions{
		Red:   nil, // defaul: device dependent
		Green: nil, // defaul: device dependent
		Blue:  nil, // defaul: device dependent
		Gray:  nil, // defaul: device dependent
	}

	param.Halftone = nil // defaul: device dependent
	// param.HalftoneOriginX = 0     // defaul: device dependent
	// param.HalftoneOriginY = 0     // defaul: device dependent

	param.FlatnessTolerance = 1
	// param.SmoothnessTolerance = 0 // defaul: device dependent

	return State{param, initializedStateBits}
}

// isSet returns true, if all of the given fields in the graphics state are set.
func (s State) isSet(bits state.Bits) bool {
	return s.Set&bits == bits
}

func (s *State) mustBeSet(bits state.Bits) error {
	missing := ^s.Set & bits
	if missing == 0 {
		return nil
	}
	return state.ErrMissing(missing)
}

// CopyTo applies the graphics state parameters to the given state.
//
// TODO(voss): rename to MergeInto?
func (s State) CopyTo(other *State) {
	set := s.Set
	other.Set |= set

	param := s.Parameters
	otherParam := other.Parameters
	if set&state.TextFont != 0 {
		otherParam.TextFont = param.TextFont
		otherParam.TextFontSize = param.TextFontSize
	}
	if set&state.TextKnockout != 0 {
		otherParam.TextKnockout = param.TextKnockout
	}
	if set&state.LineWidth != 0 {
		otherParam.LineWidth = param.LineWidth
	}
	if set&state.LineCap != 0 {
		otherParam.LineCap = param.LineCap
	}
	if set&state.LineJoin != 0 {
		otherParam.LineJoin = param.LineJoin
	}
	if set&state.MiterLimit != 0 {
		otherParam.MiterLimit = param.MiterLimit
	}
	if set&state.LineDash != 0 {
		otherParam.DashPattern = slices.Clone(param.DashPattern)
		otherParam.DashPhase = param.DashPhase
	}
	if set&state.RenderingIntent != 0 {
		otherParam.RenderingIntent = param.RenderingIntent
	}
	if set&state.StrokeAdjustment != 0 {
		otherParam.StrokeAdjustment = param.StrokeAdjustment
	}
	if set&state.BlendMode != 0 {
		otherParam.BlendMode = param.BlendMode
	}
	if set&state.SoftMask != 0 {
		otherParam.SoftMask = param.SoftMask
	}
	if set&state.StrokeAlpha != 0 {
		otherParam.StrokeAlpha = param.StrokeAlpha
	}
	if set&state.FillAlpha != 0 {
		otherParam.FillAlpha = param.FillAlpha
	}
	if set&state.AlphaSourceFlag != 0 {
		otherParam.AlphaSourceFlag = param.AlphaSourceFlag
	}
	if set&state.BlackPointCompensation != 0 {
		otherParam.BlackPointCompensation = param.BlackPointCompensation
	}
	if set&state.Overprint != 0 {
		otherParam.OverprintStroke = param.OverprintStroke
		otherParam.OverprintFill = param.OverprintFill
	}
	if set&state.OverprintMode != 0 {
		otherParam.OverprintMode = param.OverprintMode
	}
	if set&state.BlackGeneration != 0 {
		otherParam.BlackGeneration = param.BlackGeneration
	}
	if set&state.UndercolorRemoval != 0 {
		otherParam.UndercolorRemoval = param.UndercolorRemoval
	}
	if set&state.TransferFunction != 0 {
		otherParam.TransferFunction = param.TransferFunction
	}
	if set&state.Halftone != 0 {
		otherParam.Halftone = param.Halftone
	}
	if set&state.HalftoneOrigin != 0 {
		otherParam.HalftoneOriginX = param.HalftoneOriginX
		otherParam.HalftoneOriginY = param.HalftoneOriginY
	}
	if set&state.FlatnessTolerance != 0 {
		otherParam.FlatnessTolerance = param.FlatnessTolerance
	}
	if set&state.SmoothnessTolerance != 0 {
		otherParam.SmoothnessTolerance = param.SmoothnessTolerance
	}
}

// GetTextPositionDevice returns the current text position in device coordinates.
func (s State) GetTextPositionDevice() (float64, float64) {
	if err := s.mustBeSet(state.TextFont | state.TextMatrix | state.TextHorizontalScaling | state.TextRise); err != nil {
		panic(err)
	}
	M := matrix.Matrix{s.TextFontSize * s.TextHorizontalScaling, 0, 0, s.TextFontSize, 0, s.TextRise}
	M = M.Mul(s.TextMatrix)
	M = M.Mul(s.CTM)
	return M[4], M[5]
}

// GetTextPositionUser returns the current text position in user coordinates.
func (s State) GetTextPositionUser() (float64, float64) {
	if err := s.mustBeSet(state.TextFont | state.TextMatrix | state.TextHorizontalScaling | state.TextRise); err != nil {
		panic(err)
	}
	M := matrix.Matrix{s.TextFontSize * s.TextHorizontalScaling, 0, 0, s.TextFontSize, 0, s.TextRise}
	M = M.Mul(s.TextMatrix)
	return M[4], M[5]
}

const (
	// initializedStateBits lists the parameters which are initialized to
	// their default values in [NewState].
	initializedStateBits = state.StrokeColor | state.FillColor |
		state.TextCharacterSpacing | state.TextWordSpacing |
		state.TextHorizontalScaling | state.TextLeading | state.TextRenderingMode |
		state.TextRise | state.TextKnockout | state.LineWidth | state.LineCap |
		state.LineJoin | state.MiterLimit | state.LineDash | state.RenderingIntent |
		state.StrokeAdjustment | state.BlendMode | state.SoftMask |
		state.StrokeAlpha | state.FillAlpha | state.AlphaSourceFlag |
		state.BlackPointCompensation | state.Overprint | state.OverprintMode |
		state.FlatnessTolerance

	// extGStateBits lists the graphical parameters which can be encoded in an
	// ExtGState resource.
	extGStateBits = state.TextFont | state.TextKnockout | state.LineWidth |
		state.LineCap | state.LineJoin | state.MiterLimit | state.LineDash |
		state.RenderingIntent | state.StrokeAdjustment | state.BlendMode |
		state.SoftMask | state.StrokeAlpha | state.FillAlpha |
		state.AlphaSourceFlag | state.BlackPointCompensation | state.Overprint |
		state.OverprintMode | state.BlackGeneration | state.UndercolorRemoval |
		state.TransferFunction | state.Halftone | state.HalftoneOrigin |
		state.FlatnessTolerance | state.SmoothnessTolerance
)

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

// A RenderingIntent specifies the PDF rendering intent.
//
// See section 8.6.5.8 of ISO 32000-2:2020.
type RenderingIntent pdf.Name

// The PDF standard rendering intents.
const (
	AbsoluteColorimetric RenderingIntent = "AbsoluteColorimetric"
	RelativeColorimetric RenderingIntent = "RelativeColorimetric"
	Saturation           RenderingIntent = "Saturation"
	Perceptual           RenderingIntent = "Perceptual"
)
