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
	"io"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/property"
	"seehuhn.de/go/pdf/reader/scanner"
)

// A Reader reads a PDF content stream.
type Reader struct {
	R      pdf.Getter
	loader *loader.FontLoader
	x      *pdf.Extractor

	scanner *scanner.Scanner

	fontCache map[pdf.Reference]font.Instance

	Resources *pdf.Resources
	graphics.State
	stack []graphics.State

	// User callbacks.
	// TODO(voss): clean up this list
	Character func(cid cid.CID, text string, width float64) error
	TextEvent func(event TextEvent, arg float64)

	Text      func(text string) error
	UnknownOp func(op string, args []pdf.Object) error
	EveryOp   func(op string, args []pdf.Object) error

	MarkedContent      func(event MarkedContentEvent, mc *graphics.MarkedContent) error
	MarkedContentStack []*graphics.MarkedContent
}

type TextEvent uint8

const (
	TextEventNone TextEvent = iota
	TextEventSpace
	TextEventNL
	TextEventMove
)

type MarkedContentEvent uint8

const (
	MarkedContentPoint MarkedContentEvent = iota
	MarkedContentBegin
	MarkedContentEnd
)

// New creates a new Reader.
func New(r pdf.Getter, loader *loader.FontLoader) *Reader {
	return &Reader{
		R:      r,
		loader: loader,
		x:      pdf.NewExtractor(r),

		scanner: scanner.NewScanner(),

		fontCache:          make(map[pdf.Reference]font.Instance),
		MarkedContentStack: make([]*graphics.MarkedContent, 0, 8),
	}
}

