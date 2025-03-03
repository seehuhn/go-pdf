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
	"fmt"
	"io"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/reader/scanner"
)

// A Reader reads a PDF content stream.
type Reader struct {
	R      pdf.Getter
	loader *loader.FontLoader

	scanner *scanner.Scanner

	fontCache map[pdf.Reference]FontFromFile

	Resources *pdf.Resources
	graphics.State
	stack []graphics.State

	// User callbacks
	DrawGlyph func(g font.Glyph) error
	Text      func(text string) error
	UnknownOp func(op string, args []pdf.Object) error
	EveryOp   func(op string, args []pdf.Object) error
}

// New creates a new Reader.
func New(r pdf.Getter, loader *loader.FontLoader) *Reader {
	return &Reader{
		R:      r,
		loader: loader,

		scanner: scanner.NewScanner(),

		fontCache: make(map[pdf.Reference]FontFromFile),
	}
}

// Reset resets the reader to its initial state.
// This should be used before parsing a new page.
func (r *Reader) Reset() {
	r.scanner.Reset()
	r.Resources = &pdf.Resources{}
	r.State = graphics.NewState()
	r.stack = r.stack[:0]
}

// ParsePage parses a page, and calls the appropriate callback functions.
func (r *Reader) ParsePage(page pdf.Object, ctm matrix.Matrix) error {
	pageDict, err := pdf.GetDictTyped(r.R, page, "Page")
	if err != nil {
		return err
	}

	r.Reset()
	r.State.CTM = ctm

	// TODO(voss): do we need to worry about inherited resources? There is some
	// code in seehuhn.de/go/pdf/pagetree that copies inherited resources from
	// the parent, but this needs to be checked and documented.  Also, it
	// reduces generality of the ParsePage method.
	resourcesDict, err := pdf.GetDict(r.R, pageDict["Resources"])
	if err != nil {
		return err
	}
	err = pdf.DecodeDict(r.R, r.Resources, resourcesDict)
	if err != nil {
		return err
	}

	return r.parseContents(pageDict["Contents"])
}

// parseContents parses a content stream.
// Obj can be either a stream or an array of streams.
func (r *Reader) parseContents(obj pdf.Object) error {
	contents, err := pdf.Resolve(r.R, obj)
	if err != nil {
		return err
	}
	switch contents := contents.(type) {
	case *pdf.Stream:
		err := r.parsePDFStream(contents)
		if err != nil && err != io.ErrUnexpectedEOF {
			// TODO(voss): add reference to the message, as it is done below
			return pdf.Wrap(err, "content stream")
		}
	case pdf.Array:
		for _, ref := range contents {
			stm, err := pdf.GetStream(r.R, ref)
			if err != nil {
				return err
			}
			err = r.parsePDFStream(stm)
			if err != nil && err != io.ErrUnexpectedEOF {
				key := "content stream"
				if ref, ok := ref.(pdf.Reference); ok {
					key = fmt.Sprintf("content stream %s", ref)
				}
				return pdf.Wrap(err, key)
			}
		}
	default:
		return &pdf.MalformedFileError{
			Err: fmt.Errorf("unexpected type %T for content stream", contents),
		}
	}
	return nil
}

func (r *Reader) parsePDFStream(stm *pdf.Stream) error {
	body, err := pdf.DecodeStream(r.R, stm, 0)
	if err != nil {
		return err
	}
	return r.ParseContentStream(body)
}

// ParseContentStream parses a PDF content stream.
func (r *Reader) ParseContentStream(in io.Reader) error {
	r.scanner.SetInput(in)
	err := r.do()
	if err != nil {
		return err
	}
	return r.scanner.Error()
}

