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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
)

// A Reader reads a PDF content stream.
type Reader struct {
	R         pdf.Getter
	Resources *pdf.Resources
	State
	stack []State
}

// UpdateState updates the graphics state according to the given operator and
// arguments.
func (r *Reader) UpdateState(op string, args []pdf.Object) error {
	getInteger := func() (pdf.Integer, bool) {
		if len(args) == 0 {
			return 0, false
		}
		x, ok := args[0].(pdf.Integer)
		args = args[1:]
		return x, ok
	}
	getNum := func() (float64, bool) {
		if len(args) == 0 {
			return 0, false
		}
		x, ok := getNumber(args[0])
		args = args[1:]
		return x, ok
	}
	getName := func() (pdf.Name, bool) {
		if len(args) == 0 {
			return "", false
		}
		x, ok := args[0].(pdf.Name)
		args = args[1:]
		return x, ok
	}
	getArray := func() (pdf.Array, bool) {
		if len(args) == 0 {
			return nil, false
		}
		x, ok := args[0].(pdf.Array)
		args = args[1:]
		return x, ok
	}

	// Operators are listed in the order of table 50 ("Operator categories") in
	// ISO 32000-2:2020.

doOps:
	switch op {

	// == General graphics state =========================================

	case "w": // line width
		x, ok := getNum()
		if ok {
			r.LineWidth = float64(x)
			r.Set |= StateLineWidth
		}

	case "J": // line cap style
		x, ok := getInteger()
		if ok {
			if LineCapStyle(x) > 2 {
				x = 0
			}
			r.LineCap = LineCapStyle(x)
			r.Set |= StateLineCap
		}

	case "j": // line join style
		x, ok := getInteger()
		if ok {
			if LineJoinStyle(x) > 2 {
				x = 0
			}
			r.LineJoin = LineJoinStyle(x)
			r.Set |= StateLineJoin
		}

	case "M": // miter limit
		x, ok := getNum()
		if ok {
			r.MiterLimit = float64(x)
			r.Set |= StateMiterLimit
		}

	case "d": // dash pattern and phase
		patObj, ok1 := getArray()
		pattern, ok2 := convertDashPattern(patObj)
		phase, ok3 := getNum()
		if ok1 && ok2 && ok3 {
			r.DashPattern = pattern
			r.DashPhase = phase
			r.Set |= StateDash
		}

	case "ri": // rendering intent
		name, ok := getName()
		if ok {
			r.RenderingIntent = name
			r.Set |= StateRenderingIntent
		}

	case "i": // flatness tolerance
		x, ok := getNum()
		if ok {
			r.FlatnessTolerance = float64(x)
			r.Set |= StateFlatnessTolerance
		}

	case "gs": // Set parameters from graphics state parameter dictionary
		name, ok := getName()
		if ok {
			// TODO(voss): use caching
			newState, err := ReadExtGState(r.R, r.Resources.ExtGState[name], name)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return err
			}
			newState.ApplyTo(&r.State)
		}

	case "q":
		r.stack = append(r.stack, State{
			Parameters: r.Parameters.Clone(),
			Set:        r.Set,
		})

	case "Q":
		if len(r.stack) > 0 {
			r.State = r.stack[len(r.stack)-1]
			r.stack = r.stack[:len(r.stack)-1]
		}

	// == Special graphics state =========================================

	case "cm":
		m := Matrix{}
		for i := 0; i < 6; i++ {
			f, ok := getNum()
			if !ok {
				break doOps
			}
			m[i] = float64(f)
		}
		r.CTM = r.CTM.Mul(m) // TODO(voss): correct order?

	// == Text objects ===================================================

	case "BT": // Begin text object
		r.TextMatrix = IdentityMatrix
		r.TextLineMatrix = IdentityMatrix

	case "ET": // End text object

	// == Text state =====================================================

	case "Tc": // Set character spacing
		x, ok := getNum()
		if ok {
			r.TextCharacterSpacing = x
			r.Set |= StateTextCharacterSpacing
		}

	case "Tw": // Set word spacing
		x, ok := getNum()
		if ok {
			r.TextWordSpacing = x
			r.Set |= StateTextWordSpacing
		}

	case "Tz": // Set the horizontal scaling
		x, ok := getNum()
		if ok {
			r.TextHorizonalScaling = x
			r.Set |= StateTextHorizontalSpacing
		}

	case "TL": // Set the leading
		x, ok := getNum()
		if ok {
			r.TextLeading = x
			r.Set |= StateTextLeading
		}

	case "Tf": // Set text font and size
		name, ok1 := getName()
		size, ok2 := getNum()
		if ok1 && ok2 {
			var obj pdf.Object
			if r.Resources.Font != nil {
				obj = r.Resources.Font[name]
			}
			ref, _ := obj.(pdf.Reference) // TODO(voss): think about this
			r.TextFont = &Res{
				DefName: name,
				Data:    ref,
			}
			r.TextFontSize = size
			r.Set |= StateTextFont
		}

	case "Tr": // text rendering mode
		x, ok := getInteger()
		if ok {
			r.TextRenderingMode = TextRenderingMode(x)
			r.Set |= StateTextRenderingMode
		}

	case "Ts": // Set text rise
		x, ok := getNum()
		if ok {
			r.TextRise = x
			r.Set |= StateTextRise
		}

	// == Text positioning ===============================================

	case "Td": // Move text position
		dx, ok1 := getNum()
		dy, ok2 := getNum()
		if ok1 && ok2 {
			r.TextLineMatrix = Translate(dx, dy).Mul(r.TextLineMatrix)
			r.TextMatrix = r.TextLineMatrix
			if r.Set&StateTextLineMatrix != 0 {
				r.Set |= StateTextMatrix
			}
		}

	case "TD": // Move text position and set leading
		dx, ok1 := getNum()
		dy, ok2 := getNum()
		if ok1 && ok2 {
			r.TextLeading = -dy
			r.Set |= StateTextLeading
			r.TextLineMatrix = Translate(dx, dy).Mul(r.TextLineMatrix)
			r.TextMatrix = r.TextLineMatrix
			if r.Set&StateTextLineMatrix != 0 {
				r.Set |= StateTextMatrix
			}
		}

	case "Tm": // Set text matrix and text line matrix
		m := Matrix{}
		for i := 0; i < 6; i++ {
			f, ok := getNum()
			if !ok {
				break doOps
			}
			m[i] = float64(f)
		}
		r.TextMatrix = m
		r.TextLineMatrix = m
		r.Set |= StateTextMatrix | StateTextLineMatrix

	case "T*": // Move to start of next text line
		r.TextLineMatrix = Translate(0, -r.TextLeading).Mul(r.TextLineMatrix)
		r.TextMatrix = r.TextLineMatrix
		if r.Set&StateTextLineMatrix != 0 {
			r.Set |= StateTextMatrix
		}

	// == Text showing ===================================================

	case "Tj": // Show text
		// TODO(voss): update g.Tm

	case "TJ": // Show text with kerning
		// TODO(voss): update g.Tm

	// == Type 3 fonts ===================================================

	// == Color ==========================================================

	case "G": // stroking gray level
		if len(args) < 1 {
			break
		}
		gray, ok := getNumber(args[0])
		if !ok {
			break
		}
		r.StrokeColor = color.Gray(gray)
		r.Set |= StateStrokeColor

	case "g": // nonstroking gray level
		if len(args) < 1 {
			break
		}
		gray, ok := getNumber(args[0])
		if !ok {
			break
		}
		r.FillColor = color.Gray(gray)
		r.Set |= StateFillColor

	case "RG": // nonstroking DeviceRGB color
		if len(args) < 3 {
			break
		}
		var red, green, blue float64
		var ok bool
		if red, ok = getNumber(args[0]); !ok {
			break
		}
		if green, ok = getNumber(args[1]); !ok {
			break
		}
		if blue, ok = getNumber(args[2]); !ok {
			break
		}
		r.StrokeColor = color.RGB(red, green, blue)

	case "rg": // nonstroking DeviceRGB color
		if len(args) < 3 {
			break
		}
		var red, green, blue float64
		var ok bool
		if red, ok = getNumber(args[0]); !ok {
			break
		}
		if green, ok = getNumber(args[1]); !ok {
			break
		}
		if blue, ok = getNumber(args[2]); !ok {
			break
		}
		r.FillColor = color.RGB(red, green, blue)

	case "K": // stroking DeviceCMYK color
		if len(args) < 4 {
			break
		}
		var cyan, magenta, yellow, black float64
		var ok bool
		if cyan, ok = getNumber(args[0]); !ok {
			break
		}
		if magenta, ok = getNumber(args[1]); !ok {
			break
		}
		if yellow, ok = getNumber(args[2]); !ok {
			break
		}
		if black, ok = getNumber(args[3]); !ok {
			break
		}
		r.StrokeColor = color.CMYK(cyan, magenta, yellow, black)

	case "k": // nonstroking DeviceCMYK color
		if len(args) < 4 {
			break
		}
		var cyan, magenta, yellow, black float64
		var ok bool
		if cyan, ok = getNumber(args[0]); !ok {
			break
		}
		if magenta, ok = getNumber(args[1]); !ok {
			break
		}
		if yellow, ok = getNumber(args[2]); !ok {
			break
		}
		if black, ok = getNumber(args[3]); !ok {
			break
		}
		r.FillColor = color.CMYK(cyan, magenta, yellow, black)

	// == Shading patterns ===============================================

	// == Inline images ==================================================

	// == XObjects =======================================================

	// == Marked content =================================================

	case "BMC": // Begin marked-content sequence
		if len(args) < 1 {
			break
		}
		name, ok := args[0].(pdf.Name)
		if !ok {
			break
		}
		_ = name

	case "BDC": // Begin marked-content sequence with property list
		if len(args) < 2 {
			break
		}
		name, ok := args[0].(pdf.Name)
		if !ok {
			break
		}
		var dict pdf.Dict
		switch a := args[1].(type) {
		case pdf.Dict:
			dict = a
		case pdf.Name:
			var err error
			dict, err = pdf.GetDict(r.R, r.Resources.Properties[a])
			if err != nil {
				break
			}
		default:
			break
		}

		_ = name
		_ = dict

	case "EMC": // End marked-content sequence

		// == Compatibility ===================================================

	}
	return nil
}

func getNumber(x pdf.Object) (float64, bool) {
	switch x := x.(type) {
	case pdf.Real:
		return float64(x), true
	case pdf.Integer:
		return float64(x), true
	case pdf.Number:
		return float64(x), true
	default:
		return 0, false
	}
}

func convertDashPattern(dashPattern pdf.Array) (pat []float64, ok bool) {
	if dashPattern == nil {
		return nil, true
	}
	pat = make([]float64, len(dashPattern))
	for i, obj := range dashPattern {
		x, ok := getNumber(obj)
		if !ok {
			return nil, false
		}
		pat[i] = float64(x)
	}
	return pat, true
}
