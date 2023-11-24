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
)

// ExtGState represents a combination of graphics state parameters.
// This combination of parameters can be set using the [Page.SetExtGState] method.
type ExtGState struct {
	DefName pdf.Name   // leave empty to generate new names automatically
	Dict    pdf.Object // either [pdf.Dict] or [pdf.Reference]
	Value   *State
	Set     StateBits
}

// MakeExtGState creates a new ExtGState object.
func MakeExtGState(s *State, set StateBits, defaultName string) *ExtGState {
	return &ExtGState{
		DefName: pdf.Name(defaultName),
		Dict:    ExtGStateDict(s, set),
		Value:   s,
		Set:     set,
	}
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

// SetExtGState sets selected graphics state parameters.
//
// This implements the "gs" graphics operator.
func (p *Page) SetExtGState(s *ExtGState) {
	if !p.valid("SetExtGState", objPage, objText) {
		return
	}

	p.state.Update(s.Value, s.Set)

	name := p.getResourceName("ExtGState", s)
	err := name.PDF(p.Content)
	if err != nil {
		p.Err = err
		return
	}
	_, p.Err = fmt.Fprintln(p.Content, " gs")
}

// ExtGStateDict returns a graphics state parameter dictionary for the given state.
// See table 57 in ISO 32000-2:2020.
func ExtGStateDict(s *State, set StateBits) pdf.Dict {
	res := pdf.Dict{}
	if set&StateLineWidth != 0 {
		res["LW"] = pdf.Number(s.LineWidth)
	}
	if set&StateLineCap != 0 {
		res["LC"] = pdf.Integer(s.LineCap)
	}
	if set&StateLineJoin != 0 {
		res["LJ"] = pdf.Integer(s.LineJoin)
	}
	if set&StateMiterLimit != 0 {
		res["ML"] = pdf.Number(s.MiterLimit)
	}
	if set&StateDash != 0 {
		pat := make(pdf.Array, len(s.DashPattern))
		for i, x := range s.DashPattern {
			pat[i] = pdf.Number(x)
		}
		res["D"] = pdf.Array{
			pat,
			pdf.Number(s.DashPhase),
		}
	}
	if set&StateRenderingIntent != 0 {
		res["RI"] = s.RenderingIntent
	}
	if set&StateOverprint != 0 {
		res["OP"] = pdf.Boolean(s.OverprintStroke)
		if s.OverprintFill != s.OverprintStroke {
			res["op"] = pdf.Boolean(s.OverprintFill)
		}
	}
	if set&StateOverprintMode != 0 {
		res["OPM"] = pdf.Integer(s.OverprintMode)
	}
	if set&StateFont != 0 {
		res["Font"] = pdf.Array{
			s.Font.PDFObject(),
			pdf.Number(s.FontSize),
		}
	}

	// TODO(voss): black generation
	// TODO(voss): undercolor removal
	// TODO(voss): transfer function
	// TODO(voss): halftone

	if set&StateFlatnessTolerance != 0 {
		res["FL"] = pdf.Number(s.FlatnessTolerance)
	}
	if set&StateSmoothnessTolerance != 0 {
		res["SM"] = pdf.Number(s.SmoothnessTolerance)
	}
	if set&StateStrokeAdjustment != 0 {
		res["SA"] = pdf.Boolean(s.StrokeAdjustment)
	}
	if set&StateBlendMode != 0 {
		res["BM"] = s.BlendMode
	}
	if set&StateSoftMask != 0 {
		res["SMask"] = s.SoftMask
	}
	if set&StateStrokeAlpha != 0 {
		res["CA"] = pdf.Number(s.StrokeAlpha)
	}
	if set&StateFillAlpha != 0 {
		res["ca"] = pdf.Number(s.FillAlpha)
	}
	if set&StateAlphaSourceFlag != 0 {
		res["AIS"] = pdf.Boolean(s.AlphaSourceFlag)
	}
	if set&StateTextKnockout != 0 {
		res["TK"] = pdf.Boolean(s.TextKnockout)
	}
	if set&StateBlackPointCompensation != 0 {
		res["UseBlackPtComp"] = s.BlackPointCompensation
	}
	// TODO(voss): HTO

	return res
}

// ReadDict reads an graphics state parameter dictionary from a PDF file.
func ReadDict(r pdf.Getter, ref pdf.Object) (*State, StateBits, error) {
	dict, err := pdf.GetDictTyped(r, ref, "ExtGState")
	if err != nil {
		return nil, 0, err
	}

	s := &State{}
	var set StateBits
	var overprintFillSet bool
	for key, v := range dict {
		switch key {
		case "LW":
			lw, err := pdf.GetNumber(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, 0, err
			}
			s.LineWidth = float64(lw)
			set |= StateLineWidth
		case "LC":
			lc, err := pdf.GetInteger(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, 0, err
			}
			s.LineCap = LineCapStyle(lc)
			set |= StateLineCap
		case "LJ":
			lj, err := pdf.GetInteger(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, 0, err
			}
			s.LineJoin = LineJoinStyle(lj)
			set |= StateLineJoin
		case "ML":
			ml, err := pdf.GetNumber(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, 0, err
			}
			s.MiterLimit = float64(ml)
			set |= StateMiterLimit
		case "D":
			dashPattern, phase, err := readDash(r, v)
			if err != nil {
				return nil, 0, err
			} else if dashPattern != nil {
				s.DashPattern = dashPattern
				s.DashPhase = phase
				set |= StateDash
			}
		case "RI":
			ri, err := pdf.GetName(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, 0, err
			}
			s.RenderingIntent = ri
			set |= StateRenderingIntent
		case "OP":
			op, err := pdf.GetBoolean(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, 0, err
			}
			s.OverprintStroke = bool(op)
			set |= StateOverprint
		case "op":
			op, err := pdf.GetBoolean(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, 0, err
			}
			s.OverprintFill = bool(op)
			set |= StateOverprint
			overprintFillSet = true
		case "OPM":
			opm, err := pdf.GetInteger(r, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, 0, err
			}
			if opm != 0 {
				s.OverprintMode = 1
			}
			set |= StateOverprintMode
		case "Font":
			panic("not implemented")
		}
	}
	if set&StateOverprint != 0 && !overprintFillSet {
		s.OverprintFill = s.OverprintStroke
	}
	return s, set, nil
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
	pat = make([]float64, len(pat))
	for i, obj := range dashPattern {
		x, err := pdf.GetNumber(r, obj)
		if err != nil {
			return nil, 0, err
		}
		pat[i] = float64(x)
	}
	return pat, float64(phase), nil
}
