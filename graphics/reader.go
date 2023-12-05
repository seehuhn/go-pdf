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
	// Operators are listed in the order of table 50 ("Operator categories") in
	// ISO 32000-2:2020.

	switch op {

	// == General graphics state =========================================

	case "w": // line width
		if len(args) < 1 {
			break
		}
		x, ok := getNumber(args[0])
		if !ok {
			break
		}
		r.LineWidth = float64(x)
		r.Set |= StateLineWidth

	case "J": // line cap style
		if len(args) < 1 {
			break
		}
		x, ok := args[0].(pdf.Integer)
		if !ok {
			break
		}
		x = min(max(x, 0), 2)
		r.LineCap = LineCapStyle(x)
		r.Set |= StateLineCap

	case "j": // line join style
		if len(args) < 1 {
			break
		}
		x, ok := args[0].(pdf.Integer)
		if !ok {
			break
		}
		x = min(max(x, 0), 2)
		r.LineJoin = LineJoinStyle(x)
		r.Set |= StateLineJoin

	case "M": // miter limit
		if len(args) < 1 {
			break
		}
		x, ok := getNumber(args[0])
		if !ok {
			break
		}
		r.MiterLimit = float64(x)
		r.Set |= StateMiterLimit

	case "d": // dash pattern and phase
		if len(args) < 2 {
			break
		}
		pattern, phase, ok := readDashDirect(args[0], args[1])
		if !ok {
			break
		}
		r.DashPattern = pattern
		r.DashPhase = phase
		r.Set |= StateDash

	case "ri": // rendering intent
		if len(args) < 1 {
			break
		}
		name, ok := args[0].(pdf.Name)
		if !ok {
			break
		}
		r.RenderingIntent = name
		r.Set |= StateRenderingIntent

	case "i": // flatness tolerance
		if len(args) < 1 {
			break
		}
		x, ok := getNumber(args[0])
		if !ok {
			break
		}
		r.FlatnessTolerance = float64(x)
		r.Set |= StateFlatnessTolerance

	case "gs": // Set parameters from graphics state parameter dictionary
		if len(args) < 1 {
			break
		}
		name, ok := args[0].(pdf.Name)
		if !ok {
			break
		}

		// TODO(voss): use caching
		newState, err := ReadExtGState(r.R, r.Resources.ExtGState[name], name)
		if pdf.IsMalformed(err) {
			break
		} else if err != nil {
			return err
		}
		newState.ApplyTo(&r.State)

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
		if len(args) < 6 {
			break
		}
		m := Matrix{}
		for i := 0; i < 6; i++ {
			f, err := pdf.GetNumber(r.R, args[i])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return err
			}
			m[i] = float64(f)
		}
		r.CTM = r.CTM.Mul(m) // TODO(voss): correct order?

	// == Path construction ==============================================

	case "m": // Begin new subpath
		if len(args) < 2 {
			break
		}
		x, ok1 := getNumber(args[0])
		y, ok2 := getNumber(args[1])
		if !ok1 || !ok2 {
			break
		}
		_ = x
		_ = y

	case "l": // Append straight line segment to path
		if len(args) < 2 {
			break
		}
		x, ok1 := getNumber(args[0])
		y, ok2 := getNumber(args[1])
		if !ok1 || !ok2 {
			break
		}
		_ = x
		_ = y

	case "c": // Append cubic Bezier curve to path
		if len(args) < 6 {
			break
		}
		x1, ok1 := getNumber(args[0])
		y1, ok2 := getNumber(args[1])
		x2, ok3 := getNumber(args[2])
		y2, ok4 := getNumber(args[3])
		x3, ok5 := getNumber(args[4])
		y3, ok6 := getNumber(args[5])
		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
			break
		}
		_, _, _, _, _, _ = x1, y1, x2, y2, x3, y3

	case "h": // Close subpath

	case "re": // Append rectangle to path
		if len(args) < 4 {
			break
		}
		x, ok1 := getNumber(args[0])
		y, ok2 := getNumber(args[1])
		w, ok3 := getNumber(args[2])
		h, ok4 := getNumber(args[3])
		if !ok1 || !ok2 || !ok3 || !ok4 {
			break
		}
		_, _, _, _ = x, y, w, h

	// == Path painting ==================================================

	case "S": // Stroke path

	case "s": // Close and stroke path

	case "f": // Fill path using nonzero winding number rule

	case "f*": // Fill path using even-odd rule

	case "n": // End path without filling or stroking

	// == Clipping paths =================================================

	case "W": // Modify clipping path by intersecting with current path

	case "W*": // Modify clipping path by intersecting with current path, using even-odd rule

	// == Text objects ===================================================

	case "BT": // Begin text object
		r.TextMatrix = IdentityMatrix
		r.TextLineMatrix = IdentityMatrix

	case "ET": // End text object

	// == Text state =====================================================

	case "Tc": // Set character spacing
		if len(args) < 1 {
			break
		}
		Tc, ok := getNumber(args[0])
		if !ok {
			break
		}
		r.TextCharacterSpacing = Tc
		r.Set |= StateTextCharacterSpacing

	case "Tw": // Set word spacing
		if len(args) < 1 {
			break
		}
		Tw, ok := getNumber(args[0])
		if !ok {
			break
		}
		r.TextWordSpacing = Tw
		r.Set |= StateTextWordSpacing

	case "Tz": // Set the horizontal scaling
		if len(args) < 1 {
			break
		}
		Th, ok := getNumber(args[0])
		if !ok {
			break
		}
		r.TextHorizonalScaling = Th
		r.Set |= StateTextHorizontalSpacing

	case "TL": // Set the leading
		if len(args) < 1 {
			break
		}
		leading, ok := getNumber(args[0])
		if !ok {
			break
		}
		r.TextLeading = leading
		r.Set |= StateTextLeading

	case "Tf": // Set text font and size
		if len(args) < 2 {
			break
		}
		name, ok1 := args[0].(pdf.Name)
		size, ok2 := getNumber(args[1])
		if !ok1 || !ok2 {
			break
		}
		var obj pdf.Object
		if r.Resources.Font != nil {
			obj = r.Resources.Font[name]
		}
		ref, _ := obj.(pdf.Reference)
		r.TextFont = &Res{
			DefName: name,
			Data:    ref,
		}
		r.TextFontSize = size
		r.Set |= StateTextFont

	case "Tr": // text rendering mode
		if len(args) < 1 {
			break
		}
		mode, ok := args[0].(pdf.Integer)
		if !ok {
			break
		}
		r.TextRenderingMode = TextRenderingMode(mode)
		r.Set |= StateTextRenderingMode

	case "Ts": // Set text rise
		if len(args) < 1 {
			break
		}
		rise, ok := getNumber(args[0])
		if !ok {
			break
		}
		r.TextRise = rise
		r.Set |= StateTextRise

	// == Text positioning ===============================================

	case "Td": // Move text position
		if len(args) < 2 {
			break
		}
		tx, ok1 := getNumber(args[0])
		ty, ok2 := getNumber(args[1])
		if !ok1 || !ok2 {
			break
		}

		r.TextLineMatrix = Matrix{1, 0, 0, 1, tx, ty}.Mul(r.TextLineMatrix)
		r.TextMatrix = r.TextLineMatrix

	case "Tm": // Set text matrix and text line matrix
		if len(args) < 6 {
			break
		}
		var data Matrix
		for i := 0; i < 6; i++ {
			x, ok := getNumber(args[i])
			if !ok {
				break
			}
			data[i] = x
		}
		r.TextMatrix = data
		r.TextLineMatrix = data

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

	case "g": // nonstroking gray level
		if len(args) < 1 {
			break
		}
		gray, ok := getNumber(args[0])
		if !ok {
			break
		}
		r.FillColor = color.Gray(gray)

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

func readDashDirect(patObj, phaseObj pdf.Object) (pat []float64, ph float64, ok bool) {
	dashPattern, ok := patObj.(pdf.Array)
	if !ok {
		return nil, 0, false
	}
	phase, ok := getNumber(phaseObj)
	if !ok {
		return nil, 0, false
	}
	pat = make([]float64, len(dashPattern))
	for i, obj := range dashPattern {
		x, ok := getNumber(obj)
		if !ok {
			return nil, 0, false
		}
		pat[i] = float64(x)
	}
	return pat, phase, true
}
