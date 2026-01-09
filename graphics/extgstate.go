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
	"errors"
	"fmt"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/blend"
	"seehuhn.de/go/pdf/graphics/halftone"
	"seehuhn.de/go/pdf/graphics/state"
	"seehuhn.de/go/pdf/graphics/transfer"
)

// PDF 2.0 sections: 8.4.5

// ExtGState represents a combination of graphics state parameters. This
// combination of parameters can then be set in a single command, using the
// [Writer.SetExtGState] method.  The parameters here form a subset of the
// parameters in the [Parameters] struct.  Parameters not present in the
// ExtGState struct, for example colors, cannot be controlled using an extended
// graphics state.
type ExtGState struct {
	Set state.Bits

	TextFont               font.Instance
	TextFontSize           float64
	TextKnockout           bool
	LineWidth              float64
	LineCap                LineCapStyle
	LineJoin               LineJoinStyle
	MiterLimit             float64
	DashPattern            []float64
	DashPhase              float64
	RenderingIntent        RenderingIntent
	StrokeAdjustment       bool
	BlendMode              blend.Mode
	SoftMask               SoftClip
	StrokeAlpha            float64
	FillAlpha              float64
	AlphaSourceFlag        bool
	BlackPointCompensation pdf.Name
	OverprintStroke        bool
	OverprintFill          bool // for PDF<1.3 this must equal OverprintStroke
	OverprintMode          int  // for PDF<1.3 this must be 0

	// BlackGeneration specifies the black generation function to be used for
	// color conversion from DeviceRGB to DeviceCMYK.  The value nil represents
	// the device-specific default function.
	BlackGeneration pdf.Function

	// UndercolorRemoval specifies the undercolor removal function to be used
	// for color conversion from DeviceRGB to DeviceCMYK.  The value nil
	// represents the device-specific default function.
	UndercolorRemoval pdf.Function

	// TransferFunction specifies the transfer functions for the color
	// components.
	TransferFunction transfer.Functions // deprecated in PDF 2.0

	// Halftone specifies the halftone screen to be used.
	// The value nil represents the device-dependent default halftone.
	Halftone halftone.Halftone

	HalftoneOriginX float64
	HalftoneOriginY float64

	FlatnessTolerance   float64
	SmoothnessTolerance float64

	// SingleUse can be set if the extended graphics state is used only in a
	// single content stream, in order to slightly reduce output file size.  In
	// this case, the graphics state is embedded in the corresponding resource
	// dictionary, instead of being stored as an indirect object.
	SingleUse bool
}

