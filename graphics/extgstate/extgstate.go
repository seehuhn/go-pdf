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

// Package extgstate provides the ExtGState type for PDF extended graphics
// state dictionaries.
package extgstate

import (
	"errors"
	"fmt"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
)

// PDF 2.0 sections: 8.4.5

// ExtGState represents a combination of graphics state parameters. This
// combination of parameters can then be set in a single command, using the
// [Writer.SetExtGState] method.  The parameters here form a subset of the
// parameters in the [graphics.State] struct.  Parameters not present in the
// ExtGState struct, for example colors, cannot be controlled using an extended
// graphics state.
type ExtGState struct {
	// Set indicates which parameters in this ExtGState are active.
	Set graphics.Bits

	// TextFont is the current font.
	TextFont font.Instance

	// TextFontSize is the size at which glyphs are rendered.
	TextFontSize float64

	// TextKnockout controls whether overlapping glyphs knock out or overprint.
	TextKnockout bool

	// LineWidth is the thickness of stroked paths, in user space units.
	LineWidth float64

	// LineCap is the shape used at the ends of open stroked paths.
	LineCap graphics.LineCapStyle

	// LineJoin is the shape used at corners of stroked paths.
	LineJoin graphics.LineJoinStyle

	// MiterLimit is the maximum miter length to line width ratio for mitered joins.
	MiterLimit float64

	// DashPattern specifies the lengths of alternating dashes and gaps, in user space units.
	DashPattern []float64

	// DashPhase is the distance into the dash pattern at which to start, in user space units.
	DashPhase float64

	// RenderingIntent specifies how CIE-based colors are converted to device colors.
	RenderingIntent graphics.RenderingIntent

	// StrokeAdjustment controls whether to compensate for rasterization effects
	// when stroking paths with small line widths.
	StrokeAdjustment bool

	// BlendMode is the blend mode for the transparent imaging model.
	BlendMode graphics.BlendMode

	// SoftMask specifies mask shape or opacity values for transparency.
	SoftMask graphics.SoftClip

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

	// TransferFunctions (deprecated in PDF 2.0) represents the transfer
	// functions for the individual color components.
	TransferFunctions graphics.TransferFunctions

	// Halftone specifies the halftone screen to be used.
	// The value nil represents the device-dependent default halftone.
	Halftone graphics.Halftone

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

	// SingleUse can be set if the extended graphics state is used only in a
	// single content stream, in order to slightly reduce output file size. In
	// this case, the graphics state is embedded in the corresponding resource
	// dictionary, instead of being stored as an indirect object.
	SingleUse bool
}

// extGStateBits lists the graphical parameters which can be encoded in an
// ExtGState resource.
var extGStateBits = graphics.StateTextFont | graphics.StateTextKnockout | graphics.StateLineWidth |
	graphics.StateLineCap | graphics.StateLineJoin | graphics.StateMiterLimit | graphics.StateLineDash |
	graphics.StateRenderingIntent | graphics.StateStrokeAdjustment | graphics.StateBlendMode |
	graphics.StateSoftMask | graphics.StateStrokeAlpha | graphics.StateFillAlpha |
	graphics.StateAlphaSourceFlag | graphics.StateBlackPointCompensation | graphics.StateOverprint |
	graphics.StateOverprintMode | graphics.StateBlackGeneration | graphics.StateUndercolorRemoval |
	graphics.StateTransferFunction | graphics.StateHalftone | graphics.StateHalftoneOrigin |
	graphics.StateFlatnessTolerance | graphics.StateSmoothnessTolerance

