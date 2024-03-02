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
	"math/bits"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/matrix"
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

	StrokeColor color.Color
	FillColor   color.Color

	// Text State parameters:
	TextCharacterSpacing  float64 // character spacing (T_c)
	TextWordSpacing       float64 // word spacing (T_w)
	TextHorizontalScaling float64 // horizonal scaling (T_h, normal spacing = 1)
	TextLeading           float64 // leading (T_l)
	TextFont              font.Embedded
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
	StateTextHorizontalScaling
	StateTextLeading
	StateTextFont // includes size
	StateTextRenderingMode
	StateTextRise
	StateTextKnockout

	StateTextMatrix // text matrix and text line matrix

	StateLineWidth
	StateLineCap
	StateLineJoin
	StateMiterLimit
	StateLineDash // pattern and phase

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

var stateNames = []string{
	"StrokeColor",
	"FillColor",
	"TextCharacterSpacing",
	"TextWordSpacing",
	"TextHorizontalScaling",
	"TextLeading",
	"TextFont",
	"TextRenderingMode",
	"TextRise",
	"TextKnockout",
	"TextMatrix",
	"LineWidth",
	"LineCap",
	"LineJoin",
	"MiterLimit",
	"Dash",
	"RenderingIntent",
	"StrokeAdjustment",
	"BlendMode",
	"SoftMask",
	"StrokeAlpha",
	"FillAlpha",
	"AlphaSourceFlag",
	"BlackPointCompensation",
	"Overprint",
	"OverprintMode",
	"BlackGeneration",
	"UndercolorRemoval",
	"TransferFunction",
	"Halftone",
	"HalftoneOrigin",
	"FlatnessTolerance",
	"SmoothnessTolerance",
}

const (
	// initializedStateBits lists the parameters which are initialized to
	// their default values in [NewState].
	initializedStateBits = StateStrokeColor | StateFillColor |
		StateTextCharacterSpacing | StateTextWordSpacing |
		StateTextHorizontalScaling | StateTextLeading | StateTextRenderingMode |
		StateTextRise | StateTextKnockout | StateLineWidth | StateLineCap |
		StateLineJoin | StateMiterLimit | StateLineDash | StateRenderingIntent |
		StateStrokeAdjustment | StateBlendMode | StateSoftMask |
		StateStrokeAlpha | StateFillAlpha | StateAlphaSourceFlag |
		StateBlackPointCompensation | StateOverprint | StateOverprintMode |
		StateFlatnessTolerance

	// extStateBits lists the parameters which can be encoded in an ExtGState
	// resource.
	extStateBits = StateTextFont | StateTextKnockout | StateLineWidth |
		StateLineCap | StateLineJoin | StateMiterLimit | StateLineDash |
		StateRenderingIntent | StateStrokeAdjustment | StateBlendMode |
		StateSoftMask | StateStrokeAlpha | StateFillAlpha |
		StateAlphaSourceFlag | StateBlackPointCompensation | StateOverprint |
		StateOverprintMode | StateBlackGeneration | StateUndercolorRemoval |
		StateTransferFunction | StateHalftone | StateHalftoneOrigin |
		StateFlatnessTolerance | StateSmoothnessTolerance

	// TODO(voss): update this once
	// https://github.com/pdf-association/pdf-issues/issues/380
	// is resolved
	strokeStateBits = StateLineWidth | StateLineCap | StateLineJoin | StateLineDash | StateStrokeColor
	fillStateBits   = StateFillColor
)

// State represents the graphics state of a PDF processor.
type State struct {
	*Parameters
	Set StateBits
}

// NewState returns a new graphics state with default values,
// and a bit mask indicating which fields are set to their default values.
func NewState() State {
	param := &Parameters{}

	param.CTM = matrix.Identity

	param.StrokeColor = color.DeviceGray.New(0)
	param.FillColor = color.DeviceGray.New(0)

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

	return State{param, initializedStateBits}
}

// isSet returns true, if all of the given fields in the graphics state are set.
func (s State) isSet(bits StateBits) bool {
	return s.Set&bits == bits
}

