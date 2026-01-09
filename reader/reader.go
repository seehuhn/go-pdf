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
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/state"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/property"
)

// A Reader reads a PDF content stream.
type Reader struct {
	R pdf.Getter
	x *pdf.Extractor

	Resources *content.Resources
	graphics.State
	stack []graphics.State

	// User callbacks.
	// TODO(voss): clean up this list
	Character func(cid cid.CID, text string) error
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
func New(r pdf.Getter) *Reader {
	return &Reader{
		R:                  r,
		x:                  pdf.NewExtractor(r),
		MarkedContentStack: make([]*graphics.MarkedContent, 0, 8),
	}
}

// Reset resets the reader to its initial state.
// This should be used before parsing a new page.
func (r *Reader) Reset() {
	r.Resources = &content.Resources{}
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
	r.Resources, err = extract.Resources(r.x, pageDict["Resources"])
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
	v := pdf.GetVersion(r.R)
	stream, err := content.ReadStream(in, v, content.Page)
	if err != nil {
		return err
	}
	return r.processStream(stream)
}

func (r *Reader) processStream(stream content.Stream) error {
	for _, op := range stream {
		origArgs := op.Args

		switch op.Name {

		// Table 56 – Graphics state operators

		case content.OpPushGraphicsState: // q
			if len(r.stack) < maxGraphicsStackDepth {
				r.stack = append(r.stack, graphics.State{
					Parameters: r.Parameters.Clone(),
					Set:        r.Set,
				})
			}

		case content.OpPopGraphicsState: // Q
			if len(r.stack) > 0 {
				r.State = r.stack[len(r.stack)-1]
				r.stack = r.stack[:len(r.stack)-1]
			}

		case content.OpTransform: // cm
			if len(op.Args) >= 6 {
				m := matrix.Matrix{}
				ok := true
				for i := range 6 {
					if v, vok := getNumber(op.Args[i]); vok {
						m[i] = v
					} else {
						ok = false
						break
					}
				}
				if ok {
					// TODO(voss): correct order?  Add unit tests.
					r.CTM = r.CTM.Mul(m)
				}
			}

		case content.OpSetLineWidth: // w
			if lineWidth, ok := getNumber(op.Args, 0); ok {
				r.LineWidth = lineWidth
				r.Set |= state.LineWidth
			}

		case content.OpSetLineCap: // J
			if lineCap, ok := getInteger(op.Args, 0); ok {
				if lineCap < 0 {
					lineCap = 0
				} else if lineCap > 2 {
					lineCap = 2
				}
				r.LineCap = graphics.LineCapStyle(lineCap)
				r.Set |= state.LineCap
			}

		case content.OpSetLineJoin: // j
			if lineJoin, ok := getInteger(op.Args, 0); ok {
				if lineJoin < 0 {
					lineJoin = 0
				} else if lineJoin > 2 {
					lineJoin = 2
				}
				r.LineJoin = graphics.LineJoinStyle(lineJoin)
				r.Set |= state.LineJoin
			}

		case content.OpSetMiterLimit: // M
			if miterLimit, ok := getNumber(op.Args, 0); ok {
				if miterLimit < 1 {
					miterLimit = 1
				}
				r.MiterLimit = miterLimit
				r.Set |= state.MiterLimit
			}

		case content.OpSetLineDash: // d
			if len(op.Args) >= 2 {
				if patArr, ok := op.Args[0].(pdf.Array); ok {
					if phase, pok := getNumber(op.Args, 1); pok {
						if pat, dok := convertDashPattern(patArr); dok {
							r.DashPattern = pat
							r.DashPhase = phase
							r.Set |= state.LineDash
						}
					}
				}
			}

		case content.OpSetRenderingIntent: // ri
			if intent, ok := getName(op.Args, 0); ok {
				r.RenderingIntent = graphics.RenderingIntent(intent)
				r.Set |= state.RenderingIntent
			}

		case content.OpSetFlatnessTolerance: // i
			if flatness, ok := getNumber(op.Args, 0); ok {
				if flatness < 0 {
					flatness = 0
				} else if flatness > 100 {
					flatness = 100
				}
				r.FlatnessTolerance = flatness
				r.Set |= state.FlatnessTolerance
			}

		case content.OpSetExtGState: // gs
			if dictName, ok := getName(op.Args, 0); ok {
				if r.Resources != nil && r.Resources.ExtGState != nil {
					if extGState := r.Resources.ExtGState[dictName]; extGState != nil {
						extGState.ApplyTo(&r.State)
					}
				}
			}

		// Table 105 - Text object operators

		case content.OpTextBegin: // BT
			r.TextMatrix = matrix.Identity
			r.TextLineMatrix = matrix.Identity
			r.Set |= state.TextMatrix

		case content.OpTextEnd: // ET
			r.Set &= ^state.TextMatrix

		// Table 103 - Text state operators

		case content.OpTextSetCharacterSpacing: // Tc
			if charSpace, ok := getNumber(op.Args, 0); ok {
				r.TextCharacterSpacing = charSpace
				r.Set |= state.TextCharacterSpacing
			}

		case content.OpTextSetWordSpacing: // Tw
			if wordSpace, ok := getNumber(op.Args, 0); ok {
				r.TextWordSpacing = wordSpace
				r.Set |= state.TextWordSpacing
			}

		case content.OpTextSetHorizontalScaling: // Tz
			if scale, ok := getNumber(op.Args, 0); ok {
				r.TextHorizontalScaling = scale / 100
				r.Set |= state.TextHorizontalScaling
			}

		case content.OpTextSetLeading: // TL
			if leading, ok := getNumber(op.Args, 0); ok {
				r.TextLeading = leading
				r.Set |= state.TextLeading
			}

		case content.OpTextSetFont: // Tf
			fontName, ok1 := getName(op.Args, 0)
			size, ok2 := getNumber(op.Args, 1)
			if ok1 && ok2 && r.Resources != nil && r.Resources.Font != nil {
				if F := r.Resources.Font[fontName]; F != nil {
					r.TextFont = F
					r.TextFontSize = size
					r.Set |= state.TextFont
				}
			}

		case content.OpTextSetRenderingMode: // Tr
			if render, ok := getInteger(op.Args, 0); ok {
				if render < 0 {
					render = 0
				} else if render > 7 {
					render = 7
				}
				r.TextRenderingMode = graphics.TextRenderingMode(render)
				r.Set |= state.TextRenderingMode
			}

		case content.OpTextSetRise: // Ts
			if rise, ok := getNumber(op.Args, 0); ok {
				r.TextRise = rise
				r.Set |= state.TextRise
			}

		// Table 106 - Text-positioning operators

		case content.OpTextMoveOffset: // Td
			tx, ok1 := getNumber(op.Args, 0)
			ty, ok2 := getNumber(op.Args, 1)
			if ok1 && ok2 {
				r.TextLineMatrix = matrix.Translate(tx, ty).Mul(r.TextLineMatrix)
				r.TextMatrix = r.TextLineMatrix
				if r.TextEvent != nil {
					r.TextEvent(TextEventNL, 0)
				}
			}

		case content.OpTextMoveOffsetSetLeading: // TD
			tx, ok1 := getNumber(op.Args, 0)
			ty, ok2 := getNumber(op.Args, 1)
			if ok1 && ok2 {
				r.TextLeading = -ty
				r.Set |= state.TextLeading
				r.TextLineMatrix = matrix.Translate(tx, ty).Mul(r.TextLineMatrix)
				r.TextMatrix = r.TextLineMatrix
				if r.TextEvent != nil {
					r.TextEvent(TextEventNL, 0)
				}
			}

		case content.OpTextSetMatrix: // Tm
			if len(op.Args) >= 6 {
				m := matrix.Matrix{}
				ok := true
				for i := range 6 {
					if v, vok := getNumber(op.Args[i]); vok {
						m[i] = v
					} else {
						ok = false
						break
					}
				}
				if ok {
					r.TextMatrix = m
					r.TextLineMatrix = m
					r.Set |= state.TextMatrix
					if r.TextEvent != nil {
						r.TextEvent(TextEventMove, 0)
					}
				}
			}

		case content.OpTextNextLine: // T*
			r.TextLineMatrix = matrix.Translate(0, -r.TextLeading).Mul(r.TextLineMatrix)
			r.TextMatrix = r.TextLineMatrix
			if r.TextEvent != nil {
				r.TextEvent(TextEventNL, 0)
			}

		// Table 107 - Text-showing operators

		case content.OpTextShow: // Tj
			if s, ok := getString(op.Args, 0); ok && r.TextFont != nil {
				err := r.processText(s)
				if err != nil {
					return err
				}
			}

		case content.OpTextShowMoveNextLine: // '
			if s, ok := getString(op.Args, 0); ok && r.TextFont != nil {
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

		case content.OpTextShowMoveNextLineSetSpacing: // "
			aw, ok1 := getNumber(op.Args, 0)
			ac, ok2 := getNumber(op.Args, 1)
			s, ok3 := getString(op.Args, 2)
			if ok1 && ok2 && ok3 && r.TextFont != nil {
				r.TextWordSpacing = aw
				r.TextCharacterSpacing = ac
				r.Set |= state.TextWordSpacing | state.TextCharacterSpacing
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

		case content.OpTextShowArray: // TJ
			if a, ok := getArray(op.Args, 0); ok && r.TextFont != nil {
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

		case content.OpType3ColoredGlyph: // d0
			// TODO(voss): implement this

		case content.OpType3UncoloredGlyph: // d1
			// TODO(voss): implement this

		// Table 73 — Colour operators

		case content.OpSetStrokeColorSpace: // CS
			if name, ok := getName(op.Args, 0); ok {
				cs := r.getColorSpace(name)
				if cs != nil {
					r.StrokeColor = cs.Default()
					r.Set |= state.StrokeColor
				}
			}

		case content.OpSetFillColorSpace: // cs
			if name, ok := getName(op.Args, 0); ok {
				cs := r.getColorSpace(name)
				if cs != nil {
					r.FillColor = cs.Default()
					r.Set |= state.FillColor
				}
			}

		case content.OpSetStrokeColor, content.OpSetStrokeColorN: // SC, SCN
			values, pat := r.parseColorArgs(op.Args)
			r.StrokeColor = color.SCN(r.StrokeColor, values, pat)
			r.Set |= state.StrokeColor

		case content.OpSetFillColor, content.OpSetFillColorN: // sc, scn
			values, pat := r.parseColorArgs(op.Args)
			r.FillColor = color.SCN(r.FillColor, values, pat)
			r.Set |= state.FillColor

		case content.OpSetStrokeGray: // G
			if gray, ok := getNumber(op.Args, 0); ok {
				r.StrokeColor = color.DeviceGray(gray)
				r.Set |= state.StrokeColor
			}

		case content.OpSetFillGray: // g
			if gray, ok := getNumber(op.Args, 0); ok {
				r.FillColor = color.DeviceGray(gray)
				r.Set |= state.FillColor
			}

		case content.OpSetStrokeRGB: // RG
			red, ok1 := getNumber(op.Args, 0)
			green, ok2 := getNumber(op.Args, 1)
			blue, ok3 := getNumber(op.Args, 2)
			if ok1 && ok2 && ok3 {
				r.StrokeColor = color.DeviceRGB{red, green, blue}
				r.Set |= state.StrokeColor
			}

		case content.OpSetFillRGB: // rg
			red, ok1 := getNumber(op.Args, 0)
			green, ok2 := getNumber(op.Args, 1)
			blue, ok3 := getNumber(op.Args, 2)
			if ok1 && ok2 && ok3 {
				r.FillColor = color.DeviceRGB{red, green, blue}
				r.Set |= state.FillColor
			}

		case content.OpSetStrokeCMYK: // K
			c, ok1 := getNumber(op.Args, 0)
			m, ok2 := getNumber(op.Args, 1)
			y, ok3 := getNumber(op.Args, 2)
			k, ok4 := getNumber(op.Args, 3)
			if ok1 && ok2 && ok3 && ok4 {
				r.StrokeColor = color.DeviceCMYK{c, m, y, k}
				r.Set |= state.StrokeColor
			}

		case content.OpSetFillCMYK: // k
			c, ok1 := getNumber(op.Args, 0)
			m, ok2 := getNumber(op.Args, 1)
			y, ok3 := getNumber(op.Args, 2)
			k, ok4 := getNumber(op.Args, 3)
			if ok1 && ok2 && ok3 && ok4 {
				r.FillColor = color.DeviceCMYK{c, m, y, k}
				r.Set |= state.FillColor
			}

		// Table 76 - Shading operator

		case content.OpShading: // sh
			// TODO(voss): implement this

		// Table 319 - Marked-content operators

		case content.OpMarkedContentPoint: // MP
			if tag, ok := getName(op.Args, 0); ok && r.MarkedContent != nil {
				mc := &graphics.MarkedContent{
					Tag:        tag,
					Properties: nil,
				}
				err := r.MarkedContent(MarkedContentPoint, mc)
				if err != nil {
					return err
				}
			}

		case content.OpBeginMarkedContent: // BMC
			if tag, ok := getName(op.Args, 0); ok && len(r.MarkedContentStack) < maxMarkedContentDepth {
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

		case content.OpMarkedContentPointWithProperties: // DP
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

		case content.OpBeginMarkedContentWithProperties: // BDC
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

		case content.OpEndMarkedContent: // EMC
			if len(r.MarkedContentStack) > 0 {
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
				err := r.UnknownOp(string(op.Name), op.Args)
				if err != nil {
					return err
				}
			}
		}

		if r.EveryOp != nil {
			err := r.EveryOp(string(op.Name), origArgs)
			if err != nil {
				return err
			}
		}

	}
	return nil
}

// getColorSpace returns the color space for the given name.
func (r *Reader) getColorSpace(name pdf.Name) color.Space {
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
		if r.Resources != nil && r.Resources.ColorSpace != nil {
			return r.Resources.ColorSpace[name]
		}
		return nil
	}
}

// parseColorArgs extracts color values and optional pattern from operator arguments.
func (r *Reader) parseColorArgs(args []pdf.Object) ([]float64, color.Pattern) {
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
			if r.Resources != nil && r.Resources.Pattern != nil {
				pat = r.Resources.Pattern[a]
			}
		}
	}
	return values, pat
}

// extractMarkedContent extracts marked content properties from operator arguments.
// It returns the MarkedContent struct or an error if extraction fails.
func (r *Reader) extractMarkedContent(tag pdf.Name, propArg pdf.Object) (*graphics.MarkedContent, error) {
	var list property.List
	var inline bool

	if name, ok := propArg.(pdf.Name); ok {
		// Property list referenced by name from resources
		if r.Resources != nil && r.Resources.Properties != nil {
			list = r.Resources.Properties[name]
		}
		inline = false
	} else {
		// Inline property dictionary
		var err error
		list, err = property.ExtractList(r.x, propArg)
		if err != nil {
			return nil, err
		}
		inline = true
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
			err := r.Character(info.CID, info.Text)
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

// getNumber extracts a number from the argument slice at the given index.
// It can also be called with a single pdf.Object argument (index is ignored).
func getNumber(args any, idx ...int) (float64, bool) {
	var x pdf.Object
	switch a := args.(type) {
	case []pdf.Object:
		if len(idx) == 0 || idx[0] >= len(a) {
			return 0, false
		}
		x = a[idx[0]]
	case pdf.Object:
		x = a
	default:
		return 0, false
	}

	switch v := x.(type) {
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

// getString extracts a string from the argument slice at the given index.
func getString(args []pdf.Object, idx int) (pdf.String, bool) {
	if idx >= len(args) {
		return nil, false
	}
	if s, ok := args[idx].(pdf.String); ok {
		return s, true
	}
	return nil, false
}

// getArray extracts an array from the argument slice at the given index.
func getArray(args []pdf.Object, idx int) (pdf.Array, bool) {
	if idx >= len(args) {
		return nil, false
	}
	if a, ok := args[idx].(pdf.Array); ok {
		return a, true
	}
	return nil, false
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
