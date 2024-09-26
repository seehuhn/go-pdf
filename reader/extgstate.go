// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package reader

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// readExtGState reads an graphics state parameter dictionary from a PDF file.
func (r *Reader) readExtGState(ref pdf.Object) (graphics.State, error) {
	var zero graphics.State

	dict, err := pdf.GetDictTyped(r.R, ref, "ExtGState")
	if err != nil {
		return zero, err
	}

	param := &graphics.Parameters{}
	var set graphics.StateBits
	var overprintFillSet bool
	var bg1, bg2 pdf.Object
	var ucr1, ucr2 pdf.Object
	var tr1, tr2 pdf.Object
	for key, v := range dict {
		switch key {
		case "Font":
			a, err := pdf.GetArray(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			if len(a) != 2 {
				break
			}

			size, err := pdf.GetNumber(r.R, a[1])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}

			F, err := r.ReadFont(a[0])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}

			param.TextFont = F
			param.TextFontSize = float64(size)
			set |= graphics.StateTextFont
		case "TK":
			val, err := pdf.GetBoolean(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			param.TextKnockout = bool(val)
			set |= graphics.StateTextKnockout
		case "LW":
			lw, err := pdf.GetNumber(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			param.LineWidth = float64(lw)
			set |= graphics.StateLineWidth
		case "LC":
			lineCap, err := pdf.GetInteger(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			if lineCap < 0 {
				lineCap = 0
			} else if lineCap > 2 {
				lineCap = 2
			}
			param.LineCap = graphics.LineCapStyle(lineCap)
			set |= graphics.StateLineCap
		case "LJ":
			lineJoin, err := pdf.GetInteger(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			if lineJoin < 0 {
				lineJoin = 0
			} else if lineJoin > 2 {
				lineJoin = 2
			}
			param.LineJoin = graphics.LineJoinStyle(lineJoin)
			set |= graphics.StateLineJoin
		case "ML":
			miterLimit, err := pdf.GetNumber(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			if miterLimit < 1 {
				miterLimit = 1
			}
			param.MiterLimit = float64(miterLimit)
			set |= graphics.StateMiterLimit
		case "D":
			dashPattern, phase, err := readDash(r.R, v)
			if err != nil {
				return zero, err
			} else if dashPattern != nil {
				param.DashPattern = dashPattern
				param.DashPhase = phase
				set |= graphics.StateLineDash
			}
		case "RI":
			ri, err := pdf.GetName(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			param.RenderingIntent = graphics.RenderingIntent(ri)
			set |= graphics.StateRenderingIntent
		case "SA":
			val, err := pdf.GetBoolean(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			param.StrokeAdjustment = bool(val)
			set |= graphics.StateStrokeAdjustment
		case "BM":
			param.BlendMode = v
			set |= graphics.StateBlendMode
		case "SMask":
			param.SoftMask = v
			set |= graphics.StateSoftMask
		case "CA":
			ca, err := pdf.GetNumber(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			param.StrokeAlpha = float64(ca)
			set |= graphics.StateStrokeAlpha
		case "ca":
			ca, err := pdf.GetNumber(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			param.FillAlpha = float64(ca)
			set |= graphics.StateFillAlpha
		case "AIS":
			ais, err := pdf.GetBoolean(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			param.AlphaSourceFlag = bool(ais)
			set |= graphics.StateAlphaSourceFlag
		case "UseBlackPtComp":
			val, err := pdf.GetName(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			param.BlackPointCompensation = val
			set |= graphics.StateBlackPointCompensation
		case "OP":
			op, err := pdf.GetBoolean(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			param.OverprintStroke = bool(op)
			set |= graphics.StateOverprint
		case "op":
			op, err := pdf.GetBoolean(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			param.OverprintFill = bool(op)
			set |= graphics.StateOverprint
			overprintFillSet = true
		case "OPM":
			opm, err := pdf.GetInteger(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			if opm != 0 {
				param.OverprintMode = 1
			}
			set |= graphics.StateOverprintMode
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
			set |= graphics.StateHalftone
		case "HTO":
			a, err := pdf.GetArray(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			if len(a) != 2 {
				break
			}
			x, err := pdf.GetNumber(r.R, a[0])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			y, err := pdf.GetNumber(r.R, a[1])
			if pdf.IsMalformed(err) {
				break
			}
			param.HalftoneOriginX = float64(x)
			param.HalftoneOriginY = float64(y)
			set |= graphics.StateHalftoneOrigin
		case "FL":
			fl, err := pdf.GetNumber(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			param.FlatnessTolerance = float64(fl)
			set |= graphics.StateFlatnessTolerance
		case "SM":
			sm, err := pdf.GetNumber(r.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return zero, err
			}
			param.SmoothnessTolerance = float64(sm)
			set |= graphics.StateSmoothnessTolerance
		}
	}

	if set&graphics.StateOverprint != 0 && !overprintFillSet {
		param.OverprintFill = param.OverprintStroke
	}
	if bg2 != nil {
		param.BlackGeneration = bg2
		set |= graphics.StateBlackGeneration
	} else if bg1 != nil {
		param.BlackGeneration = bg1
		set |= graphics.StateBlackGeneration
	}
	if ucr2 != nil {
		param.UndercolorRemoval = ucr2
		set |= graphics.StateUndercolorRemoval
	} else if ucr1 != nil {
		param.UndercolorRemoval = ucr1
		set |= graphics.StateUndercolorRemoval
	}
	if tr2 != nil {
		param.TransferFunction = tr2
		set |= graphics.StateTransferFunction
	} else if tr1 != nil {
		param.TransferFunction = tr1
		set |= graphics.StateTransferFunction
	}

	res := graphics.State{
		Parameters: param,
		Set:        set,
	}
	return res, nil
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