// Reset resets the reader to its initial state.
// This should be used before parsing a new page.
func (r *Reader) Reset() {
	r.scanner.Reset()
	r.Resources = &pdf.Resources{}
	r.State = graphics.NewState()
	r.stack = r.stack[:0]
	r.MarkedContentStack = r.MarkedContentStack[:0]
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
	r.Resources, err = pdf.ExtractResources(r.R, pageDict["Resources"])
	if err != nil {
		return err
	}

	contentReader, err := pagetree.ContentStream(r.R, page)
	if err != nil {
		return err
	}
	return r.ParseContentStream(contentReader)
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
			for i := range 6 {
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
				extGState, err := graphics.ExtractExtGState(r.x, r.Resources.ExtGState[dictName])
				if err == nil {
					extGState.ApplyTo(&r.State)
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
					F, err := r.readFont(ref)
					if pdf.IsMalformed(err) {
						break
					} else if err != nil {
						return err
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
				if r.TextEvent != nil {
					r.TextEvent(TextEventNL, 0)
				}
			}

		case "TD":
			tx := op.GetNumber()
			ty := op.GetNumber()
			if op.OK() {
				r.TextLeading = -ty
				r.Set |= graphics.StateTextLeading
				r.TextLineMatrix = matrix.Translate(tx, ty).Mul(r.TextLineMatrix)
				r.TextMatrix = r.TextLineMatrix
				if r.TextEvent != nil {
					r.TextEvent(TextEventNL, 0)
				}
			}

		case "Tm":
			m := matrix.Matrix{}
			for i := range 6 {
				m[i] = op.GetNumber()
			}
			if op.OK() {
				r.TextMatrix = m
				r.TextLineMatrix = m
				r.Set |= graphics.StateTextMatrix
				if r.TextEvent != nil {
					r.TextEvent(TextEventMove, 0)
				}
			}

		case "T*":
			if op.OK() {
				r.TextLineMatrix = matrix.Translate(0, -r.TextLeading).Mul(r.TextLineMatrix)
				r.TextMatrix = r.TextLineMatrix
				if r.TextEvent != nil {
					r.TextEvent(TextEventNL, 0)
				}
			}

		// Table 107 - Text-showing operators

		case "Tj":
			s := op.GetString()
			if op.OK() && r.TextFont != nil {
				err := r.processText(s)
				if err != nil {
					return err
				}
			}

		case "'":
			s := op.GetString()
			if op.OK() && r.TextFont != nil {
				r.TextLineMatrix = matrix.Translate(0, -r.TextLeading).Mul(r.TextLineMatrix)
				r.TextMatrix = r.TextLineMatrix
				if r.TextEvent != nil {
					r.TextEvent(TextEventNL, 0)
				}
				err := r.processText(s)
				if err != nil {
					return err
				}
			}

		case "\"":
			aw := op.GetNumber()
			ac := op.GetNumber()
			s := op.GetString()
			if op.OK() && r.TextFont != nil {
				r.TextWordSpacing = aw
				r.TextCharacterSpacing = ac
				r.Set |= graphics.StateTextWordSpacing | graphics.StateTextCharacterSpacing
				r.TextLineMatrix = matrix.Translate(0, -r.TextLeading).Mul(r.TextLineMatrix)
				r.TextMatrix = r.TextLineMatrix
				if r.TextEvent != nil {
					r.TextEvent(TextEventNL, 0)
				}
				err := r.processText(s)
				if err != nil {
					return err
				}
			}

		case "TJ":
			a := op.GetArray()
			if op.OK() && r.TextFont != nil {
				for _, ai := range a {
					var d float64
					switch ai := ai.(type) {
					case pdf.String:
						err := r.processText(ai)
						if err != nil {
							return err
						}
					case pdf.Integer:
						d = float64(ai)
					case pdf.Real:
						d = float64(ai)
					case pdf.Number:
						d = float64(ai)
					}
					if d != 0 {
						if d < 0 && r.TextEvent != nil {
							r.TextEvent(TextEventSpace, -d)
						}

						d = d / 1000 * r.TextFontSize
						switch r.TextFont.WritingMode() {
						case font.Horizontal:
							r.TextMatrix = matrix.Translate(-d*r.TextHorizontalScaling, 0).Mul(r.TextMatrix)
						case font.Vertical:
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
			cs, err := color.ExtractSpace(r.x, csDesc)
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
				r.Set |= graphics.StateStrokeColor
			} else {
				r.FillColor = color.SCN(r.FillColor, values, pat)
				r.Set |= graphics.StateFillColor
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
				r.StrokeColor = color.DeviceRGB{red, green, blue}
				r.Set |= graphics.StateStrokeColor
			}

		case "rg":
			red, green, blue := op.GetNumber(), op.GetNumber(), op.GetNumber()
			if op.OK() {
				r.FillColor = color.DeviceRGB{red, green, blue}
				r.Set |= graphics.StateFillColor
			}

		case "K":
			c, m, y, k := op.GetNumber(), op.GetNumber(), op.GetNumber(), op.GetNumber()
			if op.OK() {
				r.StrokeColor = color.DeviceCMYK{c, m, y, k}
				r.Set |= graphics.StateStrokeColor
			}

		case "k":
			c, m, y, k := op.GetNumber(), op.GetNumber(), op.GetNumber(), op.GetNumber()
			if op.OK() {
				r.FillColor = color.DeviceCMYK{c, m, y, k}
				r.Set |= graphics.StateFillColor
			}

		// Table 76 - Shading operator

		case "sh":
			name := op.GetName()
			if op.OK() {
				_ = name
				// TODO(voss): implement this
			}

		// Table 319 - Marked-content operators

		case "MP":
			tag := op.GetName()
			if op.OK() && r.MarkedContent != nil {
				mc := &graphics.MarkedContent{
					Tag:        tag,
					Properties: nil,
				}
				err := r.MarkedContent(MarkedContentPoint, mc)
				if err != nil {
					return err
				}
			}

		case "BMC":
			tag := op.GetName()
			if op.OK() && len(r.MarkedContentStack) < maxMarkedContentDepth {
				mc := &graphics.MarkedContent{
					Tag:        tag,
					Properties: nil,
				}
				r.MarkedContentStack = append(r.MarkedContentStack, mc)
				if r.MarkedContent != nil {
					err := r.MarkedContent(MarkedContentBegin, mc)
					if err != nil {
						return err
					}
				}
			}

		case "DP":
			if len(op.Args) != 2 {
				break
			}
			tag, ok1 := op.Args[0].(pdf.Name)
			if !ok1 {
				break
			}

			mc, err := r.extractMarkedContent(tag, op.Args[1])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return err
			}

			if r.MarkedContent != nil {
				err := r.MarkedContent(MarkedContentPoint, mc)
				if err != nil {
					return err
				}
			}

		case "BDC":
			if len(op.Args) != 2 {
				break
			}

			tag, ok1 := op.Args[0].(pdf.Name)
			if !ok1 || len(r.MarkedContentStack) >= maxMarkedContentDepth {
				break
			}

			mc, err := r.extractMarkedContent(tag, op.Args[1])
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return err
			}

			r.MarkedContentStack = append(r.MarkedContentStack, mc)
			if r.MarkedContent != nil {
				err := r.MarkedContent(MarkedContentBegin, mc)
				if err != nil {
					return err
				}
			}

		case "EMC":
			if op.OK() && len(r.MarkedContentStack) > 0 {
				mc := r.MarkedContentStack[len(r.MarkedContentStack)-1]
				r.MarkedContentStack = r.MarkedContentStack[:len(r.MarkedContentStack)-1]
				if r.MarkedContent != nil {
					err := r.MarkedContent(MarkedContentEnd, mc)
					if err != nil {
						return err
					}
				}
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

// extractMarkedContent extracts marked content properties from operator arguments.
// It returns the MarkedContent struct or an error if extraction fails.
func (r *Reader) extractMarkedContent(tag pdf.Name, propArg pdf.Object) (*graphics.MarkedContent, error) {
	var propObj pdf.Object
	var inline bool
	if name, ok := propArg.(pdf.Name); ok {
		if r.Resources != nil && r.Resources.Properties != nil {
			propObj = r.Resources.Properties[name]
		}
		inline = false
	} else {
		propObj = propArg
		inline = true
	}

	var list property.List
	if propObj != nil {
		var err error
		list, err = property.ExtractList(r.x, propObj)
		if err != nil {
			return nil, err
		}
	}

	return &graphics.MarkedContent{
		Tag:        tag,
		Properties: list,
		Inline:     inline,
	}, nil
}

func (r *Reader) processText(s pdf.String) error {
	// TODO(voss): can this be merged with the corresponding code in op-text.go?

	wmode := r.TextFont.WritingMode()
	for info := range r.TextFont.Codes(s) {
		width := info.Width*r.TextFontSize + r.TextCharacterSpacing
		if info.UseWordSpacing {
			width += r.TextWordSpacing
		}
		if wmode == font.Horizontal {
			width *= r.TextHorizontalScaling
		}

		if r.Character != nil && r.TextRenderingMode != graphics.TextRenderingModeInvisible {
			err := r.Character(info.CID, info.Text, width)
			if err != nil {
				return err
			}
		}
		if r.Text != nil && r.TextRenderingMode != graphics.TextRenderingModeInvisible {
			err := r.Text(info.Text)
			if err != nil {
				return err
			}
		}

		switch wmode {
		case font.Horizontal:
			r.TextMatrix = matrix.Translate(width, 0).Mul(r.TextMatrix)
		case font.Vertical:
			r.TextMatrix = matrix.Translate(0, width).Mul(r.TextMatrix)
		}
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
		pat[i] = x
	}
	return pat, true
}

const (
	maxGraphicsStackDepth = 64
	maxMarkedContentDepth = 64
)
