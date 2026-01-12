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

package content

import (
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// applyOperatorToParams updates the graphics state parameters based on the operator.
// This is called by ApplyOperator after structural state changes (q/Q, BT/ET, etc.)
// have been handled.
func (s *State) applyOperatorToParams(name OpName, args []pdf.Object) {
	p := s.GState

	switch name {
	// Graphics state operators (Table 56)

	case OpTransform: // cm
		if m, ok := getMatrix(args); ok {
			p.CTM = m.Mul(p.CTM)
		}

	case OpSetLineWidth: // w
		if w, ok := getNumber(args, 0); ok {
			p.LineWidth = w
			s.GState.Set |= graphics.StateLineWidth
		}

	case OpSetLineCap: // J
		if cap, ok := getInteger(args, 0); ok {
			if cap < 0 {
				cap = 0
			} else if cap > 2 {
				cap = 2
			}
			p.LineCap = graphics.LineCapStyle(cap)
			s.GState.Set |= graphics.StateLineCap
		}

	case OpSetLineJoin: // j
		if join, ok := getInteger(args, 0); ok {
			if join < 0 {
				join = 0
			} else if join > 2 {
				join = 2
			}
			p.LineJoin = graphics.LineJoinStyle(join)
			s.GState.Set |= graphics.StateLineJoin
		}

	case OpSetMiterLimit: // M
		if limit, ok := getNumber(args, 0); ok {
			if limit < 1 {
				limit = 1
			}
			p.MiterLimit = limit
			s.GState.Set |= graphics.StateMiterLimit
		}

	case OpSetLineDash: // d
		if len(args) >= 2 {
			if patArr, ok := args[0].(pdf.Array); ok {
				if phase, pok := getNumber(args, 1); pok {
					if pat, dok := convertDashPattern(patArr); dok {
						p.DashPattern = pat
						p.DashPhase = phase
						s.GState.Set |= graphics.StateLineDash
					}
				}
			}
		}

	case OpSetRenderingIntent: // ri
		if intent, ok := getName(args, 0); ok {
			p.RenderingIntent = graphics.RenderingIntent(intent)
			s.GState.Set |= graphics.StateRenderingIntent
		}

	case OpSetFlatnessTolerance: // i
		if flatness, ok := getNumber(args, 0); ok {
			if flatness < 0 {
				flatness = 0
			} else if flatness > 100 {
				flatness = 100
			}
			p.FlatnessTolerance = flatness
			s.GState.Set |= graphics.StateFlatnessTolerance
		}

	case OpSetExtGState: // gs
		if dictName, ok := getName(args, 0); ok {
			if s.Resources != nil && s.Resources.ExtGState != nil {
				if extGState := s.Resources.ExtGState[dictName]; extGState != nil {
					extGState.ApplyTo(s.GState)
					s.Usable |= s.GState.Set
				}
			}
		}

	// Text object operators (Table 105)

	case OpTextBegin: // BT
		p.TextMatrix = matrix.Identity
		p.TextLineMatrix = matrix.Identity
		s.GState.Set |= graphics.StateTextMatrix

	case OpTextEnd: // ET
		// TextMatrix is cleared by ApplyStateChanges

	// Text state operators (Table 103)

	case OpTextSetCharacterSpacing: // Tc
		if cs, ok := getNumber(args, 0); ok {
			p.TextCharacterSpacing = cs
			s.GState.Set |= graphics.StateTextCharacterSpacing
		}

	case OpTextSetWordSpacing: // Tw
		if ws, ok := getNumber(args, 0); ok {
			p.TextWordSpacing = ws
			s.GState.Set |= graphics.StateTextWordSpacing
		}

	case OpTextSetHorizontalScaling: // Tz
		if scale, ok := getNumber(args, 0); ok {
			p.TextHorizontalScaling = scale / 100
			s.GState.Set |= graphics.StateTextHorizontalScaling
		}

	case OpTextSetLeading: // TL
		if leading, ok := getNumber(args, 0); ok {
			p.TextLeading = leading
			s.GState.Set |= graphics.StateTextLeading
		}

	case OpTextSetFont: // Tf
		fontName, ok1 := getName(args, 0)
		size, ok2 := getNumber(args, 1)
		if ok1 && ok2 && s.Resources != nil && s.Resources.Font != nil {
			if F := s.Resources.Font[fontName]; F != nil {
				p.TextFont = F
				p.TextFontSize = size
				s.GState.Set |= graphics.StateTextFont
			}
		}

	case OpTextSetRenderingMode: // Tr
		if mode, ok := getInteger(args, 0); ok {
			if mode < 0 {
				mode = 0
			} else if mode > 7 {
				mode = 7
			}
			p.TextRenderingMode = graphics.TextRenderingMode(mode)
			s.GState.Set |= graphics.StateTextRenderingMode
		}

	case OpTextSetRise: // Ts
		if rise, ok := getNumber(args, 0); ok {
			p.TextRise = rise
			s.GState.Set |= graphics.StateTextRise
		}

	// Text positioning operators (Table 106)

	case OpTextMoveOffset: // Td
		tx, ok1 := getNumber(args, 0)
		ty, ok2 := getNumber(args, 1)
		if ok1 && ok2 {
			p.TextLineMatrix = matrix.Translate(tx, ty).Mul(p.TextLineMatrix)
			p.TextMatrix = p.TextLineMatrix
		}

	case OpTextMoveOffsetSetLeading: // TD
		tx, ok1 := getNumber(args, 0)
		ty, ok2 := getNumber(args, 1)
		if ok1 && ok2 {
			p.TextLeading = -ty
			s.GState.Set |= graphics.StateTextLeading
			p.TextLineMatrix = matrix.Translate(tx, ty).Mul(p.TextLineMatrix)
			p.TextMatrix = p.TextLineMatrix
		}

	case OpTextSetMatrix: // Tm
		if m, ok := getMatrix(args); ok {
			p.TextMatrix = m
			p.TextLineMatrix = m
			s.GState.Set |= graphics.StateTextMatrix
		}

	case OpTextNextLine: // T*
		p.TextLineMatrix = matrix.Translate(0, -p.TextLeading).Mul(p.TextLineMatrix)
		p.TextMatrix = p.TextLineMatrix

	// Text showing operators (Table 107)
	// Note: These don't update text position here - that requires font info
	// and is done by reader/builder with updateTextPosition()

	case OpTextShowMoveNextLine: // '
		p.TextLineMatrix = matrix.Translate(0, -p.TextLeading).Mul(p.TextLineMatrix)
		p.TextMatrix = p.TextLineMatrix

	case OpTextShowMoveNextLineSetSpacing: // "
		aw, ok1 := getNumber(args, 0)
		ac, ok2 := getNumber(args, 1)
		if ok1 && ok2 {
			p.TextWordSpacing = aw
			p.TextCharacterSpacing = ac
			s.GState.Set |= graphics.StateTextWordSpacing | graphics.StateTextCharacterSpacing
			p.TextLineMatrix = matrix.Translate(0, -p.TextLeading).Mul(p.TextLineMatrix)
			p.TextMatrix = p.TextLineMatrix
		}

	// Color operators (Table 73)

	case OpSetStrokeColorSpace: // CS
		if name, ok := getName(args, 0); ok {
			cs := s.getColorSpace(name)
			if cs != nil {
				p.StrokeColor = cs.Default()
				s.GState.Set |= graphics.StateStrokeColor
			}
		}

	case OpSetFillColorSpace: // cs
		if name, ok := getName(args, 0); ok {
			cs := s.getColorSpace(name)
			if cs != nil {
				p.FillColor = cs.Default()
				s.GState.Set |= graphics.StateFillColor
			}
		}

	case OpSetStrokeColor, OpSetStrokeColorN: // SC, SCN
		values, pat := s.parseColorArgs(args)
		p.StrokeColor = color.SCN(p.StrokeColor, values, pat)
		s.GState.Set |= graphics.StateStrokeColor

	case OpSetFillColor, OpSetFillColorN: // sc, scn
		values, pat := s.parseColorArgs(args)
		p.FillColor = color.SCN(p.FillColor, values, pat)
		s.GState.Set |= graphics.StateFillColor

	case OpSetStrokeGray: // G
		if gray, ok := getNumber(args, 0); ok {
			p.StrokeColor = color.DeviceGray(gray)
			s.GState.Set |= graphics.StateStrokeColor
		}

	case OpSetFillGray: // g
		if gray, ok := getNumber(args, 0); ok {
			p.FillColor = color.DeviceGray(gray)
			s.GState.Set |= graphics.StateFillColor
		}

	case OpSetStrokeRGB: // RG
		r, ok1 := getNumber(args, 0)
		g, ok2 := getNumber(args, 1)
		b, ok3 := getNumber(args, 2)
		if ok1 && ok2 && ok3 {
			p.StrokeColor = color.DeviceRGB{r, g, b}
			s.GState.Set |= graphics.StateStrokeColor
		}

	case OpSetFillRGB: // rg
		r, ok1 := getNumber(args, 0)
		g, ok2 := getNumber(args, 1)
		b, ok3 := getNumber(args, 2)
		if ok1 && ok2 && ok3 {
			p.FillColor = color.DeviceRGB{r, g, b}
			s.GState.Set |= graphics.StateFillColor
		}

	case OpSetStrokeCMYK: // K
		c, ok1 := getNumber(args, 0)
		m, ok2 := getNumber(args, 1)
		y, ok3 := getNumber(args, 2)
		k, ok4 := getNumber(args, 3)
		if ok1 && ok2 && ok3 && ok4 {
			p.StrokeColor = color.DeviceCMYK{c, m, y, k}
			s.GState.Set |= graphics.StateStrokeColor
		}

	case OpSetFillCMYK: // k
		c, ok1 := getNumber(args, 0)
		m, ok2 := getNumber(args, 1)
		y, ok3 := getNumber(args, 2)
		k, ok4 := getNumber(args, 3)
		if ok1 && ok2 && ok3 && ok4 {
			p.FillColor = color.DeviceCMYK{c, m, y, k}
			s.GState.Set |= graphics.StateFillColor
		}
	}
}

// getColorSpace returns the color space for the given name.
func (s *State) getColorSpace(name pdf.Name) color.Space {
	switch name {
	case "DeviceGray":
		return color.SpaceDeviceGray
	case "DeviceRGB":
		return color.SpaceDeviceRGB
	case "DeviceCMYK":
		return color.SpaceDeviceCMYK
	case "Pattern":
		return color.SpacePatternColored
	default:
		if s.Resources != nil && s.Resources.ColorSpace != nil {
			return s.Resources.ColorSpace[name]
		}
		return nil
	}
}

// parseColorArgs extracts color values and optional pattern from operator arguments.
func (s *State) parseColorArgs(args []pdf.Object) ([]float64, color.Pattern) {
	var values []float64
	var pat color.Pattern
	for _, a := range args {
		switch a := a.(type) {
		case pdf.Integer:
			values = append(values, float64(a))
		case pdf.Real:
			values = append(values, float64(a))
		case pdf.Number:
			values = append(values, float64(a))
		case pdf.Name:
			if s.Resources != nil && s.Resources.Pattern != nil {
				pat = s.Resources.Pattern[a]
			}
		}
	}
	return values, pat
}

// Helper functions for extracting values from operator arguments

// getNumber extracts a number from the argument slice at the given index.
func getNumber(args []pdf.Object, idx int) (float64, bool) {
	if idx >= len(args) {
		return 0, false
	}
	switch v := args[idx].(type) {
	case pdf.Real:
		return float64(v), true
	case pdf.Integer:
		return float64(v), true
	case pdf.Number:
		return float64(v), true
	default:
		return 0, false
	}
}

// getInteger extracts an integer from the argument slice at the given index.
func getInteger(args []pdf.Object, idx int) (int64, bool) {
	if idx >= len(args) {
		return 0, false
	}
	switch v := args[idx].(type) {
	case pdf.Integer:
		return int64(v), true
	case pdf.Real:
		return int64(v), true
	case pdf.Number:
		return int64(v), true
	default:
		return 0, false
	}
}

// getName extracts a name from the argument slice at the given index.
func getName(args []pdf.Object, idx int) (pdf.Name, bool) {
	if idx >= len(args) {
		return "", false
	}
	if name, ok := args[idx].(pdf.Name); ok {
		return name, true
	}
	return "", false
}

// getMatrix extracts a 6-element matrix from operator arguments.
func getMatrix(args []pdf.Object) (matrix.Matrix, bool) {
	if len(args) < 6 {
		return matrix.Matrix{}, false
	}
	var m matrix.Matrix
	for i := range 6 {
		v, ok := getNumber(args, i)
		if !ok {
			return matrix.Matrix{}, false
		}
		m[i] = v
	}
	return m, true
}

// convertDashPattern converts a PDF array to a dash pattern slice.
func convertDashPattern(dashPattern pdf.Array) ([]float64, bool) {
	if dashPattern == nil {
		return nil, true
	}
	pat := make([]float64, len(dashPattern))
	for i, obj := range dashPattern {
		switch v := obj.(type) {
		case pdf.Real:
			pat[i] = float64(v)
		case pdf.Integer:
			pat[i] = float64(v)
		case pdf.Number:
			pat[i] = float64(v)
		default:
			return nil, false
		}
	}
	return pat, true
}
