// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

import (
	"slices"

	"seehuhn.de/go/pdf/graphics/color"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// Operands returns the operator and the operands which take effect when the
// operator name with operands args is interpreted permissively in the
// current state.
//
// The result ok is false if the operator is left out, for want of usable
// operands or because it closes a marked-content sequence which is left out;
// used is then nil.  Otherwise op is the operator to interpret and
// used its operands.  The result same is true if op is name and used
// consists of the leading elements of args, unchanged.  The slice used must
// not be modified.
//
// Operands does not take [State.Ignored] into account: an ignored operator
// has no effect whatever its operands are.
//
// The leading operands must have the kinds which the operator takes, and
// surplus operands are left out.  In addition:
//   - Tf needs a font which the resources provide, directly or through
//     [Resources.FontFallback];
//   - CS and cs need a device colour space, Pattern, or a colour space
//     which the resources provide;
//   - d needs an array of numbers;
//   - SC, SCN, sc and scn keep as many numeric operands as the current
//     colour space has components, and SCN and scn also keep the last
//     name operand, placed last;
//   - the array of TJ keeps only its strings and numbers;
//   - BDC and DP whose second operand is neither a dictionary nor a name
//     are read as BMC and MP;
//   - EMC is rejected if [State.ApplyStateChanges] has seen the BMC or BDC
//     which opens the sequence and that operator was rejected, so that a
//     marked-content sequence without a usable tag is left out as a whole.
//
// Operators which are not known, and inline images, keep their operands.
func (s *State) Operands(name OpName, args []pdf.Object) (op OpName, used []pdf.Object, same, ok bool) {
	return s.operands(name, args, nil)
}

// operands implements [State.Operands].  If tf is not nil, it receives the
// font which an accepted Tf operator selects.
func (s *State) operands(name OpName, args []pdf.Object, tf *font.Instance) (op OpName, used []pdf.Object, same, ok bool) {
	// match reports whether the leading operands have the given kinds:
	// n for a number, N for a name, s for a string, a for an array and p
	// for a property list
	match := func(kinds string) bool {
		if len(args) < len(kinds) {
			return false
		}
		for i, k := range []byte(kinds) {
			var ok bool
			switch k {
			case 'n':
				ok = isNumber(args[i])
			case 'N':
				_, ok = args[i].(pdf.Name)
			case 's':
				_, ok = args[i].(pdf.String)
			case 'a':
				_, ok = args[i].(pdf.Array)
			case 'p':
				switch args[i].(type) {
				case pdf.Dict, pdf.Name:
					ok = true
				}
			}
			if !ok {
				return false
			}
		}
		return true
	}
	take := func(kinds string) (OpName, []pdf.Object, bool, bool) {
		if !match(kinds) {
			return name, nil, false, false
		}
		return name, args[:len(kinds)], true, true
	}

	switch name {
	case OpPushGraphicsState, OpPopGraphicsState,
		OpTextBegin, OpTextEnd, OpTextNextLine,
		OpBeginCompatibility, OpEndCompatibility,
		OpClosePath, OpClipNonZero, OpClipEvenOdd,
		OpStroke, OpCloseAndStroke, OpFill,
		OpFillCompat, OpFillEvenOdd, OpFillAndStroke,
		OpFillAndStrokeEvenOdd, OpCloseFillAndStroke,
		OpCloseFillAndStrokeEvenOdd, OpEndPath:
		return take("")
	case OpSetLineWidth, OpSetLineCap, OpSetLineJoin,
		OpSetMiterLimit, OpSetFlatnessTolerance,
		OpTextSetCharacterSpacing, OpTextSetWordSpacing,
		OpTextSetHorizontalScaling, OpTextSetLeading,
		OpTextSetRenderingMode, OpTextSetRise,
		OpSetFillGray, OpSetStrokeGray:
		return take("n")
	case OpMoveTo, OpLineTo, OpTextMoveOffset,
		OpTextMoveOffsetSetLeading, OpType3ColoredGlyph:
		return take("nn")
	case OpSetFillRGB, OpSetStrokeRGB:
		return take("nnn")
	case OpSetFillCMYK, OpSetStrokeCMYK,
		OpCurveToV, OpCurveToY, OpRectangle:
		return take("nnnn")
	case OpCurveTo, OpTransform, OpTextSetMatrix,
		OpType3UncoloredGlyph:
		return take("nnnnnn")
	case OpSetRenderingIntent, OpSetExtGState, OpXObject,
		OpShading, OpBeginMarkedContent, OpMarkedContentPoint:
		return take("N")
	case OpTextShow, OpTextShowMoveNextLine:
		return take("s")
	case OpTextShowMoveNextLineSetSpacing:
		return take("nns")

	case OpSetFillColorSpace, OpSetStrokeColorSpace:
		if !match("N") || s.getColorSpace(args[0].(pdf.Name)) == nil {
			return name, nil, false, false
		}
		return take("N")
	case OpTextSetFont:
		if !match("Nn") {
			return name, nil, false, false
		}
		F := s.font(args[0].(pdf.Name))
		if F == nil {
			return name, nil, false, false
		}
		if tf != nil {
			*tf = F
		}
		return take("Nn")
	case OpSetLineDash:
		if !match("an") {
			return name, nil, false, false
		}
		for _, x := range args[0].(pdf.Array) {
			if !isNumber(x) {
				return name, nil, false, false
			}
		}
		return take("an")

	case OpSetFillColor, OpSetFillColorN, OpSetStrokeColor, OpSetStrokeColorN:
		used, same := s.colorOperands(name, args)
		return name, used, same, true

	case OpEndMarkedContent:
		if s.closesDropped() {
			return name, nil, false, false
		}
		return take("")
	case OpBeginMarkedContentWithProperties:
		if match("Np") {
			return take("Np")
		}
		_, used, _, ok := take("N")
		return OpBeginMarkedContent, used, false, ok
	case OpMarkedContentPointWithProperties:
		if match("Np") {
			return take("Np")
		}
		_, used, _, ok := take("N")
		return OpMarkedContentPoint, used, false, ok

	case OpTextShowArray:
		if !match("a") {
			return name, nil, false, false
		}
		a := args[0].(pdf.Array)
		if !slices.ContainsFunc(a, isJunk) {
			return take("a")
		}
		kept := make(pdf.Array, 0, len(a))
		for _, x := range a {
			if !isJunk(x) {
				kept = append(kept, x)
			}
		}
		return name, []pdf.Object{kept}, false, true
	}
	return name, args, true, true
}

// colorOperands returns the operands of an SC, SCN, sc or scn operator which
// [State.Operands] keeps, and whether these are leading elements of args.
func (s *State) colorOperands(name OpName, args []pdf.Object) ([]pdf.Object, bool) {
	col := s.GState.FillColor
	if name == OpSetStrokeColor || name == OpSetStrokeColorN {
		col = s.GState.StrokeColor
	}
	n := len(args)
	if col != nil && col.ColorSpace() != nil {
		n = col.ColorSpace().Channels()
	}
	withName := name == OpSetFillColorN || name == OpSetStrokeColorN

	// the kept numbers, in order, and the index of the last kept name
	numbers := 0
	nameIdx := -1
	prefix := true // whether the kept operands are a prefix of args
	for i, a := range args {
		if isNumber(a) {
			if numbers < n {
				prefix = prefix && i == numbers
				numbers++
			}
		} else if _, ok := a.(pdf.Name); ok && withName {
			nameIdx = i
		}
	}
	if nameIdx < 0 && prefix {
		return args[:numbers], true
	}
	if nameIdx == numbers && prefix {
		return args[:numbers+1], true
	}

	res := make([]pdf.Object, 0, numbers+1)
	for _, a := range args {
		if isNumber(a) && len(res) < numbers {
			res = append(res, a)
		}
	}
	if nameIdx >= 0 {
		res = append(res, args[nameIdx])
	}
	return res, false
}

// hasAllComponents reports whether the operands of an SC, SCN, sc or scn
// operator include a number for every component of the current colour space.
// It returns true for all other operators.
func (s *State) hasAllComponents(name OpName, args []pdf.Object) bool {
	var col color.Color
	switch name {
	case OpSetFillColor, OpSetFillColorN:
		col = s.GState.FillColor
	case OpSetStrokeColor, OpSetStrokeColorN:
		col = s.GState.StrokeColor
	default:
		return true
	}
	if col == nil || col.ColorSpace() == nil {
		return true
	}
	numbers := 0
	for _, a := range args {
		if isNumber(a) {
			numbers++
		}
	}
	return numbers == col.ColorSpace().Channels()
}

// font returns the font which the resources provide under name, or nil.
func (s *State) font(name pdf.Name) font.Instance {
	res := s.Resources
	if res == nil {
		return nil
	}
	if F := res.Font[name]; F != nil {
		return F
	}
	if res.FontFallback != nil {
		return res.FontFallback(name)
	}
	return nil
}

// isJunk reports whether x is neither a string nor a number.
func isJunk(x pdf.Object) bool {
	_, isStr := x.(pdf.String)
	return !isStr && !isNumber(x)
}

// isNumber reports whether x is a numeric operand.
func isNumber(x pdf.Object) bool {
	_, ok := numberValue(x)
	return ok
}
