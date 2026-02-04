// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package extract

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/halftone"
)

// ExtGState extracts an extended graphics state from a PDF file.
func ExtGState(x *pdf.Extractor, obj pdf.Object) (*extgstate.ExtGState, error) {
	singleUse := !x.IsIndirect // capture before other x method calls

	dict, err := x.GetDictTyped(obj, "ExtGState")
	if err != nil {
		return nil, err
	}

	res := &extgstate.ExtGState{}
	var set graphics.Bits
	var overprintFillSet bool
	var bg1, bg2 pdf.Object
	var ucr1, ucr2 pdf.Object
	var tr1, tr2 pdf.Object

	for key, v := range dict {
		switch key {
		case "Font":
			a, err := x.GetArray(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if len(a) != 2 {
				break
			}

			// Validate that first element is a Reference (like Embed method requires)
			fontRef, ok := a[0].(pdf.Reference)
			if !ok {
				break
			}

			size, err := x.GetNumber(a[1])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}

			F, err := Font(x, fontRef)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}

			res.TextFont = F
			res.TextFontSize = size
			set |= graphics.StateTextFont
		case "TK":
			val, err := x.GetBoolean(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.TextKnockout = bool(val)
			set |= graphics.StateTextKnockout
		case "LW":
			lw, err := x.GetNumber(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.LineWidth = lw
			set |= graphics.StateLineWidth
		case "LC":
			lineCap, err := x.GetInteger(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if lineCap < 0 {
				lineCap = 0
			} else if lineCap > 2 {
				lineCap = 2
			}
			res.LineCap = graphics.LineCapStyle(lineCap)
			set |= graphics.StateLineCap
		case "LJ":
			lineJoin, err := x.GetInteger(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if lineJoin < 0 {
				lineJoin = 0
			} else if lineJoin > 2 {
				lineJoin = 2
			}
			res.LineJoin = graphics.LineJoinStyle(lineJoin)
			set |= graphics.StateLineJoin
		case "ML":
			miterLimit, err := x.GetNumber(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if miterLimit < 1 {
				miterLimit = 1
			}
			res.MiterLimit = miterLimit
			set |= graphics.StateMiterLimit
		case "D":
			dashPattern, phase, err := readDash(x.R, v)
			if err != nil {
				return nil, err
			} else if dashPattern != nil {
				res.DashPattern = dashPattern
				res.DashPhase = phase
				set |= graphics.StateLineDash
			}
		case "RI":
			ri, err := x.GetName(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.RenderingIntent = graphics.RenderingIntent(ri)
			set |= graphics.StateRenderingIntent
		case "SA":
			val, err := x.GetBoolean(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.StrokeAdjustment = bool(val)
			set |= graphics.StateStrokeAdjustment
		case "BM":
			bm, err := BlendMode(x, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if !bm.IsZero() {
				res.BlendMode = bm
				set |= graphics.StateBlendMode
			}
		case "SMask":
			sMask, err := pdf.Optional(SoftMaskDict(x, v))
			if err != nil {
				return nil, err
			}
			res.SoftMask = sMask
			set |= graphics.StateSoftMask
		case "CA":
			ca, err := x.GetNumber(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.StrokeAlpha = ca
			set |= graphics.StateStrokeAlpha
		case "ca":
			ca, err := x.GetNumber(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.FillAlpha = ca
			set |= graphics.StateFillAlpha
		case "AIS":
			ais, err := x.GetBoolean(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.AlphaSourceFlag = bool(ais)
			set |= graphics.StateAlphaSourceFlag
		case "UseBlackPtComp":
			val, err := x.GetName(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.BlackPointCompensation = val
			set |= graphics.StateBlackPointCompensation
		case "OP":
			op, err := x.GetBoolean(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.OverprintStroke = bool(op)
			set |= graphics.StateOverprint
		case "op":
			op, err := x.GetBoolean(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.OverprintFill = bool(op)
			set |= graphics.StateOverprint
			overprintFillSet = true
		case "OPM":
			opm, err := x.GetInteger(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if opm != 0 {
				res.OverprintMode = 1
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
			ht, err := pdf.ExtractorGetOptional(x, v, halftone.Extract)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.Halftone = ht
			set |= graphics.StateHalftone
		case "HTO":
			a, err := x.GetArray(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if len(a) != 2 {
				break
			}
			xCoord, err := x.GetNumber(a[0])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			yCoord, err := x.GetNumber(a[1])
			if pdf.IsMalformed(err) {
				break
			}
			res.HalftoneOriginX = xCoord
			res.HalftoneOriginY = yCoord
			set |= graphics.StateHalftoneOrigin
		case "FL":
			fl, err := x.GetNumber(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.FlatnessTolerance = fl
			set |= graphics.StateFlatnessTolerance
		case "SM":
			sm, err := x.GetNumber(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.SmoothnessTolerance = sm
			set |= graphics.StateSmoothnessTolerance
		}
	}

	// Handle overprint fill fallback (like in reader)
	if set&graphics.StateOverprint != 0 && !overprintFillSet {
		res.OverprintFill = res.OverprintStroke
	}

	// Handle BlackGeneration precedence: BG2 > BG
	if bg2 == pdf.Name("Default") {
		res.BlackGeneration = nil
		bg2 = nil
		set |= graphics.StateBlackGeneration
	}
	if set&graphics.StateBlackGeneration == 0 && bg2 != nil {
		fn, err := pdf.ExtractorGet(x, bg2, function.Extract)
		if err == nil {
			if nIn, nOut := fn.Shape(); nIn == 1 && nOut == 1 {
				res.BlackGeneration = fn
				set |= graphics.StateBlackGeneration
			}
		}
	}
	if set&graphics.StateBlackGeneration == 0 && bg1 != nil {
		fn, err := pdf.ExtractorGet(x, bg1, function.Extract)
		if err == nil {
			if nIn, nOut := fn.Shape(); nIn == 1 && nOut == 1 {
				res.BlackGeneration = fn
				set |= graphics.StateBlackGeneration
			}
		}
	}

	// Handle UndercolorRemoval precedence: UCR2 > UCR
	if ucr2 == pdf.Name("Default") {
		res.UndercolorRemoval = nil
		ucr2 = nil
		set |= graphics.StateUndercolorRemoval
	}
	if set&graphics.StateUndercolorRemoval == 0 && ucr2 != nil {
		fn, err := pdf.ExtractorGet(x, ucr2, function.Extract)
		if err == nil {
			if nIn, nOut := fn.Shape(); nIn == 1 && nOut == 1 {
				res.UndercolorRemoval = fn
				set |= graphics.StateUndercolorRemoval
			}
		}
	}
	if set&graphics.StateUndercolorRemoval == 0 && ucr1 != nil {
		fn, err := pdf.ExtractorGet(x, ucr1, function.Extract)
		if err == nil {
			if nIn, nOut := fn.Shape(); nIn == 1 && nOut == 1 {
				res.UndercolorRemoval = fn
				set |= graphics.StateUndercolorRemoval
			}
		}
	}

	// Handle TransferFunction precedence: TR2 > TR (deprecated in PDF 2.0)
	if pdf.GetVersion(x.R) < pdf.V2_0 {
		if tr2 != nil {
			fn, err := parseTransferFunction(x, tr2)
			if err != nil {
				return nil, err
			}
			res.TransferFunctions = fn
			set |= graphics.StateTransferFunction
		} else if tr1 != nil {
			fn, err := parseTransferFunction(x, tr1)
			if err != nil {
				return nil, err
			}
			res.TransferFunctions = fn
			set |= graphics.StateTransferFunction
		}
	}

	res.SingleUse = singleUse

	res.Set = set
	return res, nil
}

// BlendMode extracts a blend mode from a PDF object.
// Handles both name and array forms (array deprecated in PDF 2.0).
func BlendMode(x *pdf.Extractor, obj pdf.Object) (graphics.BlendMode, error) {
	obj, err := pdf.Resolve(x.R, obj)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}

	switch v := obj.(type) {
	case pdf.Name:
		return graphics.BlendMode{v}, nil
	case pdf.Array:
		result := make(graphics.BlendMode, 0, len(v))
		for _, elem := range v {
			name, err := x.GetName(elem)
			if err != nil {
				continue // skip malformed entries
			}
			result = append(result, name)
		}
		if len(result) == 0 {
			return nil, nil
		}
		return result, nil
	default:
		return nil, pdf.Errorf("invalid blend mode type: %T", obj)
	}
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

func parseTransferFunction(x *pdf.Extractor, obj pdf.Object) (graphics.TransferFunctions, error) {
	var zero graphics.TransferFunctions

	// check if it's an array of four or more transfer functions
	if arr, err := x.GetArray(obj); err == nil && len(arr) >= 4 {
		var result graphics.TransferFunctions

		// parse Red component
		fn, err := parseSingleTransfer(x, arr[0])
		if err != nil {
			return zero, err
		}
		result.Red = fn

		// parse Green component
		fn, err = parseSingleTransfer(x, arr[1])
		if err != nil {
			return zero, err
		}
		result.Green = fn

		// parse Blue component
		fn, err = parseSingleTransfer(x, arr[2])
		if err != nil {
			return zero, err
		}
		result.Blue = fn

		// parse Gray component
		fn, err = parseSingleTransfer(x, arr[3])
		if err != nil {
			return zero, err
		}
		result.Gray = fn

		return result, nil
	}

	// single transfer function - apply to all components
	fn, err := parseSingleTransfer(x, obj)
	if err != nil {
		return zero, err
	}
	return graphics.TransferFunctions{
		Red:   fn,
		Green: fn,
		Blue:  fn,
		Gray:  fn,
	}, nil
}

func parseSingleTransfer(x *pdf.Extractor, obj pdf.Object) (pdf.Function, error) {
	if name, isName := obj.(pdf.Name); isName {
		switch name {
		case "Identity":
			return function.Identity, nil
		default:
			// treat all other names (including "Default") as the default
			// transfer function.
			return nil, nil
		}
	}

	fn, err := pdf.ExtractorGet(x, obj, function.Extract)
	if err != nil {
		return nil, err
	}

	if nIn, nOut := fn.Shape(); nIn != 1 || nOut != 1 {
		return nil, pdf.Errorf("wrong transfer function shape (%d,%d) != (1,1)", nIn, nOut)
	}

	return fn, nil
}
