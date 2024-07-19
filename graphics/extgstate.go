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

	"seehuhn.de/go/pdf"
)

// ExtGState represents a combination of graphics state parameters.
// This combination of parameters can be set using the [Writer.SetExtGState] method.
//
// Note that not all graphics parameters can be set using the ExtGState object.
// In particular, the stroke and fill color and some text parameters cannot be
// included in the ExtGState object.
type ExtGState struct {
	Value     State
	SingleUse bool
}

// NewExtGState creates a new ExtGState object.
//
// If s contains values for parameters that cannot be included in an ExtGState
// object, an error is returned.
func NewExtGState(s State) (*ExtGState, error) {
	set := s.Set
	if set & ^ExtGStateBits != 0 {
		return nil, errors.New("invalid states for ExtGState")
	}

	return &ExtGState{
		Value: State{
			Parameters: s.Parameters.Clone(),
			Set:        set,
		},
	}, nil
}

// Embed adds the graphics state dictionary to a PDF file.
//
// This implements the [pdf.Embedder] interface.
func (s *ExtGState) Embed(rm *pdf.ResourceManager) (pdf.Resource, error) {
	if err := pdf.CheckVersion(rm.Out, "ExtGState", pdf.V1_2); err != nil {
		return nil, err
	}

	set := s.Value.Set
	param := s.Value.Parameters

	// Build a graphics state parameter dictionary for the given state.
	// See table 57 in ISO 32000-2:2020.
	dict := pdf.Dict{}
	if set&StateTextFont != 0 {
		// TODO(voss): embed the font
		ref := param.TextFont.PDFObject().(pdf.Reference)
		dict["Font"] = pdf.Array{
			ref,
			pdf.Number(param.TextFontSize),
		}
	}
	if set&StateTextKnockout != 0 {
		dict["TK"] = pdf.Boolean(param.TextKnockout)
	}
	if set&StateLineWidth != 0 {
		dict["LW"] = pdf.Number(param.LineWidth)
	}
	if set&StateLineCap != 0 {
		dict["LC"] = pdf.Integer(param.LineCap)
	}
	if set&StateLineJoin != 0 {
		dict["LJ"] = pdf.Integer(param.LineJoin)
	}
	if set&StateMiterLimit != 0 {
		dict["ML"] = pdf.Number(param.MiterLimit)
	}
	if set&StateLineDash != 0 {
		pat := make(pdf.Array, len(param.DashPattern))
		for i, x := range param.DashPattern {
			pat[i] = pdf.Number(x)
		}
		dict["D"] = pdf.Array{
			pat,
			pdf.Number(param.DashPhase),
		}
	}
	if set&StateRenderingIntent != 0 {
		dict["RI"] = pdf.Name(param.RenderingIntent)
	}
	if set&StateStrokeAdjustment != 0 {
		dict["SA"] = pdf.Boolean(param.StrokeAdjustment)
	}
	if set&StateBlendMode != 0 {
		dict["BM"] = param.BlendMode
	}
	if set&StateSoftMask != 0 {
		dict["SMask"] = param.SoftMask
	}
	if set&StateStrokeAlpha != 0 {
		dict["CA"] = pdf.Number(param.StrokeAlpha)
	}
	if set&StateFillAlpha != 0 {
		dict["ca"] = pdf.Number(param.FillAlpha)
	}
	if set&StateAlphaSourceFlag != 0 {
		dict["AIS"] = pdf.Boolean(param.AlphaSourceFlag)
	}
	if set&StateBlackPointCompensation != 0 {
		dict["UseBlackPtComp"] = param.BlackPointCompensation
	}

	if set&StateOverprint != 0 {
		dict["OP"] = pdf.Boolean(param.OverprintStroke)
		if param.OverprintFill != param.OverprintStroke {
			dict["op"] = pdf.Boolean(param.OverprintFill)
		}
	}
	if set&StateOverprintMode != 0 {
		dict["OPM"] = pdf.Integer(param.OverprintMode)
	}
	if set&StateBlackGeneration != 0 {
		if _, isName := param.BlackGeneration.(pdf.Name); isName {
			dict["BG2"] = param.BlackGeneration
		} else {
			dict["BG"] = param.BlackGeneration
		}
	}
	if set&StateUndercolorRemoval != 0 {
		if _, isName := param.UndercolorRemoval.(pdf.Name); isName {
			dict["UCR2"] = param.UndercolorRemoval
		} else {
			dict["UCR"] = param.UndercolorRemoval
		}
	}
	if set&StateTransferFunction != 0 {
		if _, isName := param.TransferFunction.(pdf.Name); isName {
			dict["TR2"] = param.TransferFunction
		} else {
			dict["TR"] = param.TransferFunction
		}
	}
	if set&StateHalftone != 0 {
		dict["HT"] = param.Halftone
	}
	if set&StateHalftoneOrigin != 0 {
		dict["HTO"] = pdf.Array{
			pdf.Number(param.HalftoneOriginX),
			pdf.Number(param.HalftoneOriginY),
		}
	}
	if set&StateFlatnessTolerance != 0 {
		dict["FL"] = pdf.Number(param.FlatnessTolerance)
	}
	if set&StateSmoothnessTolerance != 0 {
		dict["SM"] = pdf.Number(param.SmoothnessTolerance)
	}

	var obj pdf.Object = dict
	if s.SingleUse {
		ref := rm.Out.Alloc()
		err := rm.Out.Put(ref, dict)
		if err != nil {
			return nil, err
		}
		obj = ref
	}

	return pdf.Res{Data: obj}, nil
}