func (s *State) mustBeSet(bits StateBits) error {
	missing := ^s.Set & bits
	if missing == 0 {
		return nil
	}
	return errMissingState(missing)
}

// ApplyTo applies the graphics state parameters to the given state.
func (s State) ApplyTo(other *State) {
	set := s.Set
	other.Set |= set

	param := s.Parameters
	otherParam := other.Parameters
	if set&StateTextFont != 0 {
		otherParam.TextFont = param.TextFont
		otherParam.TextFontSize = param.TextFontSize
	}
	if set&StateTextKnockout != 0 {
		otherParam.TextKnockout = param.TextKnockout
	}
	if set&StateLineWidth != 0 {
		otherParam.LineWidth = param.LineWidth
	}
	if set&StateLineCap != 0 {
		otherParam.LineCap = param.LineCap
	}
	if set&StateLineJoin != 0 {
		otherParam.LineJoin = param.LineJoin
	}
	if set&StateMiterLimit != 0 {
		otherParam.MiterLimit = param.MiterLimit
	}
	if set&StateLineDash != 0 {
		otherParam.DashPattern = slices.Clone(param.DashPattern)
		otherParam.DashPhase = param.DashPhase
	}
	if set&StateRenderingIntent != 0 {
		otherParam.RenderingIntent = param.RenderingIntent
	}
	if set&StateStrokeAdjustment != 0 {
		otherParam.StrokeAdjustment = param.StrokeAdjustment
	}
	if set&StateBlendMode != 0 {
		otherParam.BlendMode = param.BlendMode
	}
	if set&StateSoftMask != 0 {
		otherParam.SoftMask = param.SoftMask
	}
	if set&StateStrokeAlpha != 0 {
		otherParam.StrokeAlpha = param.StrokeAlpha
	}
	if set&StateFillAlpha != 0 {
		otherParam.FillAlpha = param.FillAlpha
	}
	if set&StateAlphaSourceFlag != 0 {
		otherParam.AlphaSourceFlag = param.AlphaSourceFlag
	}
	if set&StateBlackPointCompensation != 0 {
		otherParam.BlackPointCompensation = param.BlackPointCompensation
	}
	if set&StateOverprint != 0 {
		otherParam.OverprintStroke = param.OverprintStroke
		otherParam.OverprintFill = param.OverprintFill
	}
	if set&StateOverprintMode != 0 {
		otherParam.OverprintMode = param.OverprintMode
	}
	if set&StateBlackGeneration != 0 {
		otherParam.BlackGeneration = param.BlackGeneration
	}
	if set&StateUndercolorRemoval != 0 {
		otherParam.UndercolorRemoval = param.UndercolorRemoval
	}
	if set&StateTransferFunction != 0 {
		otherParam.TransferFunction = param.TransferFunction
	}
	if set&StateHalftone != 0 {
		otherParam.Halftone = param.Halftone
	}
	if set&StateHalftoneOrigin != 0 {
		otherParam.HalftoneOriginX = param.HalftoneOriginX
		otherParam.HalftoneOriginY = param.HalftoneOriginY
	}
	if set&StateFlatnessTolerance != 0 {
		otherParam.FlatnessTolerance = param.FlatnessTolerance
	}
	if set&StateSmoothnessTolerance != 0 {
		otherParam.SmoothnessTolerance = param.SmoothnessTolerance
	}
}

type errMissingState StateBits

func (e errMissingState) Error() string {
	k := bits.TrailingZeros64(uint64(e))
	return stateNames[k] + " not set"
}

// Clone returns a shallow copy of the parameter vector.
func (s *Parameters) Clone() *Parameters {
	res := *s
	return &res
}

// GetTextPositionDevice returns the current text position in device coordinates.
func (s State) GetTextPositionDevice() (float64, float64) {
	if err := s.mustBeSet(StateTextFont | StateTextMatrix | StateTextHorizontalScaling | StateTextRise); err != nil {
		panic(err)
	}
	M := matrix.Matrix{s.TextFontSize * s.TextHorizontalScaling, 0, 0, s.TextFontSize, 0, s.TextRise}
	M = M.Mul(s.TextMatrix)
	M = M.Mul(s.CTM)
	return M[4], M[5]
}

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
