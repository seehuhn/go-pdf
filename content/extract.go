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

package content

import (
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
)

type extractor struct {
	*pdf.Resources
	*graphics.Parameters
}

// Context holds information about the current state of the PDF content stream.
type Context struct {
	*pdf.Resources
	graphics.State
}

// ForAllText loads the given page of a PDF file and calls the given
// function for each text string on the page.
func ForAllText(r pdf.Getter, pageDict pdf.Object, cb func(*Context, string) error) error {
	page, err := pdf.GetDictTyped(r, pageDict, "Page")
	if err != nil {
		return err
	}

	resourcesDict, err := pdf.GetDict(r, page["Resources"])
	if err != nil {
		return err
	}
	resources := &pdf.Resources{}
	err = pdf.DecodeDict(r, resources, resourcesDict)
	if err != nil {
		return err
	}

	fonts := make(map[pdf.Name]font.NewFont)
	getFont := func(name pdf.Name) (font.NewFont, error) {
		if f, ok := fonts[name]; ok {
			return f, nil
		}
		ref, _ := resources.Font[name].(pdf.Reference)
		f, err := font.Read(r, ref, name)
		if err != nil {
			return nil, err
		}
		fonts[name] = f
		return f, nil
	}

	var graphicsStack []graphics.State
	state := graphics.NewState()

	decoders := make(map[graphics.Resource]func(pdf.String) string)
	yield := func(ctx *Context, s pdf.String) error {
		font := state.Parameters.TextFont
		decoder, ok := decoders[font]
		if !ok {
			fontRef := font.PDFObject()
			decoder, err = makeTextDecoder(r, fontRef)
			if err != nil {
				return err
			}
			decoders[font] = decoder
		}
		return cb(ctx, decoder(s))
	}

	seq := &parser{}

	err = foreachContentStreamPart(r, page["Contents"], func(r pdf.Getter, contents *pdf.Stream) error {
		stm, err := pdf.DecodeStream(r, contents, 0)
		if err != nil {
			return err
		}

		// TODO(voss): use graphics.Scanner
		err = seq.foreachCommand(stm, func(op operator, args []pdf.Object) error {
			switch op {

			// == General graphics state =========================================

			case "q":
				graphicsStack = append(graphicsStack, graphics.State{
					Parameters: state.Parameters.Clone(),
					Set:        state.Set,
				})
			case "Q":
				if len(graphicsStack) > 0 {
					state = graphicsStack[len(graphicsStack)-1]
					graphicsStack = graphicsStack[:len(graphicsStack)-1]
				}
			case "cm": // Concatenate matrix to current transformation matrix
				if len(args) < 6 {
					break
				}
				m := graphics.Matrix{}
				for i := 0; i < 6; i++ {
					f, err := pdf.GetNumber(r, args[i])
					if pdf.IsMalformed(err) {
						break
					} else if err != nil {
						return err
					}
					m[i] = float64(f)
				}
				state.CTM = m.Mul(state.CTM) // TODO(voss): correct order?
			case "w": // Set line width
				if len(args) < 1 {
					break
				}
				f, err := pdf.GetNumber(r, args[0])
				if pdf.IsMalformed(err) {
					break
				} else if err != nil {
					return err
				}
				state.LineWidth = float64(f)
			case "M": // Set miter limit
				if len(args) < 1 {
					break
				}
				f, err := pdf.GetNumber(r, args[0])
				if err != nil {
					return err
				}
				state.MiterLimit = float64(f)
			case "gs": // Set parameters from graphics state parameter dictionary
				if len(args) < 1 {
					break
				}
				name, ok := args[0].(pdf.Name)
				if !ok {
					break
				}

				newState, err := graphics.ReadExtGState(r, resources.ExtGState[name], name)
				if pdf.IsMalformed(err) {
					break
				} else if err != nil {
					return err
				}
				newState.ApplyTo(&state)

			// == Special graphics state =========================================

			// == Path construction ==============================================

			case "m": // Begin new subpath
				if len(args) < 2 {
					break
				}
				x, ok1 := getReal(args[0])
				y, ok2 := getReal(args[1])
				if !ok1 || !ok2 {
					break
				}
				_ = x
				_ = y

			case "l": // Append straight line segment to path
				if len(args) < 2 {
					break
				}
				x, ok1 := getReal(args[0])
				y, ok2 := getReal(args[1])
				if !ok1 || !ok2 {
					break
				}
				_ = x
				_ = y

			case "c": // Append cubic Bezier curve to path
				if len(args) < 6 {
					break
				}
				x1, ok1 := getReal(args[0])
				y1, ok2 := getReal(args[1])
				x2, ok3 := getReal(args[2])
				y2, ok4 := getReal(args[3])
				x3, ok5 := getReal(args[4])
				y3, ok6 := getReal(args[5])
				if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
					break
				}
				_, _, _, _, _, _ = x1, y1, x2, y2, x3, y3

			case "h": // Close subpath

			case "re": // Append rectangle to path
				if len(args) < 4 {
					break
				}
				x, ok1 := getReal(args[0])
				y, ok2 := getReal(args[1])
				w, ok3 := getReal(args[2])
				h, ok4 := getReal(args[3])
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
				state.TextMatrix = graphics.IdentityMatrix
				state.TextLineMatrix = graphics.IdentityMatrix

			case "ET": // End text object

			// == Text state =====================================================

			case "Tc": // Set character spacing
				if len(args) < 1 {
					break
				}
				Tc, ok := getReal(args[0])
				if !ok {
					break
				}
				state.TextCharacterSpacing = Tc

			case "Tf": // Set text font and size
				if len(args) < 2 {
					break
				}
				name, ok1 := args[0].(pdf.Name)
				size, ok2 := getReal(args[1])
				if !ok1 || !ok2 {
					break
				}
				F, err := getFont(name)
				if pdf.IsMalformed(err) {
					break
				} else if err != nil {
					return err
				}
				state.TextFont = F
				state.TextFontSize = size

			// == Text positioning ===============================================

			case "Td": // Move text position
				if len(args) < 2 {
					break
				}
				tx, ok1 := getReal(args[0])
				ty, ok2 := getReal(args[1])
				if !ok1 || !ok2 {
					break
				}

				state.TextLineMatrix = graphics.Matrix{1, 0, 0, 1, tx, ty}.Mul(state.TextLineMatrix)
				state.TextMatrix = state.TextLineMatrix

			case "Tm": // Set text matrix and text line matrix
				if len(args) < 6 {
					break
				}
				var data graphics.Matrix
				for i := 0; i < 6; i++ {
					x, ok := getReal(args[i])
					if !ok {
						break
					}
					data[i] = x
				}
				state.TextMatrix = data
				state.TextLineMatrix = data

			// == Text showing ===================================================

			case "Tj": // Show text
				if len(args) < 1 {
					break
				}
				s, ok := args[0].(pdf.String)
				if !ok {
					break
				}

				yield(&Context{resources, state}, s)

				// TODO(voss): update g.Tm

			case "TJ": // Show text with kerning
				if len(args) < 1 {
					break
				}
				arr, ok := args[0].(pdf.Array)
				if !ok {
					break
				}

				for _, frag := range arr {
					switch frag := frag.(type) {
					case pdf.String:
						yield(&Context{resources, state}, frag)
					case pdf.Integer:
					case pdf.Real:
					case pdf.Number:
					}
				}

				// TODO(voss): update g.Tm

			// == Type 3 fonts ===================================================

			// == Color ==========================================================

			case "G": // stroking gray level
				if len(args) < 1 {
					break
				}
				gray, ok := getReal(args[0])
				if !ok {
					break
				}
				state.StrokeColor = color.Gray(gray)

			case "g": // nonstroking gray level
				if len(args) < 1 {
					break
				}
				gray, ok := getReal(args[0])
				if !ok {
					break
				}
				state.FillColor = color.Gray(gray)

			case "RG": // nonstroking DeviceRGB color
				if len(args) < 3 {
					break
				}
				var red, green, blue float64
				var ok bool
				if red, ok = getReal(args[0]); !ok {
					break
				}
				if green, ok = getReal(args[1]); !ok {
					break
				}
				if blue, ok = getReal(args[2]); !ok {
					break
				}
				state.StrokeColor = color.RGB(red, green, blue)

			case "rg": // nonstroking DeviceRGB color
				if len(args) < 3 {
					break
				}
				var red, green, blue float64
				var ok bool
				if red, ok = getReal(args[0]); !ok {
					break
				}
				if green, ok = getReal(args[1]); !ok {
					break
				}
				if blue, ok = getReal(args[2]); !ok {
					break
				}
				state.FillColor = color.RGB(red, green, blue)

			case "K": // stroking DeviceCMYK color
				if len(args) < 4 {
					break
				}
				var cyan, magenta, yellow, black float64
				var ok bool
				if cyan, ok = getReal(args[0]); !ok {
					break
				}
				if magenta, ok = getReal(args[1]); !ok {
					break
				}
				if yellow, ok = getReal(args[2]); !ok {
					break
				}
				if black, ok = getReal(args[3]); !ok {
					break
				}
				state.StrokeColor = color.CMYK(cyan, magenta, yellow, black)

			case "k": // nonstroking DeviceCMYK color
				if len(args) < 4 {
					break
				}
				var cyan, magenta, yellow, black float64
				var ok bool
				if cyan, ok = getReal(args[0]); !ok {
					break
				}
				if magenta, ok = getReal(args[1]); !ok {
					break
				}
				if yellow, ok = getReal(args[2]); !ok {
					break
				}
				if black, ok = getReal(args[3]); !ok {
					break
				}
				state.FillColor = color.CMYK(cyan, magenta, yellow, black)

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
					dict, err = pdf.GetDict(r, resources.Properties[a])
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
		})
		return err
	})
	if err != nil {
		return err
	}

	return nil
}

type parser struct {
	args []pdf.Object
}

func (o *parser) foreachCommand(stm io.Reader, yield func(name operator, args []pdf.Object) error) error {
	// TODO(voss): use one scanner for all parts, add white space between parts
	s := newScanner(stm)
	for {
		obj, err := s.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		cmd, ok := obj.(operator)
		if !ok {
			o.args = append(o.args, obj)
			continue
		}

		yield(cmd, o.args)
		o.args = o.args[:0]
	}
}

func foreachContentStreamPart(r pdf.Getter, ref pdf.Object, yield func(pdf.Getter, *pdf.Stream) error) error {
	contents, err := pdf.Resolve(r, ref)
	if err != nil {
		return err
	}
	switch contents := contents.(type) {
	case *pdf.Stream:
		return yield(r, contents)
	case pdf.Array:
		for _, ref := range contents {
			contents, err := pdf.GetStream(r, ref)
			if err != nil {
				return err
			}
			err = yield(r, contents)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getReal(x pdf.Object) (float64, bool) {
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
