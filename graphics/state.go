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
	CTM matrix.Matrix

	// StrokeColor is the color used for stroking operations.
	StrokeColor color.Color

	// FillColor is the color used for filling and other non-stroking operations.
	FillColor color.Color

	// TextCharacterSpacing is extra spacing between glyphs, in unscaled text space units.
	TextCharacterSpacing float64

	// TextWordSpacing is extra spacing between words, in unscaled text space units.
	TextWordSpacing float64

	// TextHorizontalScaling is the horizontal scaling factor, where 1 means the natural width.
	TextHorizontalScaling float64

	// TextLeading is the vertical distance between baselines, in unscaled text space units.
	TextLeading float64

	// TextFont is the current font.
	TextFont font.Instance

	// TextFontSize is the size at which glyphs are rendered.
	TextFontSize float64

	// TextRenderingMode controls how glyphs are painted.
	TextRenderingMode TextRenderingMode

	// TextRise is the vertical offset for superscript/subscript, in unscaled text space units.
	TextRise float64

	// TextKnockout controls whether overlapping glyphs knock out or overprint.
	TextKnockout bool

	// TextMatrix is the text matrix, reset at the start of each text object.
	TextMatrix matrix.Matrix

	// TextLineMatrix is the text line matrix, reset at the start of each text object.
	TextLineMatrix matrix.Matrix

	// LineWidth is the thickness of stroked paths, in user space units.
	LineWidth float64

	// LineCap is the shape used at the ends of open stroked paths.
	LineCap LineCapStyle

	// LineJoin is the shape used at corners of stroked paths.
	LineJoin LineJoinStyle

	// MiterLimit is the maximum miter length to line width ratio for mitered joins.
	MiterLimit float64

	// DashPattern specifies the lengths of alternating dashes and gaps, in user space units.
	DashPattern []float64

	// DashPhase is the distance into the dash pattern at which to start, in user space units.
	DashPhase float64

	// RenderingIntent specifies how CIE-based colors are converted to device colors.
	RenderingIntent RenderingIntent

	// StrokeAdjustment controls whether to compensate for rasterization effects
	// when stroking paths with small line widths.
	StrokeAdjustment bool

	// BlendMode is the blend mode for the transparent imaging model.
	BlendMode blend.Mode

	// SoftMask specifies mask shape or opacity values for transparency.
	SoftMask SoftClip

	// StrokeAlpha is the constant opacity for stroking operations, from 0 to 1.
	StrokeAlpha float64

	// FillAlpha is the constant opacity for non-stroking operations, from 0 to 1.
	FillAlpha float64

	// AlphaSourceFlag specifies whether soft mask and alpha are interpreted
	// as shape values (true) or opacity values (false).
	AlphaSourceFlag bool

	// BlackPointCompensation controls the black point compensation algorithm
	// for CIE-based color conversions (PDF 2.0).
	BlackPointCompensation pdf.Name

	// OverprintStroke controls whether stroking in one colorant erases other colorants.
	OverprintStroke bool

	// OverprintFill controls whether filling in one colorant erases other colorants.
	OverprintFill bool

	// OverprintMode controls how zero values in DeviceCMYK are treated when overprinting.
	OverprintMode int

	// BlackGeneration specifies the black generation function to be used for
	// color conversion from DeviceRGB to DeviceCMYK.  The value nil represents
	// the device-specific default function.
	BlackGeneration pdf.Function

	// UndercolorRemoval specifies the undercolor removal function to be used
	// for color conversion from DeviceRGB to DeviceCMYK.  The value nil
	// represents the device-specific default function.
	UndercolorRemoval pdf.Function

	// TransferFunction (deprecated in PDF 2.0) represents the transfer
	// functions for the individual color components.
	TransferFunction transfer.Functions

	// Halftone specifies the halftone screen to be used.
	// The value nil represents the device-dependent default halftone.
	Halftone halftone.Halftone

	// HalftoneOriginX (PDF 2.0) is the X coordinate of the halftone origin.
	HalftoneOriginX float64

	// HalftoneOriginY (PDF 2.0) is the Y coordinate of the halftone origin.
	HalftoneOriginY float64

	// FlatnessTolerance is a positive number specifying the precision with
	// which curves are rendered on the output device, in device pixels.
	// Smaller numbers give smoother curves, but also increase the amount of
	// computation needed.
	FlatnessTolerance float64

	// SmoothnessTolerance controls the precision for rendering color
	// gradients.  This is a number from 0 (accurate) to 1 (fast), as a
	// fraction of the range of each color component.
	SmoothnessTolerance float64

	// StartX is the X coordinate of the current subpath's starting point, in user space.
	StartX float64

	// StartY is the Y coordinate of the current subpath's starting point, in user space.
	StartY float64

	// CurrentX is the X coordinate of the current point, in user space.
	CurrentX float64

	// CurrentY is the Y coordinate of the current point, in user space.
	CurrentY float64

	// AllSubpathsClosed is true if all subpaths of the current path are closed.
	AllSubpathsClosed bool

	// ThisSubpathClosed is true if the current subpath is closed.
	ThisSubpathClosed bool
}

