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
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/graphics/scanner"
)

func FuzzReader(f *testing.F) {
	res := &pdf.Resources{
		ExtGState: map[pdf.Name]pdf.Object{
			"G": pdf.NewReference(1, 0),
		},
		ColorSpace: map[pdf.Name]pdf.Object{},
		Pattern:    map[pdf.Name]pdf.Object{},
		Shading:    map[pdf.Name]pdf.Object{},
		XObject: map[pdf.Name]pdf.Object{
			"X": pdf.NewReference(2, 0),
		},
		Font: map[pdf.Name]pdf.Object{
			"F": pdf.NewReference(3, 0),
		},
	}

	f.Add("5 w\n")
	f.Add("/F 12 Tf\n")
	f.Add(`
	BT
	/F 12 Tf
	100 100 Td
	(Hello) Tj
	0 -15 Td
	(World) Tj
	ET
	`)
	f.Fuzz(func(t *testing.T, body string) {
		buf := &bytes.Buffer{}
		w := NewWriter(buf, pdf.V1_7)

		r := &Reader{
			R:         nil,
			Resources: res,
			State:     NewState(),
		}
		s := scanner.NewScanner()
		iter := s.Scan(strings.NewReader(body))
		iter(func(op string, args []pdf.Object) bool {
			err := r.UpdateState(op, args)
			if err != nil {
				t.Fatal(err)
			}

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

		doOps:
			switch op {
			case "w":
				x, ok := getNum()
				if ok {
					w.SetLineWidth(x)
				}
			case "J":
				x, ok := getInteger()
				if ok {
					w.SetLineCap(LineCapStyle(x))
				}
			case "j":
				x, ok := getInteger()
				if ok {
					w.SetLineJoin(LineJoinStyle(x))
				}
			case "M":
				x, ok := getNum()
				if ok {
					w.SetMiterLimit(x)
				}
			case "d":
				patObj, ok1 := getArray()
				pattern, ok2 := convertDashPattern(patObj)
				phase, ok3 := getNum()
				if ok1 && ok2 && ok3 {
					w.SetDashPattern(pattern, phase)
				}
			case "ri":
				name, ok := getName()
				if ok {
					w.SetRenderingIntent(name)
				}
			case "i":
				x, ok := getNum()
				if ok {
					w.SetFlatnessTolerance(x)
				}
			case "gs":
				name, ok := getName()
				if ok {
					ext, err := ReadExtGState(nil, res.ExtGState[name], name)
					if err == nil {
						w.SetExtGState(ext)
					}
				}
			case "q":
				w.PushGraphicsState()
			case "Q":
				if len(w.nesting) > 0 && w.nesting[len(w.nesting)-1] == pairTypeQ {
					w.PopGraphicsState()
				}

			case "cm":
				m := Matrix{}
				for i := 0; i < 6; i++ {
					f, ok := getNum()
					if !ok {
						break doOps
					}
					m[i] = f
				}
				w.Transform(m)

			case "BT":
				w.TextStart()
			case "ET":
				w.TextEnd()

			case "Tc":
				x, ok := getNum()
				if ok {
					w.TextSetCharacterSpacing(x)
				}
			case "Tw":
				x, ok := getNum()
				if ok {
					w.TextSetWordSpacing(x)
				}
			case "Tz":
				x, ok := getNum()
				if ok {
					w.TextSetHorizontalScaling(x)
				}
			case "TL":
				x, ok := getNum()
				if ok {
					w.TextSetLeading(x)
				}
			case "Tf":
				name, ok1 := getName()
				size, ok2 := getNum()
				ref, _ := res.Font[name].(pdf.Reference)
				if ok1 && ok2 {
					w.TextSetFont(&Res{DefName: "/" + name, Data: ref}, size)
				}
			case "Tr":
				x, ok := getInteger()
				if ok {
					w.TextSetRenderingMode(TextRenderingMode(x))
				}
			case "Ts":
				x, ok := getNum()
				if ok {
					w.TextSetRise(x)
				}

			case "Td": // Move text position
				tx, ok1 := getNum()
				ty, ok2 := getNum()
				if ok1 && ok2 {
					w.TextFirstLine(tx, ty)
				}

			case "TD": // Move text position and set leading
				tx, ok1 := getNum()
				ty, ok2 := getNum()
				if ok1 && ok2 {
					w.TextSecondLine(tx, ty)
				}

			case "Tm": // Set text matrix and line matrix
				m := Matrix{}
				for i := 0; i < 6; i++ {
					f, ok := getNum()
					if !ok {
						break doOps
					}
					m[i] = f
				}
				w.TextSetMatrix(m)

			// ---

			case "G":
				gray, ok := getNum()
				if ok {
					w.SetStrokeColor(color.Gray(gray))
				}
			case "k":
				cyan, ok1 := getNum()
				magenta, ok2 := getNum()
				yellow, ok3 := getNum()
				black, ok4 := getNum()
				if ok1 && ok2 && ok3 && ok4 {
					w.SetFillColor(color.CMYK(cyan, magenta, yellow, black))
				}
			}

			return true
		})
		if w.Err != nil {
			t.Fatal(w.Err)
		}
		state1 := r.State

		r = &Reader{
			R:         nil,
			Resources: w.Resources,
			State:     NewState(),
		}
		s = scanner.NewScanner()
		iter = s.Scan(bytes.NewReader(buf.Bytes()))
		iter(func(op string, args []pdf.Object) bool {
			err := r.UpdateState(op, args)
			if err != nil {
				t.Fatal(err)
			}
			return true
		})
		state2 := r.State

		if d := cmp.Diff(state1, state2); d != "" {
			fmt.Println("1:")
			fmt.Println(body)
			fmt.Println("2:")
			fmt.Println(buf.String())
			t.Errorf("state: %s", d)
		}
	})
}
