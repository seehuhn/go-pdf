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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/reader/scanner"
	"seehuhn.de/go/sfnt/glyph"
)

// A Reader reads a PDF content stream.
type Reader struct {
	R          pdf.Getter
	scanner    *scanner.Scanner
	loader     *loader.FontLoader
	nextIntRef uint32

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
		R:       r,
		scanner: scanner.NewScanner(),
		loader:  loader,

		fontCache: make(map[pdf.Reference]FontFromFile),
	}
}

func (r *Reader) NewPage() {
	r.scanner.Reset()
	r.State = graphics.NewState()
	r.stack = r.stack[:0]
	r.Resources = &pdf.Resources{}
}

// ParsePage parses a page, and calls the appropriate callback functions.
func (r *Reader) ParsePage(page pdf.Object, ctm graphics.Matrix) error {
	pageDict, err := pdf.GetDictTyped(r.R, page, "Page")
	if err != nil {
		return err
	}

	r.NewPage()
	r.State.CTM = ctm

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
	return r.scanner.Scan(in)(r.do)
}

// Do processes the given operator and arguments.
// This updates the graphics state, and calls the appropriate callback
// functions.
func (r *Reader) do(op string, args []pdf.Object) error {
	origArgs := args

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
	getString := func() (pdf.String, bool) {
		if len(args) == 0 {
			return nil, false
		}
		x, ok := args[0].(pdf.String)
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
			r.Set |= graphics.StateLineWidth
		}

	case "J": // line cap style
		x, ok := getInteger()
		if ok {
			if graphics.LineCapStyle(x) > 2 {
				x = 0
			}
			r.LineCap = graphics.LineCapStyle(x)
			r.Set |= graphics.StateLineCap
		}

	case "j": // line join style
		x, ok := getInteger()
		if ok {
			if graphics.LineJoinStyle(x) > 2 {
				x = 0
			}
			r.LineJoin = graphics.LineJoinStyle(x)
			r.Set |= graphics.StateLineJoin
		}

	case "M": // miter limit
		x, ok := getNum()
		if ok {
			r.MiterLimit = float64(x)
			r.Set |= graphics.StateMiterLimit
		}

	case "d": // dash pattern and phase
		patObj, ok1 := getArray()
		pattern, ok2 := convertDashPattern(patObj)
		phase, ok3 := getNum()
		if ok1 && ok2 && ok3 {
			r.DashPattern = pattern
			r.DashPhase = phase
			r.Set |= graphics.StateDash
		}

	case "ri": // rendering intent
		name, ok := getName()
		if ok {
			r.RenderingIntent = name
			r.Set |= graphics.StateRenderingIntent
		}

	case "i": // flatness tolerance
		x, ok := getNum()
		if ok {
			r.FlatnessTolerance = float64(x)
			r.Set |= graphics.StateFlatnessTolerance
		}

	case "gs": // Set parameters from graphics state parameter dictionary
		name, ok := getName()
		if ok {
			// TODO(voss): use caching
			newState, err := r.ReadExtGState(r.Resources.ExtGState[name], name)
			if pdf.IsMalformed(err) {
				break
			} else if err != nil {
				return err
			}
			newState.ApplyTo(&r.State)
		}

	case "q":
		r.stack = append(r.stack, graphics.State{
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
		m := graphics.Matrix{}
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
		r.TextMatrix = graphics.IdentityMatrix
		r.TextLineMatrix = graphics.IdentityMatrix
		r.Set |= graphics.StateTextMatrix

	case "ET": // End text object

	// == Text state =====================================================

	case "Tc": // Set character spacing
		x, ok := getNum()
		if ok {
			r.TextCharacterSpacing = x
			r.Set |= graphics.StateTextCharacterSpacing
		}

	case "Tw": // Set word spacing
		x, ok := getNum()
		if ok {
			r.TextWordSpacing = x
			r.Set |= graphics.StateTextWordSpacing
		}

	case "Tz": // Set the horizontal scaling
		x, ok := getNum()
		if ok {
			r.TextHorizontalScaling = x / 100
			r.Set |= graphics.StateTextHorizontalScaling
		}

	case "TL": // Set the leading
		x, ok := getNum()
		if ok {
			r.TextLeading = x
			r.Set |= graphics.StateTextLeading
		}

	case "Tf": // Set text font and size
		name, ok1 := getName()
		size, ok2 := getNum()
		if !ok1 || !ok2 || r.Resources == nil || r.Resources.Font == nil {
			break
		}
		ref := r.Resources.Font[name]
		if ref == nil {
			break
		}
		F, err := r.ReadFont(ref, name)
		if pdf.IsMalformed(err) {
			break
		} else if err != nil {
			return pdf.Wrap(err, fmt.Sprintf("font %s", name))
		}
		r.TextFont = F
		r.TextFontSize = size
		r.Set |= graphics.StateTextFont

	case "Tr": // text rendering mode
		x, ok := getInteger()
		if ok {
			r.TextRenderingMode = graphics.TextRenderingMode(x)
			r.Set |= graphics.StateTextRenderingMode
		}

	case "Ts": // Set text rise
		x, ok := getNum()
		if ok {
			r.TextRise = x
			r.Set |= graphics.StateTextRise
		}

	// == Text positioning ===============================================

	case "Td": // Move text position
		dx, ok1 := getNum()
		dy, ok2 := getNum()
		if ok1 && ok2 {
			r.TextLineMatrix = graphics.Translate(dx, dy).Mul(r.TextLineMatrix)
			r.TextMatrix = r.TextLineMatrix
		}

	case "TD": // Move text position and set leading
		dx, ok1 := getNum()
		dy, ok2 := getNum()
		if ok1 && ok2 {
			r.TextLeading = -dy
			r.Set |= graphics.StateTextLeading
			r.TextLineMatrix = graphics.Translate(dx, dy).Mul(r.TextLineMatrix)
			r.TextMatrix = r.TextLineMatrix
		}

	case "Tm": // Set text matrix and text line matrix
		m := graphics.Matrix{}
		for i := 0; i < 6; i++ {
			f, ok := getNum()
			if !ok {
				break doOps
			}
			m[i] = float64(f)
		}
		r.TextMatrix = m
		r.TextLineMatrix = m
		r.Set |= graphics.StateTextMatrix

	case "T*": // Move to start of next text line
		r.TextLineMatrix = graphics.Translate(0, -r.TextLeading).Mul(r.TextLineMatrix)
		r.TextMatrix = r.TextLineMatrix

	// == Text showing ===================================================

	case "Tj": // Show text
		s, ok := getString()
		if !ok {
			break
		}
		r.processText(s)

	case "'": // Move to next line and show text
		s, ok := getString()
		if ok {
			r.TextLineMatrix = graphics.Translate(0, -r.TextLeading).Mul(r.TextLineMatrix)
			r.TextMatrix = r.TextLineMatrix
			r.processText(s)
		}

	case "\"": // Set spacing, move to next line, and show text
		aw, ok1 := getNum()
		ac, ok2 := getNum()
		s, ok3 := getString()
		if ok1 && ok2 && ok3 {
			r.TextWordSpacing = aw
			r.TextCharacterSpacing = ac
			r.Set |= graphics.StateTextWordSpacing | graphics.StateTextCharacterSpacing
			r.processText(s)
		}

	case "TJ": // Show text with kerning
		a, ok := getArray()
		if !ok {
			break
		}
		for _, ai := range a {
			switch ai := ai.(type) {
			case pdf.String:
				r.processText(ai)
			case pdf.Number, pdf.Integer:
				var d float64
				switch ai := ai.(type) {
				case pdf.Integer:
					d = float64(ai)
				case pdf.Real:
					d = float64(ai)
				case pdf.Number:
					d = float64(ai)
				default:
					break doOps
				}
				d = d / 1000 * r.TextFontSize
				switch r.TextFont.WritingMode() {
				case 0:
					r.TextMatrix = graphics.Translate(-d*r.TextHorizontalScaling, 0).Mul(r.TextMatrix)
				case 1:
					r.TextMatrix = graphics.Translate(0, -d).Mul(r.TextMatrix)
				}
			}
		}

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
		r.StrokeColor = color.DeviceGray.New(gray)
		r.Set |= graphics.StateStrokeColor

	case "g": // nonstroking gray level
		if len(args) < 1 {
			break
		}
		gray, ok := getNumber(args[0])
		if !ok {
			break
		}
		r.FillColor = color.DeviceGray.New(gray)
		r.Set |= graphics.StateFillColor

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
		r.StrokeColor = color.DeviceRGB.New(red, green, blue)

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
		r.FillColor = color.DeviceRGB.New(red, green, blue)

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
		r.StrokeColor = color.DeviceCMYK.New(cyan, magenta, yellow, black)

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
		r.FillColor = color.DeviceCMYK.New(cyan, magenta, yellow, black)

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
		// TODO(voss): do something with this information
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

		// TODO(voss): do something with this information
		_ = name
		_ = dict

	case "EMC": // End marked-content sequence

		// == Compatibility ===================================================

	default:
		if r.UnknownOp != nil {
			err := r.UnknownOp(op, args)
			if err != nil {
				return err
			}
		}
	}
	if r.EveryOp != nil {
		err := r.EveryOp(op, origArgs)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Reader) processText(s pdf.String) {
	switch F := r.TextFont.(type) {
	case nil:
		// TODO(voss): what to do here?
		return
	case FontFromFile:
		wmode := F.WritingMode()
		F.ForeachGlyph(s, func(gid glyph.ID, text []rune, width float64, is_space bool) {
			width = width*r.TextFontSize + r.TextCharacterSpacing
			if is_space {
				width += r.TextWordSpacing
			}
			if wmode == 0 {
				width *= r.TextHorizontalScaling
			}

			if r.DrawGlyph != nil {
				g := font.Glyph{
					GID:     gid,
					Advance: width,
					Rise:    r.TextRise,
					Text:    text,
				}
				r.DrawGlyph(g)
			}
			if r.Text != nil {
				r.Text(string(text))
			}

			switch wmode {
			case 0: // horizontal
				r.TextMatrix = graphics.Translate(width, 0).Mul(r.TextMatrix)
			case 1: // vertical
				r.TextMatrix = graphics.Translate(0, width).Mul(r.TextMatrix)
			}
		})
	default:
		panic(fmt.Sprintf("unexpected font type %T", F))
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
		pat[i] = float64(x)
	}
	return pat, true
}