func (r *Reader) do() error {
	for r.scanner.Scan() {
		op := r.scanner.Operator()
		origArgs := op.Args

	cmdSwitch:
		switch op.Name {

		// Table 56 – Graphics state operators

		case "q":
			if op.OK() && len(r.stack) < maxGraphicsStackDepth {
				r.stack = append(r.stack, graphics.State{
					Parameters: r.Parameters.Clone(),
					Set:        r.Set,
				})
			}

		case "Q":
			if op.OK() && len(r.stack) > 0 {
				r.State = r.stack[len(r.stack)-1]
				r.stack = r.stack[:len(r.stack)-1]
			}

		case "cm":
			m := matrix.Matrix{}
			for i := 0; i < 6; i++ {
				m[i] = op.GetNumber()
			}
			if op.OK() {
				// TODO(voss): correct order?  Add unit tests.
				r.CTM = r.CTM.Mul(m)
			}

		case "w": // line width
			lineWidth := op.GetNumber()
			if op.OK() {
				r.LineWidth = lineWidth
				r.Set |= graphics.StateLineWidth
			}

		case "J": // line cap style
			lineCap := op.GetInteger()
			if op.OK() {
				if lineCap < 0 {
					lineCap = 0
				} else if lineCap > 2 {
					lineCap = 2
				}
				r.LineCap = graphics.LineCapStyle(lineCap)
				r.Set |= graphics.StateLineCap
			}

		case "j": // line join style
			lineJoin := op.GetInteger()
			if op.OK() {
				if lineJoin < 0 {
					lineJoin = 0
				} else if lineJoin > 2 {
					lineJoin = 2
				}
				r.LineJoin = graphics.LineJoinStyle(lineJoin)
				r.Set |= graphics.StateLineJoin
			}

		case "M": // miter limit
			miterLimit := op.GetNumber()
			if op.OK() {
				if miterLimit < 1 {
					miterLimit = 1
				}
				r.MiterLimit = miterLimit
				r.Set |= graphics.StateMiterLimit
			}

		case "d": // dash pattern and phase
			pat := op.GetArray()
			phase := op.GetNumber()
			if op.OK() {
				pat, ok := convertDashPattern(pat)
				if ok {
					r.DashPattern = pat
					r.DashPhase = phase
					r.Set |= graphics.StateLineDash
				}
			}

		case "ri": // rendering intent
			intent := op.GetName()
			if op.OK() {
				r.RenderingIntent = graphics.RenderingIntent(intent)
				r.Set |= graphics.StateRenderingIntent
			}

		case "i": // flatness tolerance
			flatness := op.GetNumber()
			if op.OK() {
				if flatness < 0 {
					flatness = 0
				} else if flatness > 100 {
					flatness = 100
				}
				r.FlatnessTolerance = flatness
				r.Set |= graphics.StateFlatnessTolerance
			}

		case "gs": // extGState
			dictName := op.GetName()
			if op.OK() {
				// TODO(voss): use caching
				newState, err := r.readExtGState(r.Resources.ExtGState[dictName])
				if err == nil {
					newState.CopyTo(&r.State)
				} else if !pdf.IsMalformed(err) {
					return err
				}
			}

		// Table 105 - Text object operators

		case "BT":
			if op.OK() {
				r.TextMatrix = matrix.Identity
				r.TextLineMatrix = matrix.Identity
				r.Set |= graphics.StateTextMatrix
			}

		case "ET":
			if op.OK() {
				r.Set &= ^graphics.StateTextMatrix
			}

		// Table 103 - Text state operators

		case "Tc":
			charSpace := op.GetNumber()
			if op.OK() {
				r.TextCharacterSpacing = charSpace
				r.Set |= graphics.StateTextCharacterSpacing
			}

		case "Tw":
			wordSpace := op.GetNumber()
			if op.OK() {
				r.TextWordSpacing = wordSpace
				r.Set |= graphics.StateTextWordSpacing
			}

		case "Tz":
			scale := op.GetNumber()
			if op.OK() {
				r.TextHorizontalScaling = scale / 100
				r.Set |= graphics.StateTextHorizontalScaling
			}

		case "TL":
			leading := op.GetNumber()
			if op.OK() {
				r.TextLeading = leading
				r.Set |= graphics.StateTextLeading
			}

		case "Tf":
			font := op.GetName()
			size := op.GetNumber()
			if op.OK() && r.Resources != nil && r.Resources.Font != nil {
				if ref := r.Resources.Font[font]; ref != nil {
					F, err := r.ReadFont(ref)
					if pdf.IsMalformed(err) {
						break
					} else if err != nil {
						return pdf.Wrap(err, fmt.Sprintf("font %s", font))
					}
					r.TextFont = F
					r.TextFontSize = size
					r.Set |= graphics.StateTextFont
				}
			}

		case "Tr":
			render := op.GetInteger()
			if op.OK() {
				if render < 0 {
					render = 0
				} else if render > 7 {
					render = 7
				}
				r.TextRenderingMode = graphics.TextRenderingMode(render)
				r.Set |= graphics.StateTextRenderingMode
			}

		case "Ts":
			rise := op.GetNumber()
			if op.OK() {
				r.TextRise = rise
				r.Set |= graphics.StateTextRise
			}

		// Table 106 - Text-positioning operators

		case "Td":
			tx := op.GetNumber()
			ty := op.GetNumber()
			if op.OK() {
				r.TextLineMatrix = matrix.Translate(tx, ty).Mul(r.TextLineMatrix)
				r.TextMatrix = r.TextLineMatrix
			}

		case "TD":
			tx := op.GetNumber()
			ty := op.GetNumber()
			if op.OK() {
				r.TextLeading = -ty
				r.Set |= graphics.StateTextLeading
				r.TextLineMatrix = matrix.Translate(tx, ty).Mul(r.TextLineMatrix)
				r.TextMatrix = r.TextLineMatrix
			}

		case "Tm":
			m := matrix.Matrix{}
			for i := 0; i < 6; i++ {
				m[i] = op.GetNumber()
			}
			if op.OK() {
				r.TextMatrix = m
				r.TextLineMatrix = m
				r.Set |= graphics.StateTextMatrix
			}

		case "T*":
			if op.OK() {
				r.TextLineMatrix = matrix.Translate(0, -r.TextLeading).Mul(r.TextLineMatrix)
				r.TextMatrix = r.TextLineMatrix
			}

		// Table 107 - Text-showing operators

		case "Tj":
			s := op.GetString()
			if op.OK() {
				r.processText(s)
			}

		case "'":
			s := op.GetString()
			if op.OK() {
				r.TextLineMatrix = matrix.Translate(0, -r.TextLeading).Mul(r.TextLineMatrix)
				r.TextMatrix = r.TextLineMatrix
				r.processText(s)
			}

		case "\"":
			aw := op.GetNumber()
			ac := op.GetNumber()
			s := op.GetString()
			if op.OK() {
				r.TextWordSpacing = aw
				r.TextCharacterSpacing = ac
				r.Set |= graphics.StateTextWordSpacing | graphics.StateTextCharacterSpacing
				r.processText(s)
			}

		case "TJ":
			a := op.GetArray()
			if op.OK() {
				for _, ai := range a {
					var d float64
					switch ai := ai.(type) {
					case pdf.String:
						r.processText(ai)
					case pdf.Integer:
						d = float64(ai)
					case pdf.Real:
						d = float64(ai)
					case pdf.Number:
						d = float64(ai)
					}
					if d != 0 {
						d = d / 1000 * r.TextFontSize
						switch r.TextFont.WritingMode() {
						case 0:
							r.TextMatrix = matrix.Translate(-d*r.TextHorizontalScaling, 0).Mul(r.TextMatrix)
						case 1:
							r.TextMatrix = matrix.Translate(0, -d).Mul(r.TextMatrix)
						}
					}
				}
			}

		// Table 111 - Type 3 font operators

		case "d0":
			wx := op.GetNumber()
			wy := op.GetNumber()
			if op.OK() {
				// TODO(voss): implement this
				_, _ = wx, wy
			}

		case "d1":
			wx := op.GetNumber()
			wy := op.GetNumber()
			llx := op.GetNumber()
			lly := op.GetNumber()
			urx := op.GetNumber()
			ury := op.GetNumber()
			if op.OK() {
				// TODO(voss): implement this
				_, _, _, _, _, _ = wx, wy, llx, lly, urx, ury
			}

		// Table 73 — Colour operators

		case "CS", "cs":
			name := op.GetName()
			if !op.OK() {
				break
			}

			var csDesc pdf.Object
			if name == "DeviceGray" || name == "DeviceRGB" || name == "DeviceCMYK" || name == "Pattern" {
				csDesc = name
			} else {
				if r.Resources == nil || r.Resources.ColorSpace == nil {
					break
				}
				csDesc = r.Resources.ColorSpace[name]
			}
			cs, err := color.ExtractSpace(r.R, csDesc)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return err
			}

			if op.Name == "CS" {
				r.StrokeColor = cs.Default()
				r.Set |= graphics.StateStrokeColor
			} else {
				r.FillColor = cs.Default()
				r.Set |= graphics.StateFillColor
			}

		case "SC", "SCN", "sc", "scn":
			var values []float64
			var pat color.Pattern
		argLoop:
			for len(op.Args) > 0 {
				a := op.Args[0]
				op.Args = op.Args[1:]

				switch a := a.(type) {
				case pdf.Integer:
					values = append(values, float64(a))
				case pdf.Real:
					values = append(values, float64(a))
				case pdf.Number:
					values = append(values, float64(a))
				case pdf.Name:
					if r.Resources != nil && r.Resources.Pattern != nil {
						pattern, err := readPattern(r.R, r.Resources.Pattern[a])
						if pdf.IsMalformed(err) {
							break cmdSwitch
						} else if err != nil {
							return err
						}
						pat = pattern
					}
					break argLoop
				}
			}
			if op.Name == "SC" || op.Name == "SCN" {
				r.StrokeColor = color.SCN(r.StrokeColor, values, pat)
			} else {
				r.FillColor = color.SCN(r.FillColor, values, pat)
			}

		case "G":
			gray := op.GetNumber()
			if op.OK() {
				r.StrokeColor = color.DeviceGray(gray)
				r.Set |= graphics.StateStrokeColor
			}

		case "g":
			gray := op.GetNumber()
			if op.OK() {
				r.FillColor = color.DeviceGray(gray)
				r.Set |= graphics.StateFillColor
			}

		case "RG":
			red, green, blue := op.GetNumber(), op.GetNumber(), op.GetNumber()
			if op.OK() {
				r.StrokeColor = color.DeviceRGB(red, green, blue)
				r.Set |= graphics.StateStrokeColor
			}

		case "rg":
			red, green, blue := op.GetNumber(), op.GetNumber(), op.GetNumber()
			if op.OK() {
				r.FillColor = color.DeviceRGB(red, green, blue)
				r.Set |= graphics.StateFillColor
			}

		case "K":
			c, m, y, k := op.GetNumber(), op.GetNumber(), op.GetNumber(), op.GetNumber()
			if op.OK() {
				r.StrokeColor = color.DeviceCMYK(c, m, y, k)
				r.Set |= graphics.StateStrokeColor
			}

		case "k":
			c, m, y, k := op.GetNumber(), op.GetNumber(), op.GetNumber(), op.GetNumber()
			if op.OK() {
				r.FillColor = color.DeviceCMYK(c, m, y, k)
				r.Set |= graphics.StateFillColor
			}

		// Table 76 - Shading operator

		case "sh":
			name := op.GetName()
			if op.OK() {
				_ = name
				// TODO(voss): implement this
			}

		default:
			if r.UnknownOp != nil {
				err := r.UnknownOp(op.Name, op.Args)
				if err != nil {
					return err
				}
			}
		}

		if r.EveryOp != nil {
			err := r.EveryOp(op.Name, origArgs)
			if err != nil {
				return err
			}
		}
	}
	return r.scanner.Error()
}

