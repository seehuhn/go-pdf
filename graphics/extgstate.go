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
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/blend"
	"seehuhn.de/go/pdf/graphics/halftone"
	"seehuhn.de/go/pdf/graphics/state"
	"seehuhn.de/go/pdf/graphics/transfer"
)

// FontExtractFunc is the function type for extracting fonts.
// This is set by the extract package to avoid import cycles.
var FontExtractFunc func(x *pdf.Extractor, obj pdf.Object) (font.Instance, error)

// PDF 2.0 sections: 8.4.5

// ExtGState represents a combination of graphics state parameters. This
// combination of parameters can then be set in a single command, using the
// [Writer.SetExtGState] method.  The parameters here form a subset of the
// parameters in the [Parameters] struct.  Parameters not present in the
// ExtGState struct, for example colors, cannot be controlled using an extended
// graphics state.
type ExtGState struct {
	Set state.Bits

	TextFont               font.Instance
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
	BlendMode              blend.Mode
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

// Equal reports whether two ExtGState values are equal.
func (e *ExtGState) Equal(other *ExtGState) bool {
	if e == nil || other == nil {
		return e == nil && other == nil
	}

	if e.Set != other.Set || e.SingleUse != other.SingleUse {
		return false
	}

	set := e.Set

	if set&state.TextFont != 0 {
		if !font.InstancesEqual(e.TextFont, other.TextFont) ||
			e.TextFontSize != other.TextFontSize {
			return false
		}
	}
	if set&state.TextKnockout != 0 && e.TextKnockout != other.TextKnockout {
		return false
	}
	if set&state.LineWidth != 0 && e.LineWidth != other.LineWidth {
		return false
	}
	if set&state.LineCap != 0 && e.LineCap != other.LineCap {
		return false
	}
	if set&state.LineJoin != 0 && e.LineJoin != other.LineJoin {
		return false
	}
	if set&state.MiterLimit != 0 && e.MiterLimit != other.MiterLimit {
		return false
	}
	if set&state.LineDash != 0 {
		if !slices.Equal(e.DashPattern, other.DashPattern) ||
			e.DashPhase != other.DashPhase {
			return false
		}
	}
	if set&state.RenderingIntent != 0 && e.RenderingIntent != other.RenderingIntent {
		return false
	}
	if set&state.StrokeAdjustment != 0 && e.StrokeAdjustment != other.StrokeAdjustment {
		return false
	}
	if set&state.BlendMode != 0 && !e.BlendMode.Equal(other.BlendMode) {
		return false
	}
	if set&state.SoftMask != 0 && !pdf.Equal(e.SoftMask, other.SoftMask) {
		return false
	}
	if set&state.StrokeAlpha != 0 && e.StrokeAlpha != other.StrokeAlpha {
		return false
	}
	if set&state.FillAlpha != 0 && e.FillAlpha != other.FillAlpha {
		return false
	}
	if set&state.AlphaSourceFlag != 0 && e.AlphaSourceFlag != other.AlphaSourceFlag {
		return false
	}
	if set&state.BlackPointCompensation != 0 && e.BlackPointCompensation != other.BlackPointCompensation {
		return false
	}
	if set&state.Overprint != 0 {
		if e.OverprintStroke != other.OverprintStroke ||
			e.OverprintFill != other.OverprintFill {
			return false
		}
	}
	if set&state.OverprintMode != 0 && e.OverprintMode != other.OverprintMode {
		return false
	}
	if set&state.BlackGeneration != 0 && !function.Equal(e.BlackGeneration, other.BlackGeneration) {
		return false
	}
	if set&state.UndercolorRemoval != 0 && !function.Equal(e.UndercolorRemoval, other.UndercolorRemoval) {
		return false
	}
	if set&state.TransferFunction != 0 {
		if !function.Equal(e.TransferFunction.Red, other.TransferFunction.Red) ||
			!function.Equal(e.TransferFunction.Green, other.TransferFunction.Green) ||
			!function.Equal(e.TransferFunction.Blue, other.TransferFunction.Blue) ||
			!function.Equal(e.TransferFunction.Gray, other.TransferFunction.Gray) {
			return false
		}
	}
	if set&state.Halftone != 0 {
		// Compare halftones by type and transfer function
		if (e.Halftone == nil) != (other.Halftone == nil) {
			return false
		}
		if e.Halftone != nil {
			if e.Halftone.HalftoneType() != other.Halftone.HalftoneType() {
				return false
			}
			if !function.Equal(e.Halftone.GetTransferFunction(), other.Halftone.GetTransferFunction()) {
				return false
			}
		}
	}
	if set&state.HalftoneOrigin != 0 {
		if e.HalftoneOriginX != other.HalftoneOriginX ||
			e.HalftoneOriginY != other.HalftoneOriginY {
			return false
		}
	}
	if set&state.FlatnessTolerance != 0 && e.FlatnessTolerance != other.FlatnessTolerance {
		return false
	}
	if set&state.SmoothnessTolerance != 0 && e.SmoothnessTolerance != other.SmoothnessTolerance {
		return false
	}

	return true
}

func ExtractExtGState(x *pdf.Extractor, obj pdf.Object) (*ExtGState, error) {
	_, isIndirect := obj.(pdf.Reference)

	dict, err := x.GetDictTyped(obj, "ExtGState")
	if err != nil {
		return nil, err
	}

	res := &ExtGState{}
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

			if FontExtractFunc == nil {
				break
			}
			F, err := FontExtractFunc(x, fontRef)
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
			res.LineCap = LineCapStyle(lineCap)
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
			res.LineJoin = LineJoinStyle(lineJoin)
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
			res.RenderingIntent = RenderingIntent(ri)
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
			res.SoftMask = v
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

// Embed adds the graphics state dictionary to a PDF file.
//
// This implements the [pdf.Embedder] interface.
func (e *ExtGState) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "ExtGState", pdf.V1_2); err != nil {
		return nil, err
	}

	set := e.Set
	if excess := set &^ extGStateBits; excess != 0 {
		return nil, fmt.Errorf("unsupported graphics state bits: 0b%b", excess)
	}

	// Build a graphics state parameter dictionary for the given state.
	// See table 57 in ISO 32000-2:2020.
	dict := pdf.Dict{}
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("ExtGState")
	}
	if set&state.TextFont != 0 {
		E, err := rm.Embed(e.TextFont)
		if err != nil {
			return nil, err
		}
		if _, ok := E.(pdf.Reference); !ok {
			err := fmt.Errorf("font %q cannot be used in ExtGState",
				e.TextFont.PostScriptName())
			return nil, err
		}
		dict["Font"] = pdf.Array{E, pdf.Number(e.TextFontSize)}
	} else {
		if e.TextFont != nil {
			return nil, errors.New("unexpected TextFont value")
		}
		if e.TextFontSize != 0 {
			return nil, errors.New("unexpected TextFontSize value")
		}
	}
	if set&state.TextKnockout != 0 {
		dict["TK"] = pdf.Boolean(e.TextKnockout)
	} else {
		if e.TextKnockout {
			return nil, errors.New("unexpected TextKnockout value")
		}
	}
	if set&state.LineWidth != 0 {
		dict["LW"] = pdf.Number(e.LineWidth)
	} else {
		if e.LineWidth != 0 {
			return nil, errors.New("unexpected LineWidth value")
		}
	}
	if set&state.LineCap != 0 {
		dict["LC"] = pdf.Integer(e.LineCap)
	} else {
		if e.LineCap != 0 {
			return nil, errors.New("unexpected LineCap value")
		}
	}
	if set&state.LineJoin != 0 {
		dict["LJ"] = pdf.Integer(e.LineJoin)
	} else {
		if e.LineJoin != 0 {
			return nil, errors.New("unexpected LineJoin value")
		}
	}
	if set&state.MiterLimit != 0 {
		dict["ML"] = pdf.Number(e.MiterLimit)
	} else {
		if e.MiterLimit != 0 {
			return nil, errors.New("unexpected MiterLimit value")
		}
	}
	if set&state.LineDash != 0 {
		pat := make(pdf.Array, len(e.DashPattern))
		for i, x := range e.DashPattern {
			pat[i] = pdf.Number(x)
		}
		dict["D"] = pdf.Array{
			pat,
			pdf.Number(e.DashPhase),
		}
	} else {
		if e.DashPattern != nil {
			return nil, errors.New("unexpected DashPattern value")
		}
		if e.DashPhase != 0 {
			return nil, errors.New("unexpected DashPhase value")
		}
	}
	if set&state.RenderingIntent != 0 {
		dict["RI"] = pdf.Name(e.RenderingIntent)
	} else {
		if e.RenderingIntent != "" {
			return nil, errors.New("unexpected RenderingIntent value")
		}
	}
	if set&state.StrokeAdjustment != 0 {
		dict["SA"] = pdf.Boolean(e.StrokeAdjustment)
	} else {
		if e.StrokeAdjustment {
			return nil, errors.New("unexpected StrokeAdjustment value")
		}
	}
	if set&state.BlendMode != 0 {
		dict["BM"] = e.BlendMode.AsPDF()
	} else {
		if !e.BlendMode.IsZero() {
			return nil, errors.New("unexpected BlendMode value")
		}
	}
	if set&state.SoftMask != 0 {
		dict["SMask"] = e.SoftMask
	} else {
		if e.SoftMask != nil {
			return nil, errors.New("unexpected SoftMask value")
		}
	}
	if set&state.StrokeAlpha != 0 {
		dict["CA"] = pdf.Number(e.StrokeAlpha)
	} else {
		if e.StrokeAlpha != 0 {
			return nil, errors.New("unexpected StrokeAlpha value")
		}
	}
	if set&state.FillAlpha != 0 {
		dict["ca"] = pdf.Number(e.FillAlpha)
	} else {
		if e.FillAlpha != 0 {
			return nil, errors.New("unexpected FillAlpha value")
		}
	}
	if set&state.AlphaSourceFlag != 0 {
		dict["AIS"] = pdf.Boolean(e.AlphaSourceFlag)
	} else {
		if e.AlphaSourceFlag {
			return nil, errors.New("unexpected AlphaSourceFlag value")
		}
	}
	if set&state.BlackPointCompensation != 0 {
		dict["UseBlackPtComp"] = e.BlackPointCompensation
	} else {
		if e.BlackPointCompensation != "" {
			return nil, errors.New("unexpected BlackPointCompensation value")
		}
	}

	if set&state.Overprint != 0 {
		dict["OP"] = pdf.Boolean(e.OverprintStroke)
		if e.OverprintFill != e.OverprintStroke {
			dict["op"] = pdf.Boolean(e.OverprintFill)
		}
	} else {
		if e.OverprintStroke {
			return nil, errors.New("unexpected OverprintStroke value")
		}
		if e.OverprintFill {
			return nil, errors.New("unexpected OverprintFill value")
		}
	}
	if set&state.OverprintMode != 0 {
		dict["OPM"] = pdf.Integer(e.OverprintMode)
	} else {
		if e.OverprintMode != 0 {
			return nil, errors.New("unexpected OverprintMode value")
		}
	}
	if set&state.BlackGeneration != 0 {
		if e.BlackGeneration == nil {
			if err := pdf.CheckVersion(rm.Out(), "BG2 in ExtGState", pdf.V1_3); err != nil {
				return nil, err
			}
			dict["BG2"] = pdf.Name("Default")
		} else {
			obj, err := rm.Embed(e.BlackGeneration)
			if err != nil {
				return nil, err
			}
			dict["BG"] = obj
		}
	} else {
		if e.BlackGeneration != nil {
			return nil, errors.New("unexpected BlackGeneration value")
		}
	}
	if set&state.UndercolorRemoval != 0 {
		if e.UndercolorRemoval == nil {
			if err := pdf.CheckVersion(rm.Out(), "UCR2 in ExtGState", pdf.V1_3); err != nil {
				return nil, err
			}
			dict["UCR2"] = pdf.Name("Default")
		} else {
			obj, err := rm.Embed(e.UndercolorRemoval)
			if err != nil {
				return nil, err
			}
			dict["UCR"] = obj
		}
	} else {
		if e.UndercolorRemoval != nil {
			return nil, errors.New("unexpected UndercolorRemoval value")
		}
	}
	if set&state.TransferFunction != 0 {
		if v := pdf.GetVersion(rm.Out()); v >= pdf.V2_0 {
			return nil, errors.New("TransferFunction is deprecated in PDF 2.0")
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
				if err := pdf.CheckVersion(rm.Out(), "TR2 in ExtGState", pdf.V1_3); err != nil {
					return nil, err
				}
				key = "TR2"
			} else if nIn, nOut := fn.Shape(); nIn != 1 || nOut != 1 {
				return nil, fmt.Errorf("wrong transfer function shape (%d,%d) != (1,1)", nIn, nOut)
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
				obj, err = rm.Embed(fn)
				if err != nil {
					return nil, err
				}
			}
			a[i] = obj
		}
		if needsArray {
			dict[key] = a
		} else {
			dict[key] = a[0]
		}
	} else {
		if e.TransferFunction.Red != nil || e.TransferFunction.Green != nil ||
			e.TransferFunction.Blue != nil || e.TransferFunction.Gray != nil {
			return nil, errors.New("unexpected TransferFunction value")
		}
	}
	if set&state.Halftone != 0 {
		htEmbedded, err := rm.Embed(e.Halftone)
		if err != nil {
			return nil, err
		}
		dict["HT"] = htEmbedded
	} else {
		if e.Halftone != nil {
			return nil, errors.New("unexpected Halftone value")
		}
	}
	if set&state.HalftoneOrigin != 0 {
		dict["HTO"] = pdf.Array{
			pdf.Number(e.HalftoneOriginX),
			pdf.Number(e.HalftoneOriginY),
		}
	} else {
		if e.HalftoneOriginX != 0 {
			return nil, errors.New("unexpected HalftoneOriginX value")
		}
		if e.HalftoneOriginY != 0 {
			return nil, errors.New("unexpected HalftoneOriginY value")
		}
	}
	if set&state.FlatnessTolerance != 0 {
		dict["FL"] = pdf.Number(e.FlatnessTolerance)
	} else {
		if e.FlatnessTolerance != 0 {
			return nil, errors.New("unexpected FlatnessTolerance value")
		}
	}
	if set&state.SmoothnessTolerance != 0 {
		dict["SM"] = pdf.Number(e.SmoothnessTolerance)
	} else {
		if e.SmoothnessTolerance != 0 {
			return nil, errors.New("unexpected SmoothnessTolerance value")
		}
	}

	if e.SingleUse {
		return dict, nil
	}
	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// ApplyTo modifies the given graphics state according to the parameters in
