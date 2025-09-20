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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	fontdict "seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/halftone"
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

func ExtractExtGState(x *pdf.Extractor, obj pdf.Object) (*ExtGState, error) {
	dict, err := pdf.GetDictTyped(x.R, obj, "ExtGState")
	if err != nil {
		return nil, err
	}

	res := &ExtGState{}
	var set StateBits
	var overprintFillSet bool
	var bg1, bg2 pdf.Object
	var ucr1, ucr2 pdf.Object
	var tr1, tr2 pdf.Object

	for key, v := range dict {
		switch key {
		case "Font":
			a, err := pdf.GetArray(x.R, v)
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

			size, err := pdf.GetNumber(x.R, a[1])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}

			F, err := fontdict.ExtractFont(x, fontRef)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}

			res.TextFont = F
			res.TextFontSize = float64(size)
			set |= StateTextFont
		case "TK":
			val, err := pdf.GetBoolean(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.TextKnockout = bool(val)
			set |= StateTextKnockout
		case "LW":
			lw, err := pdf.GetNumber(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.LineWidth = float64(lw)
			set |= StateLineWidth
		case "LC":
			lineCap, err := pdf.GetInteger(x.R, v)
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
			res.LineCap = LineCapStyle(lineCap)
			set |= StateLineCap
		case "LJ":
			lineJoin, err := pdf.GetInteger(x.R, v)
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
			res.LineJoin = LineJoinStyle(lineJoin)
			set |= StateLineJoin
		case "ML":
			miterLimit, err := pdf.GetNumber(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if miterLimit < 1 {
				miterLimit = 1
			}
			res.MiterLimit = float64(miterLimit)
			set |= StateMiterLimit
		case "D":
			dashPattern, phase, err := readDash(x.R, v)
			if err != nil {
				return nil, err
			} else if dashPattern != nil {
				res.DashPattern = dashPattern
				res.DashPhase = phase
				set |= StateLineDash
			}
		case "RI":
			ri, err := pdf.GetName(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.RenderingIntent = RenderingIntent(ri)
			set |= StateRenderingIntent
		case "SA":
			val, err := pdf.GetBoolean(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.StrokeAdjustment = bool(val)
			set |= StateStrokeAdjustment
		case "BM":
			res.BlendMode = v
			set |= StateBlendMode
		case "SMask":
			res.SoftMask = v
			set |= StateSoftMask
		case "CA":
			ca, err := pdf.GetNumber(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.StrokeAlpha = float64(ca)
			set |= StateStrokeAlpha
		case "ca":
			ca, err := pdf.GetNumber(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.FillAlpha = float64(ca)
			set |= StateFillAlpha
		case "AIS":
			ais, err := pdf.GetBoolean(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.AlphaSourceFlag = bool(ais)
			set |= StateAlphaSourceFlag
		case "UseBlackPtComp":
			val, err := pdf.GetName(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.BlackPointCompensation = val
			set |= StateBlackPointCompensation
		case "OP":
			op, err := pdf.GetBoolean(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.OverprintStroke = bool(op)
			set |= StateOverprint
		case "op":
			op, err := pdf.GetBoolean(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.OverprintFill = bool(op)
			set |= StateOverprint
			overprintFillSet = true
		case "OPM":
			opm, err := pdf.GetInteger(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if opm != 0 {
				res.OverprintMode = 1
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
			ht, err := halftone.Extract(x, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.Halftone = ht
			set |= StateHalftone
		case "HTO":
			a, err := pdf.GetArray(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			if len(a) != 2 {
				break
			}
			xCoord, err := pdf.GetNumber(x.R, a[0])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			yCoord, err := pdf.GetNumber(x.R, a[1])
			if pdf.IsMalformed(err) {
				break
			}
			res.HalftoneOriginX = float64(xCoord)
			res.HalftoneOriginY = float64(yCoord)
			set |= StateHalftoneOrigin
		case "FL":
			fl, err := pdf.GetNumber(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.FlatnessTolerance = float64(fl)
			set |= StateFlatnessTolerance
		case "SM":
			sm, err := pdf.GetNumber(x.R, v)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return nil, err
			}
			res.SmoothnessTolerance = float64(sm)
			set |= StateSmoothnessTolerance
		}
	}

	// Handle overprint fill fallback (like in reader)
	if set&StateOverprint != 0 && !overprintFillSet {
		res.OverprintFill = res.OverprintStroke
	}

	// Handle BlackGeneration precedence: BG2 > BG
	if bg2 == pdf.Name("Default") {
		res.BlackGeneration = nil
		bg2 = nil
		set |= StateBlackGeneration
	}
	if set&StateBlackGeneration == 0 && bg2 != nil {
		fn, err := function.Extract(x, bg2)
		if err == nil {
			if nIn, nOut := fn.Shape(); nIn == 1 && nOut == 1 {
				res.BlackGeneration = fn
				set |= StateBlackGeneration
			}
		}
	}
	if set&StateBlackGeneration == 0 && bg1 != nil {
		fn, err := function.Extract(x, bg1)
		if err == nil {
			if nIn, nOut := fn.Shape(); nIn == 1 && nOut == 1 {
				res.BlackGeneration = fn
				set |= StateBlackGeneration
			}
		}
	}

	// Handle UndercolorRemoval precedence: UCR2 > UCR
	if ucr2 == pdf.Name("Default") {
		res.UndercolorRemoval = nil
		ucr2 = nil
		set |= StateUndercolorRemoval
	}
	if set&StateUndercolorRemoval == 0 && ucr2 != nil {
		fn, err := function.Extract(x, ucr2)
		if err == nil {
			if nIn, nOut := fn.Shape(); nIn == 1 && nOut == 1 {
				res.UndercolorRemoval = fn
				set |= StateUndercolorRemoval
			}
		}
	}
	if set&StateUndercolorRemoval == 0 && ucr1 != nil {
		fn, err := function.Extract(x, ucr1)
		if err == nil {
			if nIn, nOut := fn.Shape(); nIn == 1 && nOut == 1 {
				res.UndercolorRemoval = fn
				set |= StateUndercolorRemoval
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
			set |= StateTransferFunction
		} else if tr1 != nil {
			fn, err := parseTransferFunction(x.R, tr1)
			if err != nil {
				return nil, err
			}
			res.TransferFunction = fn
			set |= StateTransferFunction
		}
	}

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

// Embed adds the graphics state dictionary to a PDF file.
//
// This implements the [pdf.Embedder] interface.
//
// TODO(voss): remove the State return value?
func (e *ExtGState) Embed(rm *pdf.ResourceManager) (pdf.Native, State, error) {
	res := State{
		Parameters: &Parameters{},
	}

	if err := pdf.CheckVersion(rm.Out, "ExtGState", pdf.V1_2); err != nil {
		return nil, res, err
	}

	set := e.Set

	// Build a graphics state parameter dictionary for the given state.
	// See table 57 in ISO 32000-2:2020.
	dict := pdf.Dict{}
	if set&StateTextFont != 0 {
		E, embedded, err := pdf.ResourceManagerEmbed(rm, e.TextFont)
		if err != nil {
			return nil, res, err
		}
		if _, ok := E.(pdf.Reference); !ok {
			err := fmt.Errorf("font %q cannot be used in ExtGState",
				e.TextFont.PostScriptName())
			return nil, res, err
		}
		dict["Font"] = pdf.Array{E, pdf.Number(e.TextFontSize)}
		res.TextFont = embedded
		res.TextFontSize = e.TextFontSize
	}
	if set&StateTextKnockout != 0 {
		dict["TK"] = pdf.Boolean(e.TextKnockout)
		res.TextKnockout = e.TextKnockout
	}
	if set&StateLineWidth != 0 {
		dict["LW"] = pdf.Number(e.LineWidth)
		res.LineWidth = e.LineWidth
	}
	if set&StateLineCap != 0 {
		dict["LC"] = pdf.Integer(e.LineCap)
		res.LineCap = e.LineCap
	}
	if set&StateLineJoin != 0 {
		dict["LJ"] = pdf.Integer(e.LineJoin)
		res.LineJoin = e.LineJoin
	}
	if set&StateMiterLimit != 0 {
		dict["ML"] = pdf.Number(e.MiterLimit)
		res.MiterLimit = e.MiterLimit
	}
	if set&StateLineDash != 0 {
		pat := make(pdf.Array, len(e.DashPattern))
		for i, x := range e.DashPattern {
			pat[i] = pdf.Number(x)
		}
		dict["D"] = pdf.Array{
			pat,
			pdf.Number(e.DashPhase),
		}
		res.DashPattern = e.DashPattern
		res.DashPhase = e.DashPhase
	}
	if set&StateRenderingIntent != 0 {
		dict["RI"] = pdf.Name(e.RenderingIntent)
		res.RenderingIntent = e.RenderingIntent
	}
	if set&StateStrokeAdjustment != 0 {
		dict["SA"] = pdf.Boolean(e.StrokeAdjustment)
		res.StrokeAdjustment = e.StrokeAdjustment
	}
	if set&StateBlendMode != 0 {
		dict["BM"] = e.BlendMode
		res.BlendMode = e.BlendMode
	}
	if set&StateSoftMask != 0 {
		dict["SMask"] = e.SoftMask
		res.SoftMask = e.SoftMask
	}
	if set&StateStrokeAlpha != 0 {
		dict["CA"] = pdf.Number(e.StrokeAlpha)
		res.StrokeAlpha = e.StrokeAlpha
	}
	if set&StateFillAlpha != 0 {
		dict["ca"] = pdf.Number(e.FillAlpha)
		res.FillAlpha = e.FillAlpha
	}
	if set&StateAlphaSourceFlag != 0 {
		dict["AIS"] = pdf.Boolean(e.AlphaSourceFlag)
		res.AlphaSourceFlag = e.AlphaSourceFlag
	}
	if set&StateBlackPointCompensation != 0 {
		dict["UseBlackPtComp"] = e.BlackPointCompensation
		res.BlackPointCompensation = e.BlackPointCompensation
	}

	if set&StateOverprint != 0 {
		dict["OP"] = pdf.Boolean(e.OverprintStroke)
		if e.OverprintFill != e.OverprintStroke {
			dict["op"] = pdf.Boolean(e.OverprintFill)
		}
		res.OverprintStroke = e.OverprintStroke
		res.OverprintFill = e.OverprintFill
	}
	if set&StateOverprintMode != 0 {
		dict["OPM"] = pdf.Integer(e.OverprintMode)
		res.OverprintMode = e.OverprintMode
	}
	if set&StateBlackGeneration != 0 {
		if e.BlackGeneration == nil {
			if err := pdf.CheckVersion(rm.Out, "BG2 in ExtGState", pdf.V1_3); err != nil {
				return nil, res, err
			}
			dict["BG2"] = pdf.Name("Default")
		} else {
			obj, _, err := pdf.ResourceManagerEmbed(rm, e.BlackGeneration)
			if err != nil {
				return nil, res, err
			}
			dict["BG"] = obj
		}
		res.BlackGeneration = e.BlackGeneration
	}
	if set&StateUndercolorRemoval != 0 {
		if e.UndercolorRemoval == nil {
			if err := pdf.CheckVersion(rm.Out, "UCR2 in ExtGState", pdf.V1_3); err != nil {
				return nil, res, err
			}
			dict["UCR2"] = pdf.Name("Default")
		} else {
			obj, _, err := pdf.ResourceManagerEmbed(rm, e.UndercolorRemoval)
			if err != nil {
				return nil, res, err
			}
			dict["UCR"] = obj
		}
		res.UndercolorRemoval = e.UndercolorRemoval
	}
	if set&StateTransferFunction != 0 {
		if v := pdf.GetVersion(rm.Out); v >= pdf.V2_0 {
			return nil, res, errors.New("TransferFunction is deprecated in PDF 2.0")
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
				if err := pdf.CheckVersion(rm.Out, "TR2 in ExtGState", pdf.V1_3); err != nil {
					return nil, res, err
				}
				key = "TR2"
			} else if nIn, nOut := fn.Shape(); nIn != 1 || nOut != 1 {
				return nil, res, fmt.Errorf("wrong transfer function shape (%d,%d) != (1,1)", nIn, nOut)
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
				obj, _, err = pdf.ResourceManagerEmbed(rm, fn)
				if err != nil {
					return nil, res, err
				}
			}
			a[i] = obj
		}
		if needsArray {
			dict[key] = a
		} else {
			dict[key] = a[0]
		}
		res.TransferFunction = e.TransferFunction
	}
	if set&StateHalftone != 0 {
		htEmbedded, _, err := pdf.ResourceManagerEmbed(rm, e.Halftone)
		if err != nil {
			return nil, res, err
		}
		dict["HT"] = htEmbedded
		res.Halftone = e.Halftone
	}
	if set&StateHalftoneOrigin != 0 {
		dict["HTO"] = pdf.Array{
			pdf.Number(e.HalftoneOriginX),
			pdf.Number(e.HalftoneOriginY),
		}
		res.HalftoneOriginX = e.HalftoneOriginX
		res.HalftoneOriginY = e.HalftoneOriginY
	}
	if set&StateFlatnessTolerance != 0 {
		dict["FL"] = pdf.Number(e.FlatnessTolerance)
		res.FlatnessTolerance = e.FlatnessTolerance
	}
	if set&StateSmoothnessTolerance != 0 {
		dict["SM"] = pdf.Number(e.SmoothnessTolerance)
		res.SmoothnessTolerance = e.SmoothnessTolerance
	}

	res.Set = set & extGStateBits

	if e.SingleUse {
		return dict, res, nil
	}
	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, dict)
	if err != nil {
		return nil, res, err
	}
	return ref, res, nil
}
