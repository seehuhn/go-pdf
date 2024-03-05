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
	pdf.Res
	Value State
}

// NewExtGState creates a new ExtGState object.
//
// If s contains values for parameters that cannot be included in an ExtGState
// object, an error is returned.
func NewExtGState(s State, defaultName pdf.Name) (*ExtGState, error) {
	set := s.Set
	if set & ^ExtGStateBits != 0 {
		return nil, errors.New("invalid states for ExtGState")
	}

	// Build a graphics state parameter dictionary for the given state.
	// See table 57 in ISO 32000-2:2020.
	dict := pdf.Dict{}
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
	if set&StateLineDash != 0 {
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
		dict["RI"] = pdf.Name(s.RenderingIntent)
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
		Res: pdf.Res{
			DefName: pdf.Name(defaultName),
			Data:    dict,
		},
		Value: State{
			Parameters: s.Parameters.Clone(),
			Set:        set,
		},
	}, nil
}

// Embed writes the graphics state dictionary into the PDF file so that the
// graphics state can refer to it by reference.
// This can reduce the PDF file size if the graphics state is shared between
// multiple content streams.
func (s *ExtGState) Embed(w pdf.Putter) (*ExtGState, error) {
	if _, alreadyDone := s.Res.Data.(pdf.Reference); alreadyDone {
		return s, nil
	}

	ref := w.Alloc()
	err := w.Put(ref, s.Res.Data)
	if err != nil {
		return nil, err
	}

	res := &ExtGState{
		Res:   pdf.Res{DefName: s.DefName, Data: ref},
		Value: s.Value,
	}
	return res, nil
}
