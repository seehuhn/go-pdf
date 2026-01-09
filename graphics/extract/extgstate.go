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
	"seehuhn.de/go/pdf/graphics/blend"
	"seehuhn.de/go/pdf/graphics/halftone"
	"seehuhn.de/go/pdf/graphics/state"
	"seehuhn.de/go/pdf/graphics/transfer"
)

// ExtGState extracts an extended graphics state from a PDF file.
func ExtGState(x *pdf.Extractor, obj pdf.Object) (*graphics.ExtGState, error) {
	_, isIndirect := obj.(pdf.Reference)

	dict, err := x.GetDictTyped(obj, "ExtGState")
	if err != nil {
		return nil, err
	}

	res := &graphics.ExtGState{}
	var set state.Bits
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
			set |= state.TextFont
		case "TK":
			val, err := x.GetBoolean(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.TextKnockout = bool(val)
			set |= state.TextKnockout
		case "LW":
			lw, err := x.GetNumber(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.LineWidth = lw
			set |= state.LineWidth
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
			set |= state.LineCap
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
			set |= state.LineJoin
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
			set |= state.MiterLimit
		case "D":
			dashPattern, phase, err := readDash(x.R, v)
			if err != nil {
				return nil, err
			} else if dashPattern != nil {
				res.DashPattern = dashPattern
				res.DashPhase = phase
				set |= state.LineDash
			}
		case "RI":
			ri, err := x.GetName(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.RenderingIntent = graphics.RenderingIntent(ri)
			set |= state.RenderingIntent
		case "SA":
			val, err := x.GetBoolean(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.StrokeAdjustment = bool(val)
			set |= state.StrokeAdjustment
		case "BM":
			bm, err := blend.Extract(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if !bm.IsZero() {
				res.BlendMode = bm
				set |= state.BlendMode
			}
		case "SMask":
			sMask, err := pdf.Optional(SoftMaskDict(x, v))
			if err != nil {
				return nil, err
			}
			res.SoftMask = sMask
			set |= state.SoftMask
		case "CA":
			ca, err := x.GetNumber(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.StrokeAlpha = ca
			set |= state.StrokeAlpha
		case "ca":
			ca, err := x.GetNumber(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.FillAlpha = ca
			set |= state.FillAlpha
		case "AIS":
			ais, err := x.GetBoolean(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.AlphaSourceFlag = bool(ais)
			set |= state.AlphaSourceFlag
		case "UseBlackPtComp":
			val, err := x.GetName(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.BlackPointCompensation = val
			set |= state.BlackPointCompensation
		case "OP":
			op, err := x.GetBoolean(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.OverprintStroke = bool(op)
			set |= state.Overprint
		case "op":
			op, err := x.GetBoolean(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.OverprintFill = bool(op)
			set |= state.Overprint
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
			set |= state.OverprintMode
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
			ht, err := halftone.Extract(x, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.Halftone = ht
			set |= state.Halftone
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
			set |= state.HalftoneOrigin
		case "FL":
			fl, err := x.GetNumber(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.FlatnessTolerance = fl
			set |= state.FlatnessTolerance
		case "SM":
			sm, err := x.GetNumber(v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.SmoothnessTolerance = sm
			set |= state.SmoothnessTolerance
		}
	}

	// Handle overprint fill fallback (like in reader)
	if set&state.Overprint != 0 && !overprintFillSet {
		res.OverprintFill = res.OverprintStroke
	}

	// Handle BlackGeneration precedence: BG2 > BG
	if bg2 == pdf.Name("Default") {
		res.BlackGeneration = nil
		bg2 = nil
		set |= state.BlackGeneration
	}
	if set&state.BlackGeneration == 0 && bg2 != nil {
		fn, err := function.Extract(x, bg2)
		if err == nil {
			if nIn, nOut := fn.Shape(); nIn == 1 && nOut == 1 {
				res.BlackGeneration = fn
				set |= state.BlackGeneration
			}
		}
	}
	if set&state.BlackGeneration == 0 && bg1 != nil {
		fn, err := function.Extract(x, bg1)
		if err == nil {
			if nIn, nOut := fn.Shape(); nIn == 1 && nOut == 1 {
				res.BlackGeneration = fn
				set |= state.BlackGeneration
			}
		}
	}

	// Handle UndercolorRemoval precedence: UCR2 > UCR
	if ucr2 == pdf.Name("Default") {
		res.UndercolorRemoval = nil
		ucr2 = nil
		set |= state.UndercolorRemoval
	}
	if set&state.UndercolorRemoval == 0 && ucr2 != nil {
		fn, err := function.Extract(x, ucr2)
		if err == nil {
			if nIn, nOut := fn.Shape(); nIn == 1 && nOut == 1 {
				res.UndercolorRemoval = fn
				set |= state.UndercolorRemoval
			}
		}
	}
	if set&state.UndercolorRemoval == 0 && ucr1 != nil {
		fn, err := function.Extract(x, ucr1)
		if err == nil {
			if nIn, nOut := fn.Shape(); nIn == 1 && nOut == 1 {
				res.UndercolorRemoval = fn
				set |= state.UndercolorRemoval
			}
		}
	}

	// Handle TransferFunction precedence: TR2 > TR (deprecated in PDF 2.0)
	if pdf.GetVersion(x.R) < pdf.V2_0 {
		if tr2 != nil {
			fn, err := parseTransferFunction(x.R, tr2)
			if err != nil {
				return nil, err
			}
			res.TransferFunction = fn
			set |= state.TransferFunction
		} else if tr1 != nil {
			fn, err := parseTransferFunction(x.R, tr1)
			if err != nil {
				return nil, err
			}
			res.TransferFunction = fn
			set |= state.TransferFunction
		}
	}

	res.SingleUse = !isIndirect

	res.Set = set
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

func parseTransferFunction(r pdf.Getter, obj pdf.Object) (transfer.Functions, error) {
	var zero transfer.Functions

	x := pdf.NewExtractor(r)

	// check if it's an array of four or more transfer functions
	if arr, err := pdf.GetArray(r, obj); err == nil && len(arr) >= 4 {
		var result transfer.Functions

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
	return transfer.Functions{
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
			return transfer.Identity, nil
		default:
			// treat all other names (including "Default") as the default
			// transfer function.
			return nil, nil
		}
	}

	fn, err := function.Extract(x, obj)
	if err != nil {
		return nil, err
	}

	if nIn, nOut := fn.Shape(); nIn != 1 || nOut != 1 {
		return nil, pdf.Errorf("wrong transfer function shape (%d,%d) != (1,1)", nIn, nOut)
	}

	return fn, nil
}
