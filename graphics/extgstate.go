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
	if !p.valid("SetExtGState", objPage, objText) {
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
	Dict    pdf.Object // either [pdf.Dict] or [pdf.Reference]
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
		// TODO(voss): verify that the font is given as a reference?
		dict["Font"] = pdf.Array{
			s.TextFont.PDFObject(),
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

// ReadExtGState reads an graphics state parameter dictionary from a PDF file.
func ReadExtGState(r pdf.Getter, ref pdf.Object, defaultName pdf.Name) (*ExtGState, error) {
	dict, err := pdf.GetDictTyped(r, ref, "ExtGState")
	if err != nil {
		return nil, err
	}

	param := &Parameters{}
	var set StateBits
	var overprintFillSet bool
	var bg1, bg2 pdf.Object
	var ucr1, ucr2 pdf.Object
	var tr1, tr2 pdf.Object
	for key, v := range dict {
		switch key {
		case "Font":
			a, err := pdf.GetArray(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if len(a) != 2 {
				break
			}
			ref, ok := a[0].(pdf.Reference)
			if !ok {
				break
			}
			size, err := pdf.GetNumber(r, a[1])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.TextFont = Res{Data: ref}
			param.TextFontSize = float64(size)
			set |= StateTextFont
		case "TK":
			val, err := pdf.GetBoolean(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.TextKnockout = bool(val)
			set |= StateTextKnockout
		case "LW":
			lw, err := pdf.GetNumber(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.LineWidth = float64(lw)
			set |= StateLineWidth
		case "LC":
			lc, err := pdf.GetInteger(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.LineCap = LineCapStyle(lc)
			set |= StateLineCap
		case "LJ":
			lj, err := pdf.GetInteger(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.LineJoin = LineJoinStyle(lj)
			set |= StateLineJoin
		case "ML":
			ml, err := pdf.GetNumber(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.MiterLimit = float64(ml)
			set |= StateMiterLimit
		case "D":
			dashPattern, phase, err := readDash(r, v)
			if err != nil {
				return nil, err
			} else if dashPattern != nil {
				param.DashPattern = dashPattern
				param.DashPhase = phase
				set |= StateDash
			}
		case "RI":
			ri, err := pdf.GetName(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.RenderingIntent = ri
			set |= StateRenderingIntent
		case "SA":
			val, err := pdf.GetBoolean(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.StrokeAdjustment = bool(val)
			set |= StateStrokeAdjustment
		case "BM":
			param.BlendMode = v
			set |= StateBlendMode
		case "SMask":
			param.SoftMask = v
			set |= StateSoftMask
		case "CA":
			ca, err := pdf.GetNumber(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.StrokeAlpha = float64(ca)
			set |= StateStrokeAlpha
		case "ca":
			ca, err := pdf.GetNumber(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.FillAlpha = float64(ca)
			set |= StateFillAlpha
		case "AIS":
			ais, err := pdf.GetBoolean(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.AlphaSourceFlag = bool(ais)
			set |= StateAlphaSourceFlag
		case "UseBlackPtComp":
			val, err := pdf.GetName(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.BlackPointCompensation = val
			set |= StateBlackPointCompensation
		case "OP":
			op, err := pdf.GetBoolean(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.OverprintStroke = bool(op)
			set |= StateOverprint
		case "op":
			op, err := pdf.GetBoolean(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.OverprintFill = bool(op)
			set |= StateOverprint
			overprintFillSet = true
		case "OPM":
			opm, err := pdf.GetInteger(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if opm != 0 {
				param.OverprintMode = 1
			}
			set |= StateOverprintMode
		case "BG":
			bg1 = v
		case "BG2":
			bg2 = v
		case "UCR":
			ucr1 = v
		case "UCR2":
			ucr2 = v
		case "TR":
			tr1 = v
		case "TR2":
			tr2 = v
		case "HT":
			param.Halftone = v
			set |= StateHalftone
		case "HTO":
			a, err := pdf.GetArray(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if len(a) != 2 {
				break
			}
			x, err := pdf.GetNumber(r, a[0])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			y, err := pdf.GetNumber(r, a[1])
			if pdf.IsMalformed(err) {
				break
			}
			param.HalftoneOriginX = float64(x)
			param.HalftoneOriginY = float64(y)
			set |= StateHalftoneOrigin
		case "FL":
			fl, err := pdf.GetNumber(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.FlatnessTolerance = float64(fl)
			set |= StateFlatnessTolerance
		case "SM":
			sm, err := pdf.GetNumber(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			param.SmoothnessTolerance = float64(sm)
			set |= StateSmoothnessTolerance
		}
	}

	if set&StateOverprint != 0 && !overprintFillSet {
		param.OverprintFill = param.OverprintStroke
	}
	if bg2 != nil {
		param.BlackGeneration = bg2
		set |= StateBlackGeneration
	} else if bg1 != nil {
		param.BlackGeneration = bg1
		set |= StateBlackGeneration
	}
	if ucr2 != nil {
		param.UndercolorRemoval = ucr2
		set |= StateUndercolorRemoval
	} else if ucr1 != nil {
		param.UndercolorRemoval = ucr1
		set |= StateUndercolorRemoval
	}
	if tr2 != nil {
		param.TransferFunction = tr2
		set |= StateTransferFunction
	} else if tr1 != nil {
		param.TransferFunction = tr1
		set |= StateTransferFunction
	}

	res := &ExtGState{
		DefName: defaultName,
		Dict:    ref,
		Value: State{
			Parameters: param,
			Set:        set,
		},
	}
	return res, nil
}

// ApplyTo applies the graphics state parameters to the given state.
//
// TODO(voss): unexport this method.
func (s *ExtGState) ApplyTo(other *State) {
	set := s.Value.Set
	param := s.Value.Parameters

	other.Set |= set
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

func readDash(r pdf.Getter, obj pdf.Object) (pat []float64, ph float64, err error) {
	defer func() {
		if _, isMalformed := err.(*pdf.MalformedFileError); isMalformed {
			err = nil
		}
	}()

	a, err := pdf.GetArray(r, obj)
	if len(a) != 2 { // either error or malformed
		return nil, 0, err
	}
	dashPattern, err := pdf.GetArray(r, a[0])
	if err != nil {
		return nil, 0, err
	}
	phase, err := pdf.GetNumber(r, a[1])
	if err != nil {
		return nil, 0, err
	}
	pat = make([]float64, len(dashPattern))
	for i, obj := range dashPattern {
		x, err := pdf.GetNumber(r, obj)
		if err != nil {
			return nil, 0, err
		}
		pat[i] = float64(x)
	}
	return pat, float64(phase), nil
}