// Equal reports whether two ExtGState values are equal.
func (e *ExtGState) Equal(other *ExtGState) bool {
	if e == nil || other == nil {
		return e == nil && other == nil
	}

	if e.Set != other.Set || e.SingleUse != other.SingleUse {
		return false
	}

	set := e.Set

	if set&graphics.StateTextFont != 0 {
		if !font.InstancesEqual(e.TextFont, other.TextFont) ||
			e.TextFontSize != other.TextFontSize {
			return false
		}
	}
	if set&graphics.StateTextKnockout != 0 && e.TextKnockout != other.TextKnockout {
		return false
	}
	if set&graphics.StateLineWidth != 0 && e.LineWidth != other.LineWidth {
		return false
	}
	if set&graphics.StateLineCap != 0 && e.LineCap != other.LineCap {
		return false
	}
	if set&graphics.StateLineJoin != 0 && e.LineJoin != other.LineJoin {
		return false
	}
	if set&graphics.StateMiterLimit != 0 && e.MiterLimit != other.MiterLimit {
		return false
	}
	if set&graphics.StateLineDash != 0 {
		if !slices.Equal(e.DashPattern, other.DashPattern) ||
			e.DashPhase != other.DashPhase {
			return false
		}
	}
	if set&graphics.StateRenderingIntent != 0 && e.RenderingIntent != other.RenderingIntent {
		return false
	}
	if set&graphics.StateStrokeAdjustment != 0 && e.StrokeAdjustment != other.StrokeAdjustment {
		return false
	}
	if set&graphics.StateBlendMode != 0 && !e.BlendMode.Equal(other.BlendMode) {
		return false
	}
	if set&graphics.StateSoftMask != 0 {
		if (e.SoftMask == nil) != (other.SoftMask == nil) {
			return false
		}
		if e.SoftMask != nil && !e.SoftMask.Equal(other.SoftMask) {
			return false
		}
	}
	if set&graphics.StateStrokeAlpha != 0 && e.StrokeAlpha != other.StrokeAlpha {
		return false
	}
	if set&graphics.StateFillAlpha != 0 && e.FillAlpha != other.FillAlpha {
		return false
	}
	if set&graphics.StateAlphaSourceFlag != 0 && e.AlphaSourceFlag != other.AlphaSourceFlag {
		return false
	}
	if set&graphics.StateBlackPointCompensation != 0 && e.BlackPointCompensation != other.BlackPointCompensation {
		return false
	}
	if set&graphics.StateOverprint != 0 {
		if e.OverprintStroke != other.OverprintStroke ||
			e.OverprintFill != other.OverprintFill {
			return false
		}
	}
	if set&graphics.StateOverprintMode != 0 && e.OverprintMode != other.OverprintMode {
		return false
	}
	if set&graphics.StateBlackGeneration != 0 && !function.Equal(e.BlackGeneration, other.BlackGeneration) {
		return false
	}
	if set&graphics.StateUndercolorRemoval != 0 && !function.Equal(e.UndercolorRemoval, other.UndercolorRemoval) {
		return false
	}
	if set&graphics.StateTransferFunction != 0 {
		if !function.Equal(e.TransferFunctions.Red, other.TransferFunctions.Red) ||
			!function.Equal(e.TransferFunctions.Green, other.TransferFunctions.Green) ||
			!function.Equal(e.TransferFunctions.Blue, other.TransferFunctions.Blue) ||
			!function.Equal(e.TransferFunctions.Gray, other.TransferFunctions.Gray) {
			return false
		}
	}
	if set&graphics.StateHalftone != 0 {
		// Compare halftones by type and transfer function
		if (e.Halftone == nil) != (other.Halftone == nil) {
			return false
		}
		if e.Halftone != nil {
			if e.Halftone.HalftoneType() != other.Halftone.HalftoneType() {
				return false
			}
			if !function.Equal(e.Halftone.GetTransferFunction(), other.Halftone.GetTransferFunction()) {
				return false
			}
		}
	}
	if set&graphics.StateHalftoneOrigin != 0 {
		if e.HalftoneOriginX != other.HalftoneOriginX ||
			e.HalftoneOriginY != other.HalftoneOriginY {
			return false
		}
	}
	if set&graphics.StateFlatnessTolerance != 0 && e.FlatnessTolerance != other.FlatnessTolerance {
		return false
	}
	if set&graphics.StateSmoothnessTolerance != 0 && e.SmoothnessTolerance != other.SmoothnessTolerance {
		return false
	}

	return true
}