type toTextSpacer interface {
	ToTextSpace(float64) float64
}

func divideBy1000(x float64) float64 {
	return x / 1000
}

func (r *Reader) processText(s pdf.String) {
	// TODO(voss): can this be merged with the corresponding code in op-text.go?

	var toTextSpace func(float64) float64
	if f, ok := r.TextFont.(toTextSpacer); ok {
		// Type 3 fonts use the font matrix, ...
		toTextSpace = f.ToTextSpace
	} else {
		// ... everybode else divides by 1000.
		toTextSpace = divideBy1000
	}

	wmode := r.TextFont.WritingMode()
	for info := range r.TextFont.Codes(s) {
		width := toTextSpace(info.Width)
		width = width*r.TextFontSize + r.TextCharacterSpacing
		if info.UseWordSpacing {
			width += r.TextWordSpacing
		}
		if wmode == font.Horizontal {
			width *= r.TextHorizontalScaling
		}

		if r.DrawGlyph != nil {
			g := font.Glyph{
				// GID:     gid,
				Advance: width,
				Rise:    r.TextRise,
				Text:    []rune(info.Text),
			}
			r.DrawGlyph(g)
		}
		if r.Text != nil {
			r.Text(info.Text)
		}

		switch wmode {
		case font.Horizontal:
			r.TextMatrix = matrix.Translate(width, 0).Mul(r.TextMatrix)
		case font.Vertical:
			r.TextMatrix = matrix.Translate(0, width).Mul(r.TextMatrix)
		}
	}
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
		pat[i] = x
	}
	return pat, true
}

const (
	maxGraphicsStackDepth = 64
)