// Clone returns a shallow copy of the parameter vector.
func (p *Parameters) Clone() *Parameters {
	res := *p
	return &res
}

// State represents the graphics state of a PDF processor.
type State struct {
	// Parameters holds the actual graphics parameter values.
	*Parameters

	// Set indicates which parameters have been explicitly set.
	Set state.Bits
}

// NewState returns a new graphics state with parameters initialized to
// their default values as defined by the PDF specification. The returned
// State's Set field indicates which parameters have been initialized.
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

	param.BlackGeneration = nil   // default: device dependent
	param.UndercolorRemoval = nil // default: device dependent
	param.TransferFunction = transfer.Functions{
		Red:   nil, // default: device dependent
		Green: nil, // default: device dependent
		Blue:  nil, // default: device dependent
		Gray:  nil, // default: device dependent
	}

	param.Halftone = nil // default: device dependent
	// param.HalftoneOriginX = 0     // default: device dependent
	// param.HalftoneOriginY = 0     // default: device dependent

	param.FlatnessTolerance = 1
	// param.SmoothnessTolerance = 0 // default: device dependent

	return State{param, initializedStateBits}
}

func (s *State) mustBeSet(bits state.Bits) error {
	missing := ^s.Set & bits
	if missing == 0 {
		return nil
	}
	return state.ErrMissing(missing)
}

// ApplyTo applies the graphics state parameters to the given state.
func (s *State) ApplyTo(other *State) {
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
func (s *State) GetTextPositionDevice() (float64, float64) {
	if err := s.mustBeSet(state.TextFont | state.TextMatrix | state.TextHorizontalScaling | state.TextRise); err != nil {
		panic(err)
	}
	M := matrix.Matrix{s.TextFontSize * s.TextHorizontalScaling, 0, 0, s.TextFontSize, 0, s.TextRise}
	M = M.Mul(s.TextMatrix)
	M = M.Mul(s.CTM)
	return M[4], M[5]
}

// GetTextPositionUser returns the current text position in user coordinates.
func (s *State) GetTextPositionUser() (float64, float64) {
	if err := s.mustBeSet(state.TextFont | state.TextMatrix | state.TextHorizontalScaling | state.TextRise); err != nil {
		panic(err)
	}
	M := matrix.Matrix{s.TextFontSize * s.TextHorizontalScaling, 0, 0, s.TextFontSize, 0, s.TextRise}
	M = M.Mul(s.TextMatrix)
	return M[4], M[5]
}

// initializedStateBits lists the parameters which are initialized to
// their default values in [NewState].
const initializedStateBits = state.StrokeColor | state.FillColor |
	state.TextCharacterSpacing | state.TextWordSpacing |
	state.TextHorizontalScaling | state.TextLeading | state.TextRenderingMode |
	state.TextRise | state.TextKnockout | state.LineWidth | state.LineCap |
	state.LineJoin | state.MiterLimit | state.LineDash | state.RenderingIntent |
	state.StrokeAdjustment | state.BlendMode | state.SoftMask |
	state.StrokeAlpha | state.FillAlpha | state.AlphaSourceFlag |
	state.BlackPointCompensation | state.Overprint | state.OverprintMode |
	state.FlatnessTolerance