// Embed adds the graphics state dictionary to a PDF file.
//
// This implements the [pdf.Embedder] interface.
func (e *ExtGState) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "ExtGState", pdf.V1_2); err != nil {
		return nil, err
	}

	set := e.Set
	if excess := set &^ extGStateBits; excess != 0 {
		return nil, fmt.Errorf("unsupported graphics state bits: 0b%b", excess)
	}

	// Build a graphics state parameter dictionary for the given state.
	// See table 57 in ISO 32000-2:2020.
	dict := pdf.Dict{}
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("ExtGState")
	}
	if set&graphics.StateTextFont != 0 {
		E, err := rm.Embed(e.TextFont)
		if err != nil {
			return nil, err
		}
		if _, ok := E.(pdf.Reference); !ok {
			err := fmt.Errorf("font %q cannot be used in ExtGState",
				e.TextFont.PostScriptName())
			return nil, err
		}
		dict["Font"] = pdf.Array{E, pdf.Number(e.TextFontSize)}
	} else {
		if e.TextFont != nil {
			return nil, errors.New("unexpected TextFont value")
		}
		if e.TextFontSize != 0 {
			return nil, errors.New("unexpected TextFontSize value")
		}
	}
	if set&graphics.StateTextKnockout != 0 {
		dict["TK"] = pdf.Boolean(e.TextKnockout)
	} else {
		if e.TextKnockout {
			return nil, errors.New("unexpected TextKnockout value")
		}
	}
	if set&graphics.StateLineWidth != 0 {
		dict["LW"] = pdf.Number(e.LineWidth)
	} else {
		if e.LineWidth != 0 {
			return nil, errors.New("unexpected LineWidth value")
		}
	}
	if set&graphics.StateLineCap != 0 {
		dict["LC"] = pdf.Integer(e.LineCap)
	} else {
		if e.LineCap != 0 {
			return nil, errors.New("unexpected LineCap value")
		}
	}
	if set&graphics.StateLineJoin != 0 {
		dict["LJ"] = pdf.Integer(e.LineJoin)
	} else {
		if e.LineJoin != 0 {
			return nil, errors.New("unexpected LineJoin value")
		}
	}
	if set&graphics.StateMiterLimit != 0 {
		dict["ML"] = pdf.Number(e.MiterLimit)
	} else {
		if e.MiterLimit != 0 {
			return nil, errors.New("unexpected MiterLimit value")
		}
	}
	if set&graphics.StateLineDash != 0 {
		pat := make(pdf.Array, len(e.DashPattern))
		for i, x := range e.DashPattern {
			pat[i] = pdf.Number(x)
		}
		dict["D"] = pdf.Array{
			pat,
			pdf.Number(e.DashPhase),
		}
	} else {
		if e.DashPattern != nil {
			return nil, errors.New("unexpected DashPattern value")
		}
		if e.DashPhase != 0 {
			return nil, errors.New("unexpected DashPhase value")
		}
	}
	if set&graphics.StateRenderingIntent != 0 {
		dict["RI"] = pdf.Name(e.RenderingIntent)
	} else {
		if e.RenderingIntent != "" {
			return nil, errors.New("unexpected RenderingIntent value")
		}
	}
	if set&graphics.StateStrokeAdjustment != 0 {
		dict["SA"] = pdf.Boolean(e.StrokeAdjustment)
	} else {
		if e.StrokeAdjustment {
			return nil, errors.New("unexpected StrokeAdjustment value")
		}
	}
	if set&graphics.StateBlendMode != 0 {
		dict["BM"] = e.BlendMode.AsPDF()
	} else {
		if !e.BlendMode.IsZero() {
			return nil, errors.New("unexpected BlendMode value")
		}
	}
	if set&graphics.StateSoftMask != 0 {
		if e.SoftMask == nil {
			dict["SMask"] = pdf.Name("None")
		} else {
			sMask, err := rm.Embed(e.SoftMask)
			if err != nil {
				return nil, err
			}
			dict["SMask"] = sMask
		}
	} else {
		if e.SoftMask != nil {
			return nil, errors.New("unexpected SoftMask value")
		}
	}
	if set&graphics.StateStrokeAlpha != 0 {
		dict["CA"] = pdf.Number(e.StrokeAlpha)
	} else {
		if e.StrokeAlpha != 0 {
			return nil, errors.New("unexpected StrokeAlpha value")
		}
	}
	if set&graphics.StateFillAlpha != 0 {
		dict["ca"] = pdf.Number(e.FillAlpha)
	} else {
		if e.FillAlpha != 0 {
			return nil, errors.New("unexpected FillAlpha value")
		}
	}
	if set&graphics.StateAlphaSourceFlag != 0 {
		dict["AIS"] = pdf.Boolean(e.AlphaSourceFlag)
	} else {
		if e.AlphaSourceFlag {
			return nil, errors.New("unexpected AlphaSourceFlag value")
		}
	}
	if set&graphics.StateBlackPointCompensation != 0 {
		dict["UseBlackPtComp"] = e.BlackPointCompensation
	} else {
		if e.BlackPointCompensation != "" {
			return nil, errors.New("unexpected BlackPointCompensation value")
		}
	}

	if set&graphics.StateOverprint != 0 {
		dict["OP"] = pdf.Boolean(e.OverprintStroke)
		if e.OverprintFill != e.OverprintStroke {
			dict["op"] = pdf.Boolean(e.OverprintFill)
		}
	} else {
		if e.OverprintStroke {
			return nil, errors.New("unexpected OverprintStroke value")
		}
		if e.OverprintFill {
			return nil, errors.New("unexpected OverprintFill value")
		}
	}
	if set&graphics.StateOverprintMode != 0 {
		dict["OPM"] = pdf.Integer(e.OverprintMode)
	} else {
		if e.OverprintMode != 0 {
			return nil, errors.New("unexpected OverprintMode value")
		}
	}
	if set&graphics.StateBlackGeneration != 0 {
		if e.BlackGeneration == nil {
			if err := pdf.CheckVersion(rm.Out(), "BG2 in ExtGState", pdf.V1_3); err != nil {
				return nil, err
			}
			dict["BG2"] = pdf.Name("Default")
		} else {
			obj, err := rm.Embed(e.BlackGeneration)
			if err != nil {
				return nil, err
			}
			dict["BG"] = obj
		}
	} else {
		if e.BlackGeneration != nil {
			return nil, errors.New("unexpected BlackGeneration value")
		}
	}
	if set&graphics.StateUndercolorRemoval != 0 {
		if e.UndercolorRemoval == nil {
			if err := pdf.CheckVersion(rm.Out(), "UCR2 in ExtGState", pdf.V1_3); err != nil {
				return nil, err
			}
			dict["UCR2"] = pdf.Name("Default")
		} else {
			obj, err := rm.Embed(e.UndercolorRemoval)
			if err != nil {
				return nil, err
			}
			dict["UCR"] = obj
		}
	} else {
		if e.UndercolorRemoval != nil {
			return nil, errors.New("unexpected UndercolorRemoval value")
		}
	}
	if set&graphics.StateTransferFunction != 0 {
		if v := pdf.GetVersion(rm.Out()); v >= pdf.V2_0 {
			return nil, errors.New("TransferFunction is deprecated in PDF 2.0")
		}
		all := []pdf.Function{
			e.TransferFunctions.Red,
			e.TransferFunctions.Green,
			e.TransferFunctions.Blue,
			e.TransferFunctions.Gray,
		}
		needsArray := false
		key := pdf.Name("TR")
		for _, fn := range all {
			if fn != all[0] {
				needsArray = true
			}
			if fn == nil {
				if err := pdf.CheckVersion(rm.Out(), "TR2 in ExtGState", pdf.V1_3); err != nil {
					return nil, err
				}
				key = "TR2"
			} else if nIn, nOut := fn.Shape(); nIn != 1 || nOut != 1 {
				return nil, fmt.Errorf("wrong transfer function shape (%d,%d) != (1,1)", nIn, nOut)
			}
		}
		a := make(pdf.Array, len(all))
		for i, fn := range all {
			var obj pdf.Object
			switch fn {
			case nil:
				obj = pdf.Name("Default")
			case function.Identity:
				obj = pdf.Name("Identity")
			default:
				var err error
				obj, err = rm.Embed(fn)
				if err != nil {
					return nil, err
				}
			}
			a[i] = obj
		}
		if needsArray {
			dict[key] = a
		} else {
			dict[key] = a[0]
		}
	} else {
		if e.TransferFunctions.Red != nil || e.TransferFunctions.Green != nil ||
			e.TransferFunctions.Blue != nil || e.TransferFunctions.Gray != nil {
			return nil, errors.New("unexpected TransferFunction value")
		}
	}
	if set&graphics.StateHalftone != 0 {
		htEmbedded, err := rm.Embed(e.Halftone)
		if err != nil {
			return nil, err
		}
		dict["HT"] = htEmbedded
	} else {
		if e.Halftone != nil {
			return nil, errors.New("unexpected Halftone value")
		}
	}
	if set&graphics.StateHalftoneOrigin != 0 {
		dict["HTO"] = pdf.Array{
			pdf.Number(e.HalftoneOriginX),
			pdf.Number(e.HalftoneOriginY),
		}
	} else {
		if e.HalftoneOriginX != 0 {
			return nil, errors.New("unexpected HalftoneOriginX value")
		}
		if e.HalftoneOriginY != 0 {
			return nil, errors.New("unexpected HalftoneOriginY value")
		}
	}
	if set&graphics.StateFlatnessTolerance != 0 {
		dict["FL"] = pdf.Number(e.FlatnessTolerance)
	} else {
		if e.FlatnessTolerance != 0 {
			return nil, errors.New("unexpected FlatnessTolerance value")
		}
	}
	if set&graphics.StateSmoothnessTolerance != 0 {
		dict["SM"] = pdf.Number(e.SmoothnessTolerance)
	} else {
		if e.SmoothnessTolerance != 0 {
			return nil, errors.New("unexpected SmoothnessTolerance value")
		}
	}

	if e.SingleUse {
		return dict, nil
	}
	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// ApplyTo modifies the given graphics state according to the parameters in
