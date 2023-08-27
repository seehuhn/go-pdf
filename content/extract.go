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
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/graphics"
)

// Context holds information about the current state of the PDF content stream.
type Context struct {
	*pdf.Resources
	*graphics.State
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

	var graphicsStack []*graphics.State
	g := graphics.NewState()

	decoders := make(map[pdf.Name]func(pdf.String) string)
	yield := func(ctx *Context, s pdf.String) error {
		fontName := g.Font
		decoder, ok := decoders[fontName]
		if !ok {
			fontRef := resources.Font[fontName]
			decoder, err = makeTextDecoder(r, fontRef)
			if err != nil {
				return err
			}
			decoders[fontName] = decoder
		}
		return cb(ctx, decoder(s))
	}

	seq := &operatorSeq{}

	err = forAllContentStreamParts(r, page["Contents"], func(r pdf.Getter, contents *pdf.Stream) error {
		stm, err := pdf.DecodeStream(r, contents, 0)
		if err != nil {
			return err
		}

		err = seq.forAllCommands(stm, func(cmd Operator, args []pdf.Object) error {
			switch cmd {

			// == General graphics state =========================================

			case "q":
				graphicsStack = append(graphicsStack, g.Clone())
			case "Q":
				if len(graphicsStack) == 0 {
					return errors.New("unexpected operator Q")
				}
				g = graphicsStack[len(graphicsStack)-1]
				graphicsStack = graphicsStack[:len(graphicsStack)-1]
			case "cm": // Concatenate matrix to current transformation matrix
				if len(args) < 6 {
					return errTooFewArgs
				}
				m := graphics.Matrix{}
				for i := 0; i < 6; i++ {
					f, err := pdf.GetNumber(r, args[i])
					if err != nil {
						return err
					}
					m[i] = float64(f)
				}
				// fmt.Println("cm", m)
				g.CTM = m.Mul(g.CTM) // TODO(voss): correct order?
			case "w": // Set line width
				if len(args) < 1 {
					return errTooFewArgs
				}
				f, err := pdf.GetNumber(r, args[0])
				if err != nil {
					return err
				}
				// fmt.Println("w", f)
				g.LineWidth = float64(f)
			case "M": // Set miter limit
				if len(args) < 1 {
					return errTooFewArgs
				}
				f, err := pdf.GetNumber(r, args[0])
				if err != nil {
					return err
				}
				// fmt.Println("M", f)
				g.MiterLimit = float64(f)
			case "gs": // Set parameters from graphics state parameter dictionary
				if len(args) < 1 {
					return errTooFewArgs
				}
				name, ok := args[0].(pdf.Name)
				if !ok {
					return fmt.Errorf("unexpected type %T for graphics state name", args[0])
				}
				// fmt.Printf("gs %s\n", name)

				dict, err := pdf.GetDict(r, resources.ExtGState[name])
				if err != nil {
					return err
				}
				for key, val := range dict {
					switch key {
					case "Type":
						// pass
					case "LW":
						lw, err := pdf.GetNumber(r, val)
						if err != nil {
							return err
						}
						// fmt.Println("\tLW:", lw)
						g.LineWidth = float64(lw)
					case "OP": // stroking overprint
						op, err := pdf.GetBoolean(r, val)
						if err != nil {
							return err
						}
						// fmt.Println("\tOP:", op)
						g.OverprintStroke = bool(op)
						if _, ok := dict["op"]; !ok {
							g.OverprintFill = bool(op)
						}
					case "op": // non-stroking overprint
						op, err := pdf.GetBoolean(r, val)
						if err != nil {
							return err
						}
						// fmt.Println("\top:", op)
						g.OverprintFill = bool(op)
					case "OPM": // overprint mode
						opm, err := pdf.GetInteger(r, val)
						if err != nil {
							return err
						}
						// fmt.Println("\tOPM:", opm)
						g.OverprintMode = int(opm)
					case "SA": // stroke adjustment
						sa, err := pdf.GetBoolean(r, val)
						if err != nil {
							return err
						}
						// fmt.Println("\tSA:", sa)
						g.StrokeAdjustment = bool(sa)
					case "BM": // blend mode
						name, err := pdf.GetName(r, val)
						if err != nil {
							return err
						}
						// fmt.Println("\tBM:", name)
						g.BlendMode = name
					case "SMask": // soft mask
						val, err := pdf.Resolve(r, val)
						if err != nil {
							return err
						}
						// fmt.Println("\tSMask:", val)
						if val == pdf.Name("None") {
							g.SoftMask = nil
						} else if dict, ok := val.(pdf.Dict); ok {
							g.SoftMask = dict
						}

					case "CA": // stroking alpha constant
						CA, err := pdf.GetNumber(r, val)
						if err != nil {
							return err
						}
						// fmt.Println("\tCA:", CA)
						g.StrokeAlpha = float64(CA)
					case "ca": // nonstroking alpha constant
						ca, err := pdf.GetNumber(r, val)
						if err != nil {
							return err
						}
						// fmt.Println("\tca:", ca)
						g.FillAlpha = float64(ca)
					case "AIS":
						val, err := pdf.GetBoolean(r, val)
						if err != nil {
							return err
						}
						g.AlphaSourceFlag = bool(val)
					default:
						// fmt.Printf("* unknown graphics state key: %s\n", key)
						panic("fish")
					}
				}

			// == Special graphics state =========================================

			// == Path construction ==============================================

			case "m": // Begin new subpath
				if len(args) < 2 {
					return errTooFewArgs
				}
				x, ok1 := getReal(args[0])
				y, ok2 := getReal(args[1])
				if !ok1 || !ok2 {
					return fmt.Errorf("unexpected type for m: %T %T", args[0], args[1])
				}
				_ = x
				_ = y
				// fmt.Printf("m %f %f\n", x, y)

			case "l": // Append straight line segment to path
				if len(args) < 2 {
					return errTooFewArgs
				}
				x, ok1 := getReal(args[0])
				y, ok2 := getReal(args[1])
				if !ok1 || !ok2 {
					return fmt.Errorf("unexpected type for l: %T %T", args[0], args[1])
				}
				_ = x
				_ = y
				// fmt.Printf("l %f %f\n", x, y)

			case "c": // Append cubic Bezier curve to path
				if len(args) < 6 {
					return errTooFewArgs
				}
				x1, ok1 := getReal(args[0])
				y1, ok2 := getReal(args[1])
				x2, ok3 := getReal(args[2])
				y2, ok4 := getReal(args[3])
				x3, ok5 := getReal(args[4])
				y3, ok6 := getReal(args[5])
				if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
					return fmt.Errorf("unexpected type for c: %T %T %T %T %T %T", args[0], args[1], args[2], args[3], args[4], args[5])
				}
				_, _, _, _, _, _ = x1, y1, x2, y2, x3, y3
				// fmt.Printf("c %f %f %f %f %f %f\n", x1, y1, x2, y2, x3, y3)

			case "h": // Close subpath
				// fmt.Println("h")

			case "re": // Append rectangle to path
				if len(args) < 4 {
					return errTooFewArgs
				}
				x, ok1 := getReal(args[0])
				y, ok2 := getReal(args[1])
				w, ok3 := getReal(args[2])
				h, ok4 := getReal(args[3])
				if !ok1 || !ok2 || !ok3 || !ok4 {
					return fmt.Errorf("unexpected type for rectangle: %T %T %T %T", args[0], args[1], args[2], args[3])
				}
				_, _, _, _ = x, y, w, h
				// fmt.Printf("re %f %f %f %f\n", x, y, w, h)

			// == Path painting ==================================================

			case "S": // Stroke path
				// fmt.Println("S")

			case "s": // Close and stroke path
				// fmt.Println("s")

			case "f": // Fill path using nonzero winding number rule
				// fmt.Println("f")

			case "f*": // Fill path using even-odd rule
				// fmt.Println("f*")

			case "n": // End path without filling or stroking
				// fmt.Println("n")

			// == Clipping paths =================================================

			case "W": // Modify clipping path by intersecting with current path
				// fmt.Println("W")

			case "W*": // Modify clipping path by intersecting with current path, using even-odd rule
				// fmt.Println("W*")

			// == Text objects ===================================================

			case "BT": // Begin text object
				// fmt.Println("BT")

				g.Tm = graphics.IdentityMatrix
				g.Tlm = graphics.IdentityMatrix

			case "ET": // End text object
				// fmt.Println("ET")

			// == Text state =====================================================

			case "Tc": // Set character spacing
				if len(args) < 1 {
					return errTooFewArgs
				}
				Tc, ok := getReal(args[0])
				if !ok {
					return fmt.Errorf("unexpected type for character spacing: %T", args[0])
				}
				// fmt.Printf("Tc %f\n", Tc)
				g.Tc = Tc

			case "Tf": // Set text font and size
				if len(args) < 2 {
					return errTooFewArgs
				}
				name, ok1 := args[0].(pdf.Name)
				size, ok2 := getReal(args[1])
				if !ok1 || !ok2 {
					return fmt.Errorf("unexpected type for font: %T %T", args[0], args[1])
				}
				// fmt.Printf("Tf %s %f\n", name, size)
				g.Font = name
				g.FontSize = size

			// == Text positioning ===============================================

			case "Td": // Move text position
				if len(args) < 2 {
					return errTooFewArgs
				}
				tx, ok1 := getReal(args[0])
				ty, ok2 := getReal(args[1])
				if !ok1 || !ok2 {
					return fmt.Errorf("unexpected type for text position: %T %T", args[0], args[1])
				}
				// fmt.Printf("Td %f %f\n", tx, ty)

				g.Tlm = graphics.Matrix{1, 0, 0, 1, tx, ty}.Mul(g.Tlm)
				g.Tm = g.Tlm

			case "Tm": // Set text matrix and text line matrix
				if len(args) < 6 {
					return errTooFewArgs
				}
				var data graphics.Matrix
				for i := 0; i < 6; i++ {
					x, ok := getReal(args[i])
					if !ok {
						return fmt.Errorf("unexpected type for text matrix: %T", args[i])
					}
					data[i] = x
				}
				// fmt.Printf("Tm %f %f %f %f %f %f\n", data[0], data[1], data[2], data[3], data[4], data[5])
				g.Tm = data
				g.Tlm = data

			// == Text showing ===================================================

			case "Tj": // Show text
				if len(args) < 1 {
					return errTooFewArgs
				}
				s, ok := args[0].(pdf.String)
				if !ok {
					return fmt.Errorf("unexpected type for text string: %T", args[0])
				}
				// fmt.Printf("Tj %s\n", s)

				yield(&Context{resources, g}, s)

				// TODO(voss): update g.Tm

			case "TJ": // Show text with kerning
				if len(args) < 1 {
					return errTooFewArgs
				}
				arr, ok := args[0].(pdf.Array)
				if !ok {
					return fmt.Errorf("unexpected type for text array: %T", args[0])
				}

				// fmt.Printf("TJ %s\n", arr)

				for _, frag := range arr {
					switch frag := frag.(type) {
					case pdf.String:
						yield(&Context{resources, g}, frag)
					case pdf.Integer:
						// fmt.Printf("  %d\n", frag)
					case pdf.Real:
						// fmt.Printf("  %f\n", frag)
					case pdf.Number:
						// fmt.Printf("  %f\n", frag)
					default:
						return fmt.Errorf("unexpected type for text array fragment: %T", frag)
					}
				}

				// TODO(voss): update g.Tm

			// == Type 3 fonts ===================================================

			// == Color ==========================================================

			case "G": // stroking gray level
				if len(args) < 1 {
					return errTooFewArgs
				}
				gray, ok := getReal(args[0])
				if !ok {
					return fmt.Errorf("unexpected type for gray level: %T", args[0])
				}
				// fmt.Printf("G %f\n", gray)
				g.StrokeColor = color.Gray(gray)

			case "g": // nonstroking gray level
				if len(args) < 1 {
					return errTooFewArgs
				}
				gray, ok := getReal(args[0])
				if !ok {
					return fmt.Errorf("unexpected type for gray level: %T", args[0])
				}
				// fmt.Printf("g %f\n", gray)
				g.FillColor = color.Gray(gray)

			case "RG": // nonstroking DeviceRGB color
				if len(args) < 3 {
					return errTooFewArgs
				}
				var red, green, blue float64
				var ok bool
				if red, ok = getReal(args[0]); !ok {
					return fmt.Errorf("unexpected type for red: %T", args[0])
				}
				if green, ok = getReal(args[1]); !ok {
					return fmt.Errorf("unexpected type for green: %T", args[1])
				}
				if blue, ok = getReal(args[2]); !ok {
					return fmt.Errorf("unexpected type for blue: %T", args[2])
				}
				// fmt.Printf("RG %f %f %f\n", red, green, blue)
				g.StrokeColor = color.RGB(red, green, blue)

			case "rg": // nonstroking DeviceRGB color
				if len(args) < 3 {
					return errTooFewArgs
				}
				var red, green, blue float64
				var ok bool
				if red, ok = getReal(args[0]); !ok {
					return fmt.Errorf("unexpected type for red: %T", args[0])
				}
				if green, ok = getReal(args[1]); !ok {
					return fmt.Errorf("unexpected type for green: %T", args[1])
				}
				if blue, ok = getReal(args[2]); !ok {
					return fmt.Errorf("unexpected type for blue: %T", args[2])
				}
				// fmt.Printf("rg %f %f %f\n", red, green, blue)
				g.FillColor = color.RGB(red, green, blue)

			case "K": // stroking DeviceCMYK color
				if len(args) < 4 {
					return errTooFewArgs
				}
				var cyan, magenta, yellow, black float64
				var ok bool
				if cyan, ok = getReal(args[0]); !ok {
					return fmt.Errorf("unexpected type for cyan: %T", args[0])
				}
				if magenta, ok = getReal(args[1]); !ok {
					return fmt.Errorf("unexpected type for magenta: %T", args[1])
				}
				if yellow, ok = getReal(args[2]); !ok {
					return fmt.Errorf("unexpected type for yellow: %T", args[2])
				}
				if black, ok = getReal(args[3]); !ok {
					return fmt.Errorf("unexpected type for black: %T", args[3])
				}
				// fmt.Printf("K %f %f %f %f\n", cyan, magenta, yellow, black)
				g.StrokeColor = color.CMYK(cyan, magenta, yellow, black)

			case "k": // nonstroking DeviceCMYK color
				if len(args) < 4 {
					return errTooFewArgs
				}
				var cyan, magenta, yellow, black float64
				var ok bool
				if cyan, ok = getReal(args[0]); !ok {
					return fmt.Errorf("unexpected type for cyan: %T", args[0])
				}
				if magenta, ok = getReal(args[1]); !ok {
					return fmt.Errorf("unexpected type for magenta: %T", args[1])
				}
				if yellow, ok = getReal(args[2]); !ok {
					return fmt.Errorf("unexpected type for yellow: %T", args[2])
				}
				if black, ok = getReal(args[3]); !ok {
					return fmt.Errorf("unexpected type for black: %T", args[3])
				}
				// fmt.Printf("k %f %f %f %f\n", cyan, magenta, yellow, black)
				g.FillColor = color.CMYK(cyan, magenta, yellow, black)

			// == Shading patterns ===============================================

			// == Inline images ==================================================

			// == XObjects =======================================================

			// == Marked content =================================================

			case "BMC": // Begin marked-content sequence
				if len(args) < 1 {
					return errTooFewArgs
				}
				name, ok := args[0].(pdf.Name)
				if !ok {
					return fmt.Errorf("unexpected type %T for marked-content name", args[0])
				}
				_ = name
				// fmt.Printf("BMC %s\n", name)

			case "BDC": // Begin marked-content sequence with property list
				if len(args) < 2 {
					return errTooFewArgs
				}
				name, ok := args[0].(pdf.Name)
				if !ok {
					return fmt.Errorf("unexpected type %T for marked-content name", args[0])
				}
				var dict pdf.Dict
				switch a := args[1].(type) {
				case pdf.Dict:
					dict = a
				case pdf.Name:
					dict, err = pdf.GetDict(r, resources.Properties[a])
					if err != nil {
						return fmt.Errorf("BDC: unknown property list %s", a)
					}
				default:
					return fmt.Errorf("BDC: unexpected type %T for marked-content property list", args[1])
				}

				_ = name
				_ = dict
				// fmt.Printf("BDC %s %s\n", name, dict)

			case "EMC": // End marked-content sequence
				// fmt.Println("EMC")

			// == Compatibility ===================================================

			default:
				return errors.New("unknown command " + string(cmd))
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

type operatorSeq struct {
	args []pdf.Object
}

func (o *operatorSeq) forAllCommands(stm io.Reader, yield func(name Operator, args []pdf.Object) error) error {
	// TODO(voss): use one scanner for all parts, add white space between parts
	s := NewScanner(stm)
	for {
		obj, err := s.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		cmd, ok := obj.(Operator)
		if !ok {
			o.args = append(o.args, obj)
			continue
		}

		yield(cmd, o.args)
		o.args = o.args[:0]
	}
}

func forAllContentStreamParts(r pdf.Getter, ref pdf.Object, yield func(pdf.Getter, *pdf.Stream) error) error {
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
	default:
		return fmt.Errorf("unexpected type %T for page contents", contents)
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

var (
	errTooFewArgs = errors.New("not enough arguments")
)
