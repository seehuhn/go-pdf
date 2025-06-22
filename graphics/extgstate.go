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
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// ExtGState represents a combination of graphics state parameters. This
// combination of parameters can then be set in a single command, using the
// [Writer.SetExtGState] method.  The parameters here form a subset of the
// parameters in the [Parameters] struct.  Parameters not present in the
// ExtGState struct, for example colors, cannot be controlled using an extended
// graphics state.
type ExtGState struct {
	Set StateBits

	TextFont               font.Font
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
	BlendMode              pdf.Object
	SoftMask               pdf.Object
	StrokeAlpha            float64
	FillAlpha              float64
	AlphaSourceFlag        bool
	BlackPointCompensation pdf.Name
	OverprintStroke        bool
	OverprintFill          bool       // for PDF<1.3 this must equal OverprintStroke
	OverprintMode          int        // for PDF<1.3 this must be 0
	BlackGeneration        pdf.Object // TODO(voss): use pdf.Function
	UndercolorRemoval      pdf.Object // TODO(voss): use pdf.Function
	TransferFunction       pdf.Object // TODO(voss): use pdf.Function
	Halftone               Halftone
	HalftoneOriginX        float64
	HalftoneOriginY        float64
	FlatnessTolerance      float64
	SmoothnessTolerance    float64

	// SingleUse can be set if the extended graphics state is used only in a
	// single content stream, in order to slightly reduce output file size.  In
	// this case, the graphics state is embedded in the corresponding resource
	// dictionary, instead of being stored as an indirect object.
	SingleUse bool
}