// the extended graphics state.
func (e *ExtGState) ApplyTo(s *State) {
	set := e.Set
	s.Set |= set

	param := s.Parameters
	if set&state.TextFont != 0 {
		param.TextFont = e.TextFont
		param.TextFontSize = e.TextFontSize
	}
	if set&state.TextKnockout != 0 {
		param.TextKnockout = e.TextKnockout
	}
	if set&state.LineWidth != 0 {
		param.LineWidth = e.LineWidth
	}
	if set&state.LineCap != 0 {
		param.LineCap = e.LineCap
	}
	if set&state.LineJoin != 0 {
		param.LineJoin = e.LineJoin
	}
	if set&state.MiterLimit != 0 {
		param.MiterLimit = e.MiterLimit
	}
	if set&state.LineDash != 0 {
		param.DashPattern = slices.Clone(e.DashPattern)
		param.DashPhase = e.DashPhase
	}
	if set&state.RenderingIntent != 0 {
		param.RenderingIntent = e.RenderingIntent
	}
	if set&state.StrokeAdjustment != 0 {
		param.StrokeAdjustment = e.StrokeAdjustment
	}
	if set&state.BlendMode != 0 {
		param.BlendMode = e.BlendMode
	}
	if set&state.SoftMask != 0 {
		param.SoftMask = e.SoftMask
	}
	if set&state.StrokeAlpha != 0 {
		param.StrokeAlpha = e.StrokeAlpha
	}
	if set&state.FillAlpha != 0 {
		param.FillAlpha = e.FillAlpha
	}
	if set&state.AlphaSourceFlag != 0 {
		param.AlphaSourceFlag = e.AlphaSourceFlag
	}
	if set&state.BlackPointCompensation != 0 {
		param.BlackPointCompensation = e.BlackPointCompensation
	}
	if set&state.Overprint != 0 {
		param.OverprintStroke = e.OverprintStroke
		param.OverprintFill = e.OverprintFill
	}
	if set&state.OverprintMode != 0 {
		param.OverprintMode = e.OverprintMode
	}
	if set&state.BlackGeneration != 0 {
		param.BlackGeneration = e.BlackGeneration
	}
	if set&state.UndercolorRemoval != 0 {
		param.UndercolorRemoval = e.UndercolorRemoval
	}
	if set&state.TransferFunction != 0 {
		param.TransferFunction = e.TransferFunction
	}
	if set&state.Halftone != 0 {
		param.Halftone = e.Halftone
	}
	if set&state.HalftoneOrigin != 0 {
		param.HalftoneOriginX = e.HalftoneOriginX
		param.HalftoneOriginY = e.HalftoneOriginY
	}
	if set&state.FlatnessTolerance != 0 {
		param.FlatnessTolerance = e.FlatnessTolerance
	}
	if set&state.SmoothnessTolerance != 0 {
		param.SmoothnessTolerance = e.SmoothnessTolerance
	}
}
