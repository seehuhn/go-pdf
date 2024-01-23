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
)

// SetExtGState sets selected graphics state parameters.
//
// This implements the "gs" graphics operator.
func (p *Writer) SetExtGState(s *ExtGState) {
	if !p.isValid("SetExtGState", objPage|objText) {
		return
	}

	s.ApplyTo(&p.State)

	name := p.getResourceName(catExtGState, s)
	err := name.PDF(p.Content)
	if err != nil {
		p.Err = err
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, " gs")
}

// ExtGState represents a combination of graphics state parameters.
// This combination of parameters can be set using the [Page.SetExtGState] method.
type ExtGState struct {
	DefName pdf.Name   // leave empty to generate new names automatically
	Dict    pdf.Object // [pdf.Dict], can be indirect
	Value   State
}

// NewExtGState creates a new ExtGState object.
func NewExtGState(s State, defaultName string) (*ExtGState, error) {
	set := s.Set
	if set & ^extStateBits != 0 {
		return nil, errors.New("invalid states for ExtGState")
	}

	dict := pdf.Dict{}
	// Build a graphics state parameter dictionary for the given state.
	// See table 57 in ISO 32000-2:2020.
	if set&StateTextFont != 0 {
		ref := s.TextFont.PDFObject().(pdf.Reference)
		dict["Font"] = pdf.Array{
			ref,
			pdf.Number(s.TextFontSize),
		}
	}
	if set&StateTextKnockout != 0 {
		dict["TK"] = pdf.Boolean(s.TextKnockout)
	}
	if set&StateLineWidth != 0 {
		dict["LW"] = pdf.Number(s.LineWidth)
	}
	if set&StateLineCap != 0 {
		dict["LC"] = pdf.Integer(s.LineCap)
	}
	if set&StateLineJoin != 0 {
		dict["LJ"] = pdf.Integer(s.LineJoin)
	}
	if set&StateMiterLimit != 0 {
		dict["ML"] = pdf.Number(s.MiterLimit)
	}
	if set&StateDash != 0 {
		pat := make(pdf.Array, len(s.DashPattern))
		for i, x := range s.DashPattern {
			pat[i] = pdf.Number(x)
		}
		dict["D"] = pdf.Array{
			pat,
			pdf.Number(s.DashPhase),
		}
	}
	if set&StateRenderingIntent != 0 {
		dict["RI"] = s.RenderingIntent
	}
	if set&StateStrokeAdjustment != 0 {
		dict["SA"] = pdf.Boolean(s.StrokeAdjustment)
	}
	if set&StateBlendMode != 0 {
		dict["BM"] = s.BlendMode
	}
	if set&StateSoftMask != 0 {
		dict["SMask"] = s.SoftMask
	}
	if set&StateStrokeAlpha != 0 {
		dict["CA"] = pdf.Number(s.StrokeAlpha)
	}
	if set&StateFillAlpha != 0 {
		dict["ca"] = pdf.Number(s.FillAlpha)
	}
	if set&StateAlphaSourceFlag != 0 {
		dict["AIS"] = pdf.Boolean(s.AlphaSourceFlag)
	}
	if set&StateBlackPointCompensation != 0 {
		dict["UseBlackPtComp"] = s.BlackPointCompensation
	}

	if set&StateOverprint != 0 {
		dict["OP"] = pdf.Boolean(s.OverprintStroke)
		if s.OverprintFill != s.OverprintStroke {
			dict["op"] = pdf.Boolean(s.OverprintFill)
		}
	}
	if set&StateOverprintMode != 0 {
		dict["OPM"] = pdf.Integer(s.OverprintMode)
	}
	if set&StateBlackGeneration != 0 {
		if _, isName := s.BlackGeneration.(pdf.Name); isName {
			dict["BG2"] = s.BlackGeneration
		} else {
			dict["BG"] = s.BlackGeneration
		}
	}
	if set&StateUndercolorRemoval != 0 {
		if _, isName := s.UndercolorRemoval.(pdf.Name); isName {
			dict["UCR2"] = s.UndercolorRemoval
		} else {
			dict["UCR"] = s.UndercolorRemoval
		}
	}
	if set&StateTransferFunction != 0 {
		if _, isName := s.TransferFunction.(pdf.Name); isName {
			dict["TR2"] = s.TransferFunction
		} else {
			dict["TR"] = s.TransferFunction
		}
	}
	if set&StateHalftone != 0 {
		dict["HT"] = s.Halftone
	}
	if set&StateHalftoneOrigin != 0 {
		dict["HTO"] = pdf.Array{
			pdf.Number(s.HalftoneOriginX),
			pdf.Number(s.HalftoneOriginY),
		}
	}
	if set&StateFlatnessTolerance != 0 {
		dict["FL"] = pdf.Number(s.FlatnessTolerance)
	}
	if set&StateSmoothnessTolerance != 0 {
		dict["SM"] = pdf.Number(s.SmoothnessTolerance)
	}

	return &ExtGState{
		DefName: pdf.Name(defaultName),
		Dict:    dict,
		Value: State{
			Parameters: s.Parameters.Clone(),
			Set:        set,
		},
	}, nil
}

// ApplyTo applies the graphics state parameters to the given state.
//
// TODO(voss): unexport this method?
func (s *ExtGState) ApplyTo(other *State) {
	set := s.Value.Set
	other.Set |= set

	param := s.Value.Parameters
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
	if set&StateDash != 0 {
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

// Embed writes the graphics state dictionary into the PDF file so that the
// graphics state can refer to it by reference.
// This allows for efficient sharing of PDF graphics state dictionaries
// between content streams.
func (s *ExtGState) Embed(w pdf.Putter) (*ExtGState, error) {
	if _, alreadyDone := s.Dict.(pdf.Reference); alreadyDone {
		return s, nil
	}
	ref := w.Alloc()
	err := w.Put(ref, s.Dict)
	if err != nil {
		return nil, err
	}

	res := &ExtGState{
		DefName: s.DefName,
		Dict:    ref,
		Value:   s.Value,
	}
	return res, nil
}

// DefaultName returns the default name for this resource.
func (s *ExtGState) DefaultName() pdf.Name {
	return s.DefName
}

// PDFObject returns the value to use in the PDF Resources dictionary.
// This can either be [pdf.Reference] or [pdf.Dict].
func (s *ExtGState) PDFObject() pdf.Object {
	return s.Dict
}