// Embed adds the graphics state dictionary to a PDF file.
//
// This implements the [pdf.Embedder] interface.
func (s *ExtGState) Embed(rm *pdf.ResourceManager) (pdf.Native, State, error) {
	res := State{
		Parameters: &Parameters{},
	}

	if err := pdf.CheckVersion(rm.Out, "ExtGState", pdf.V1_2); err != nil {
		return nil, res, err
	}

	set := s.Set

	// Build a graphics state parameter dictionary for the given state.
	// See table 57 in ISO 32000-2:2020.
	dict := pdf.Dict{}
	if set&StateTextFont != 0 {
		E, embedded, err := pdf.ResourceManagerEmbed(rm, s.TextFont)
		if err != nil {
			return nil, res, err
		}
		if _, ok := E.(pdf.Reference); !ok {
			err := fmt.Errorf("font %q cannot be used in ExtGState",
				s.TextFont.PostScriptName())
			return nil, res, err
		}
		dict["Font"] = pdf.Array{E, pdf.Number(s.TextFontSize)}
		res.TextFont = embedded
		res.TextFontSize = s.TextFontSize
	}
	if set&StateTextKnockout != 0 {
		dict["TK"] = pdf.Boolean(s.TextKnockout)
		res.TextKnockout = s.TextKnockout
	}
	if set&StateLineWidth != 0 {
		dict["LW"] = pdf.Number(s.LineWidth)
		res.LineWidth = s.LineWidth
	}
	if set&StateLineCap != 0 {
		dict["LC"] = pdf.Integer(s.LineCap)
		res.LineCap = s.LineCap
	}
	if set&StateLineJoin != 0 {
		dict["LJ"] = pdf.Integer(s.LineJoin)
		res.LineJoin = s.LineJoin
	}
	if set&StateMiterLimit != 0 {
		dict["ML"] = pdf.Number(s.MiterLimit)
		res.MiterLimit = s.MiterLimit
	}
	if set&StateLineDash != 0 {
		pat := make(pdf.Array, len(s.DashPattern))
		for i, x := range s.DashPattern {
			pat[i] = pdf.Number(x)
		}
		dict["D"] = pdf.Array{
			pat,
			pdf.Number(s.DashPhase),
		}
		res.DashPattern = s.DashPattern
		res.DashPhase = s.DashPhase
	}
	if set&StateRenderingIntent != 0 {
		dict["RI"] = pdf.Name(s.RenderingIntent)
		res.RenderingIntent = s.RenderingIntent
	}
	if set&StateStrokeAdjustment != 0 {
		dict["SA"] = pdf.Boolean(s.StrokeAdjustment)
		res.StrokeAdjustment = s.StrokeAdjustment
	}
	if set&StateBlendMode != 0 {
		dict["BM"] = s.BlendMode
		res.BlendMode = s.BlendMode
	}
	if set&StateSoftMask != 0 {
		dict["SMask"] = s.SoftMask
		res.SoftMask = s.SoftMask
	}
	if set&StateStrokeAlpha != 0 {
		dict["CA"] = pdf.Number(s.StrokeAlpha)
		res.StrokeAlpha = s.StrokeAlpha
	}
	if set&StateFillAlpha != 0 {
		dict["ca"] = pdf.Number(s.FillAlpha)
		res.FillAlpha = s.FillAlpha
	}
	if set&StateAlphaSourceFlag != 0 {
		dict["AIS"] = pdf.Boolean(s.AlphaSourceFlag)
		res.AlphaSourceFlag = s.AlphaSourceFlag
	}
	if set&StateBlackPointCompensation != 0 {
		dict["UseBlackPtComp"] = s.BlackPointCompensation
		res.BlackPointCompensation = s.BlackPointCompensation
	}

	if set&StateOverprint != 0 {
		dict["OP"] = pdf.Boolean(s.OverprintStroke)
		if s.OverprintFill != s.OverprintStroke {
			dict["op"] = pdf.Boolean(s.OverprintFill)
		}
		res.OverprintStroke = s.OverprintStroke
		res.OverprintFill = s.OverprintFill
	}
	if set&StateOverprintMode != 0 {
		dict["OPM"] = pdf.Integer(s.OverprintMode)
		res.OverprintMode = s.OverprintMode
	}
	if set&StateBlackGeneration != 0 {
		if _, isName := s.BlackGeneration.(pdf.Name); isName {
			dict["BG2"] = s.BlackGeneration
		} else {
			dict["BG"] = s.BlackGeneration
		}
		res.BlackGeneration = s.BlackGeneration
	}
	if set&StateUndercolorRemoval != 0 {
		if _, isName := s.UndercolorRemoval.(pdf.Name); isName {
			dict["UCR2"] = s.UndercolorRemoval
		} else {
			dict["UCR"] = s.UndercolorRemoval
		}
		res.UndercolorRemoval = s.UndercolorRemoval
	}
	if set&StateTransferFunction != 0 {
		if _, isName := s.TransferFunction.(pdf.Name); isName {
			dict["TR2"] = s.TransferFunction
		} else {
			dict["TR"] = s.TransferFunction
		}
		res.TransferFunction = s.TransferFunction
	}
	if set&StateHalftone != 0 {
		htEmbedded, _, err := pdf.ResourceManagerEmbed(rm, s.Halftone)
		if err != nil {
			return nil, res, err
		}
		dict["HT"] = htEmbedded
		res.Halftone = s.Halftone
	}
	if set&StateHalftoneOrigin != 0 {
		dict["HTO"] = pdf.Array{
			pdf.Number(s.HalftoneOriginX),
			pdf.Number(s.HalftoneOriginY),
		}
		res.HalftoneOriginX = s.HalftoneOriginX
		res.HalftoneOriginY = s.HalftoneOriginY
	}
	if set&StateFlatnessTolerance != 0 {
		dict["FL"] = pdf.Number(s.FlatnessTolerance)
		res.FlatnessTolerance = s.FlatnessTolerance
	}
	if set&StateSmoothnessTolerance != 0 {
		dict["SM"] = pdf.Number(s.SmoothnessTolerance)
		res.SmoothnessTolerance = s.SmoothnessTolerance
	}

	res.Set = set & ExtGStateBits

	if s.SingleUse {
		return dict, res, nil
	}
	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, dict)
	if err != nil {
		return nil, res, err
	}
	return ref, res, nil
}