// Equal reports whether two ExtGState values are equal.
func (e *ExtGState) Equal(other *ExtGState) bool {
	if e == nil || other == nil {
		return e == nil && other == nil
	}

	if e.Set != other.Set || e.SingleUse != other.SingleUse {
		return false
	}

	set := e.Set

	if set&state.TextFont != 0 {
		if !font.InstancesEqual(e.TextFont, other.TextFont) ||
			e.TextFontSize != other.TextFontSize {
			return false
		}
	}
	if set&state.TextKnockout != 0 && e.TextKnockout != other.TextKnockout {
		return false
	}
	if set&state.LineWidth != 0 && e.LineWidth != other.LineWidth {
		return false
	}
	if set&state.LineCap != 0 && e.LineCap != other.LineCap {
		return false
	}
	if set&state.LineJoin != 0 && e.LineJoin != other.LineJoin {
		return false
	}
	if set&state.MiterLimit != 0 && e.MiterLimit != other.MiterLimit {
		return false
	}
	if set&state.LineDash != 0 {
		if !slices.Equal(e.DashPattern, other.DashPattern) ||
			e.DashPhase != other.DashPhase {
			return false
		}
	}
	if set&state.RenderingIntent != 0 && e.RenderingIntent != other.RenderingIntent {
		return false
	}
	if set&state.StrokeAdjustment != 0 && e.StrokeAdjustment != other.StrokeAdjustment {
		return false
	}
	if set&state.BlendMode != 0 && !e.BlendMode.Equal(other.BlendMode) {
		return false
	}
	if set&state.SoftMask != 0 {
		if (e.SoftMask == nil) != (other.SoftMask == nil) {
			return false
		}
		if e.SoftMask != nil && !e.SoftMask.Equal(other.SoftMask) {
			return false
		}
	}
	if set&state.StrokeAlpha != 0 && e.StrokeAlpha != other.StrokeAlpha {
		return false
	}
	if set&state.FillAlpha != 0 && e.FillAlpha != other.FillAlpha {
		return false
	}
	if set&state.AlphaSourceFlag != 0 && e.AlphaSourceFlag != other.AlphaSourceFlag {
		return false
	}
	if set&state.BlackPointCompensation != 0 && e.BlackPointCompensation != other.BlackPointCompensation {
		return false
	}
	if set&state.Overprint != 0 {
		if e.OverprintStroke != other.OverprintStroke ||
			e.OverprintFill != other.OverprintFill {
			return false
		}
	}
	if set&state.OverprintMode != 0 && e.OverprintMode != other.OverprintMode {
		return false
	}
	if set&state.BlackGeneration != 0 && !function.Equal(e.BlackGeneration, other.BlackGeneration) {
		return false
	}
	if set&state.UndercolorRemoval != 0 && !function.Equal(e.UndercolorRemoval, other.UndercolorRemoval) {
		return false
	}
	if set&state.TransferFunction != 0 {
		if !function.Equal(e.TransferFunction.Red, other.TransferFunction.Red) ||
			!function.Equal(e.TransferFunction.Green, other.TransferFunction.Green) ||
			!function.Equal(e.TransferFunction.Blue, other.TransferFunction.Blue) ||
			!function.Equal(e.TransferFunction.Gray, other.TransferFunction.Gray) {
			return false
		}
	}
	if set&state.Halftone != 0 {
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
	if set&state.HalftoneOrigin != 0 {
		if e.HalftoneOriginX != other.HalftoneOriginX ||
			e.HalftoneOriginY != other.HalftoneOriginY {
			return false
		}
	}
	if set&state.FlatnessTolerance != 0 && e.FlatnessTolerance != other.FlatnessTolerance {
		return false
	}
	if set&state.SmoothnessTolerance != 0 && e.SmoothnessTolerance != other.SmoothnessTolerance {
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
	if set&state.TextFont != 0 {
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
	if set&state.TextKnockout != 0 {
		dict["TK"] = pdf.Boolean(e.TextKnockout)
	} else {
		if e.TextKnockout {
			return nil, errors.New("unexpected TextKnockout value")
		}
	}
	if set&state.LineWidth != 0 {
		dict["LW"] = pdf.Number(e.LineWidth)
	} else {
		if e.LineWidth != 0 {
			return nil, errors.New("unexpected LineWidth value")
		}
	}
	if set&state.LineCap != 0 {
		dict["LC"] = pdf.Integer(e.LineCap)
	} else {
		if e.LineCap != 0 {
			return nil, errors.New("unexpected LineCap value")
		}
	}
	if set&state.LineJoin != 0 {
		dict["LJ"] = pdf.Integer(e.LineJoin)
	} else {
		if e.LineJoin != 0 {
			return nil, errors.New("unexpected LineJoin value")
		}
	}
	if set&state.MiterLimit != 0 {
		dict["ML"] = pdf.Number(e.MiterLimit)
	} else {
		if e.MiterLimit != 0 {
			return nil, errors.New("unexpected MiterLimit value")
		}
	}
	if set&state.LineDash != 0 {
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
	if set&state.RenderingIntent != 0 {
		dict["RI"] = pdf.Name(e.RenderingIntent)
	} else {
		if e.RenderingIntent != "" {
			return nil, errors.New("unexpected RenderingIntent value")
		}
	}
	if set&state.StrokeAdjustment != 0 {
		dict["SA"] = pdf.Boolean(e.StrokeAdjustment)
	} else {
		if e.StrokeAdjustment {
			return nil, errors.New("unexpected StrokeAdjustment value")
		}
	}
	if set&state.BlendMode != 0 {
		dict["BM"] = e.BlendMode.AsPDF()
	} else {
		if !e.BlendMode.IsZero() {
			return nil, errors.New("unexpected BlendMode value")
		}
	}
	if set&state.SoftMask != 0 {
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
	if set&state.StrokeAlpha != 0 {
		dict["CA"] = pdf.Number(e.StrokeAlpha)
	} else {
		if e.StrokeAlpha != 0 {
			return nil, errors.New("unexpected StrokeAlpha value")
		}
	}
	if set&state.FillAlpha != 0 {
		dict["ca"] = pdf.Number(e.FillAlpha)
	} else {
		if e.FillAlpha != 0 {
			return nil, errors.New("unexpected FillAlpha value")
		}
	}
	if set&state.AlphaSourceFlag != 0 {
		dict["AIS"] = pdf.Boolean(e.AlphaSourceFlag)
	} else {
		if e.AlphaSourceFlag {
			return nil, errors.New("unexpected AlphaSourceFlag value")
		}
	}
	if set&state.BlackPointCompensation != 0 {
		dict["UseBlackPtComp"] = e.BlackPointCompensation
	} else {
		if e.BlackPointCompensation != "" {
			return nil, errors.New("unexpected BlackPointCompensation value")
		}
	}

	if set&state.Overprint != 0 {
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
	if set&state.OverprintMode != 0 {
		dict["OPM"] = pdf.Integer(e.OverprintMode)
	} else {
		if e.OverprintMode != 0 {
			return nil, errors.New("unexpected OverprintMode value")
		}
	}
	if set&state.BlackGeneration != 0 {
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
	if set&state.UndercolorRemoval != 0 {
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
	if set&state.TransferFunction != 0 {
		if v := pdf.GetVersion(rm.Out()); v >= pdf.V2_0 {
			return nil, errors.New("TransferFunction is deprecated in PDF 2.0")
		}
		all := []pdf.Function{
			e.TransferFunction.Red,
			e.TransferFunction.Green,
			e.TransferFunction.Blue,
			e.TransferFunction.Gray,
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
			case transfer.Identity:
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
		if e.TransferFunction.Red != nil || e.TransferFunction.Green != nil ||
			e.TransferFunction.Blue != nil || e.TransferFunction.Gray != nil {
			return nil, errors.New("unexpected TransferFunction value")
		}
	}
	if set&state.Halftone != 0 {
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
	if set&state.HalftoneOrigin != 0 {
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
	if set&state.FlatnessTolerance != 0 {
		dict["FL"] = pdf.Number(e.FlatnessTolerance)
	} else {
		if e.FlatnessTolerance != 0 {
			return nil, errors.New("unexpected FlatnessTolerance value")
		}
	}
	if set&state.SmoothnessTolerance != 0 {
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
func (e *ExtGState) ApplyTo(s *State) {
	set := e.Set
	s.Set |= set

	param := s.Parameters
	if set&state.TextFont != 0 {
		param.TextFont = e.TextFont
		param.TextFontSize = e.TextFontSize
	}
	if set&state.TextKnockout != 0 {
		param.TextKnockout = e.TextKnockout
	}
	if set&state.LineWidth != 0 {
		param.LineWidth = e.LineWidth
	}
	if set&state.LineCap != 0 {
		param.LineCap = e.LineCap
	}
	if set&state.LineJoin != 0 {
		param.LineJoin = e.LineJoin
	}
	if set&state.MiterLimit != 0 {
		param.MiterLimit = e.MiterLimit
	}
	if set&state.LineDash != 0 {
		param.DashPattern = slices.Clone(e.DashPattern)
		param.DashPhase = e.DashPhase
	}
	if set&state.RenderingIntent != 0 {
		param.RenderingIntent = e.RenderingIntent
	}
	if set&state.StrokeAdjustment != 0 {
		param.StrokeAdjustment = e.StrokeAdjustment
	}
	if set&state.BlendMode != 0 {
		param.BlendMode = e.BlendMode
	}
	if set&state.SoftMask != 0 {
		param.SoftMask = e.SoftMask
	}
	if set&state.StrokeAlpha != 0 {
		param.StrokeAlpha = e.StrokeAlpha
	}
	if set&state.FillAlpha != 0 {
		param.FillAlpha = e.FillAlpha
	}
	if set&state.AlphaSourceFlag != 0 {
		param.AlphaSourceFlag = e.AlphaSourceFlag
	}
	if set&state.BlackPointCompensation != 0 {
		param.BlackPointCompensation = e.BlackPointCompensation
	}
	if set&state.Overprint != 0 {
		param.OverprintStroke = e.OverprintStroke
		param.OverprintFill = e.OverprintFill
	}
	if set&state.OverprintMode != 0 {
		param.OverprintMode = e.OverprintMode
	}
	if set&state.BlackGeneration != 0 {
		param.BlackGeneration = e.BlackGeneration
	}
	if set&state.UndercolorRemoval != 0 {
		param.UndercolorRemoval = e.UndercolorRemoval
	}
	if set&state.TransferFunction != 0 {
		param.TransferFunction = e.TransferFunction
	}
	if set&state.Halftone != 0 {
		param.Halftone = e.Halftone
	}
	if set&state.HalftoneOrigin != 0 {
		param.HalftoneOriginX = e.HalftoneOriginX
		param.HalftoneOriginY = e.HalftoneOriginY
	}
	if set&state.FlatnessTolerance != 0 {
		param.FlatnessTolerance = e.FlatnessTolerance
	}
	if set&state.SmoothnessTolerance != 0 {
		param.SmoothnessTolerance = e.SmoothnessTolerance
	}
}