// the extended graphics state.
func (e *ExtGState) ApplyTo(s *graphics.State) {
	set := e.Set
	s.Set |= set

	if set&graphics.StateTextFont != 0 {
		s.TextFont = e.TextFont
		s.TextFontSize = e.TextFontSize
	}
	if set&graphics.StateTextKnockout != 0 {
		s.TextKnockout = e.TextKnockout
	}
	if set&graphics.StateLineWidth != 0 {
		s.LineWidth = e.LineWidth
	}
	if set&graphics.StateLineCap != 0 {
		s.LineCap = e.LineCap
	}
	if set&graphics.StateLineJoin != 0 {
		s.LineJoin = e.LineJoin
	}
	if set&graphics.StateMiterLimit != 0 {
		s.MiterLimit = e.MiterLimit
	}
	if set&graphics.StateLineDash != 0 {
		s.DashPattern = slices.Clone(e.DashPattern)
		s.DashPhase = e.DashPhase
	}
	if set&graphics.StateRenderingIntent != 0 {
		s.RenderingIntent = e.RenderingIntent
	}
	if set&graphics.StateStrokeAdjustment != 0 {
		s.StrokeAdjustment = e.StrokeAdjustment
	}
	if set&graphics.StateBlendMode != 0 {
		s.BlendMode = e.BlendMode
	}
	if set&graphics.StateSoftMask != 0 {
		s.SoftMask = e.SoftMask
	}
	if set&graphics.StateStrokeAlpha != 0 {
		s.StrokeAlpha = e.StrokeAlpha
	}
	if set&graphics.StateFillAlpha != 0 {
		s.FillAlpha = e.FillAlpha
	}
	if set&graphics.StateAlphaSourceFlag != 0 {
		s.AlphaSourceFlag = e.AlphaSourceFlag
	}
	if set&graphics.StateBlackPointCompensation != 0 {
		s.BlackPointCompensation = e.BlackPointCompensation
	}
	if set&graphics.StateOverprint != 0 {
		s.OverprintStroke = e.OverprintStroke
		s.OverprintFill = e.OverprintFill
	}
	if set&graphics.StateOverprintMode != 0 {
		s.OverprintMode = e.OverprintMode
	}
	if set&graphics.StateBlackGeneration != 0 {
		s.BlackGeneration = e.BlackGeneration
	}
	if set&graphics.StateUndercolorRemoval != 0 {
		s.UndercolorRemoval = e.UndercolorRemoval
	}
	if set&graphics.StateTransferFunction != 0 {
		s.TransferFunctions = e.TransferFunctions
	}
	if set&graphics.StateHalftone != 0 {
		s.Halftone = e.Halftone
	}
	if set&graphics.StateHalftoneOrigin != 0 {
		s.HalftoneOriginX = e.HalftoneOriginX
		s.HalftoneOriginY = e.HalftoneOriginY
	}
	if set&graphics.StateFlatnessTolerance != 0 {
		s.FlatnessTolerance = e.FlatnessTolerance
	}
	if set&graphics.StateSmoothnessTolerance != 0 {
		s.SmoothnessTolerance = e.SmoothnessTolerance
	}
}
