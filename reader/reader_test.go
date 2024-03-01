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

package reader

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/sfnt/cff"

	"seehuhn.de/go/pdf"
	pdffont "seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/matrix"
	"seehuhn.de/go/pdf/internal/dummyfont"
	"seehuhn.de/go/pdf/reader/scanner"
)

func FuzzReader(f *testing.F) {
	data := pdf.NewData(pdf.V1_7)
	F := dummyfont.Embed(data, "F")

	res := &pdf.Resources{
		ExtGState: map[pdf.Name]pdf.Object{
			"G": data.Alloc(),
		},
		ColorSpace: map[pdf.Name]pdf.Object{},
		Pattern:    map[pdf.Name]pdf.Object{},
		Shading:    map[pdf.Name]pdf.Object{},
		XObject: map[pdf.Name]pdf.Object{
			"X": data.Alloc(),
		},
		Font: map[pdf.Name]pdf.Object{
			"F": F.PDFObject(),
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
		// Use a new writer to try to replicate the read state.
		buf := &bytes.Buffer{}
		w := graphics.NewWriter(buf, pdf.V1_7)

		// Read from content stream from 'body'
		r := New(pdf.NewData(pdf.V1_7), nil)
		r.State = graphics.NewState()
		r.stack = r.stack[:0]
		r.Resources = res
		r.EveryOp = func(op string, args []pdf.Object) error {
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
					w.SetLineCap(graphics.LineCapStyle(x))
				}
			case "j":
				x, ok := getInteger()
				if ok {
					w.SetLineJoin(graphics.LineJoinStyle(x))
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
					w.SetRenderingIntent(graphics.RenderingIntent(name))
				}
			case "i":
				x, ok := getNum()
				if ok {
					w.SetFlatnessTolerance(x)
				}
			case "gs":
				name, ok := getName()
				if ok {
					ext, err := r.ReadExtGState(res.ExtGState[name], name)
					if err == nil {
						w.SetExtGState(ext)
					}
				}
			case "q":
				w.PushGraphicsState()
			case "Q":
				w.PopGraphicsState()

			case "cm":
				m := matrix.Matrix{}
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
					w.TextSetHorizontalScaling(x / 100)
				}
			case "TL":
				x, ok := getNum()
				if ok {
					w.TextSetLeading(x)
				}
			case "Tf":
				name, ok1 := getName()
				size, ok2 := getNum()
				F, err := r.ReadFont(res.Font[name], name)
				if pdf.IsMalformed(err) {
					break
				} else if err != nil {
					t.Fatal(err)
				}
				if ok1 && ok2 {
					w.TextSetFont(F, size)
				}
			case "Tr":
				x, ok := getInteger()
				if ok {
					w.TextSetRenderingMode(graphics.TextRenderingMode(x))
				}
			case "Ts":
				x, ok := getNum()
				if ok {
					w.TextSetRise(x)
				}

			case "Td": // Move text position
				dx, ok1 := getNum()
				dy, ok2 := getNum()
				if ok1 && ok2 {
					w.TextFirstLine(dx, dy)
				}

			case "TD": // Move text position and set leading
				dx, ok1 := getNum()
				dy, ok2 := getNum()
				if ok1 && ok2 {
					w.TextSecondLine(dx, dy)
				}

			case "Tm": // Set text matrix and line matrix
				m := matrix.Matrix{}
				for i := 0; i < 6; i++ {
					f, ok := getNum()
					if !ok {
						break doOps
					}
					m[i] = f
				}
				w.TextSetMatrix(m)

			case "T*": // Move to start of next text line
				w.TextNextLine()

			// ---

			case "G":
				gray, ok := getNum()
				if ok {
					w.SetStrokeColor(color.DeviceGray.New(gray))
				}

			case "g":
				gray, ok := getNum()
				if ok {
					w.SetFillColor(color.DeviceGray.New(gray))
				}

			case "k":
				cyan, ok1 := getNum()
				magenta, ok2 := getNum()
				yellow, ok3 := getNum()
				black, ok4 := getNum()
				if ok1 && ok2 && ok3 && ok4 {
					w.SetFillColor(color.DeviceCMYK.New(cyan, magenta, yellow, black))
				}
			}

			if w.Err != nil {
				return w.Err
			}
			return nil
		}
		err := r.ParseContentStream(strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		state1 := r.State

		r.scanner.Reset()
		r.State = graphics.NewState()
		r.stack = r.stack[:0]
		r.Resources = w.Resources

		r.EveryOp = nil
		replicate := buf.String()
		err = r.ParseContentStream(buf)
		if err != nil {
			t.Fatal(err)
		}
		state2 := r.State

		if d := cmp.Diff(state1, state2); d != "" {
			fmt.Println("1:")
			fmt.Println(body)
			fmt.Println("2:")
			fmt.Println(replicate)
			t.Errorf("state: %s", d)
		}
	})
}

func TestParameters(t *testing.T) {
	buf := &bytes.Buffer{}
	w := graphics.NewWriter(buf, pdf.V1_7)
	w.Set = 0

	data := pdf.NewData(pdf.V1_7)
	font := dummyfont.Embed(data, "dummy")

	w.SetLineWidth(12.3)
	w.SetLineCap(graphics.LineCapRound)
	w.SetLineJoin(graphics.LineJoinBevel)
	w.SetMiterLimit(4)
	w.SetDashPattern([]float64{5, 6, 7}, 8)
	w.SetRenderingIntent(graphics.Perceptual)
	w.SetFlatnessTolerance(10)
	m := matrix.Matrix{1, 2, 3, 4, 5, 6}
	w.Transform(m)
	w.TextSetCharacterSpacing(9)
	w.TextSetWordSpacing(10)
	w.TextSetHorizontalScaling(11)
	w.TextSetLeading(12)
	w.TextSetFont(font, 14)
	w.TextSetRenderingMode(graphics.TextRenderingModeFillStrokeClip)
	w.TextSetRise(15)

	r := New(data, nil)
	r.Resources = w.Resources
	r.State = graphics.NewState()
	r.Set = 0
	s := scanner.NewScanner()
	iter := s.Scan(bytes.NewReader(buf.Bytes()))
	err := iter(func(op string, args []pdf.Object) error {
		err := r.do(op, args)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if r.State.LineWidth != 12.3 {
		t.Errorf("LineWidth: got %v, want 12.3", r.State.LineWidth)
	}
	if r.State.LineCap != graphics.LineCapRound {
		t.Errorf("LineCap: got %v, want %v", r.State.LineCap, graphics.LineCapRound)
	}
	if r.State.LineJoin != graphics.LineJoinBevel {
		t.Errorf("LineJoin: got %v, want %v", r.State.LineJoin, graphics.LineJoinBevel)
	}
	if r.State.MiterLimit != 4 {
		t.Errorf("MiterLimit: got %v, want 4", r.State.MiterLimit)
	}
	if d := cmp.Diff(r.State.DashPattern, []float64{5, 6, 7}); d != "" {
		t.Errorf("DashPattern: %s", d)
	}
	if r.State.DashPhase != 8 {
		t.Errorf("DashPhase: got %v, want 8", r.State.DashPhase)
	}
	if r.State.RenderingIntent != graphics.Perceptual {
		t.Errorf("RenderingIntent: got %v, want %v", r.State.RenderingIntent, graphics.Perceptual)
	}
	if r.State.FlatnessTolerance != 10 {
		t.Errorf("Flatness: got %v, want 10", r.State.FlatnessTolerance)
	}
	if r.State.CTM != m {
		t.Errorf("CTM: got %v, want %v", r.State.CTM, m)
	}
	if r.State.TextCharacterSpacing != 9 {
		t.Errorf("Tc: got %v, want 9", r.State.TextCharacterSpacing)
	}
	if r.State.TextWordSpacing != 10 {
		t.Errorf("Tw: got %v, want 10", r.State.TextWordSpacing)
	}
	if r.State.TextHorizontalScaling != 11 {
		t.Errorf("Th: got %v, want 11", r.State.TextHorizontalScaling)
	}
	if r.State.TextLeading != 12 {
		t.Errorf("Tl: got %v, want 12", r.State.TextLeading)
	}
	if !resEqual(r.State.TextFont, font) || r.State.TextFontSize != 14 { // TODO(voss)
		t.Errorf("Font: got %v, %v, want %v, 14", r.State.TextFont, r.State.TextFontSize, font)
	}
	if r.State.TextRenderingMode != graphics.TextRenderingModeFillStrokeClip {
		t.Errorf("TextRenderingMode: got %v, want %v", r.State.TextRenderingMode, graphics.TextRenderingModeFillStrokeClip)
	}
	if r.State.TextRise != 15 {
		t.Errorf("Tr: got %v, want 15", r.State.TextRise)
	}

	cmpFDSelectFn := cmp.Comparer(func(fn1, fn2 cff.FDSelectFn) bool {
		return true
	})
	cmpFont := cmp.Comparer(func(f1, f2 pdffont.Embedded) bool {
		if f1.PDFObject() != f2.PDFObject() {
			return false
		}
		if f1.DefaultName() != f2.DefaultName() {
			return false
		}
		if f1.WritingMode() != f2.WritingMode() {
			return false
		}
		// TODO(voss): add more checks?
		return true
	})

	if d := cmp.Diff(w.State, r.State, cmpFDSelectFn, cmpFont); d != "" {
		t.Errorf("State: %s", d)
	}
}

func resEqual(a, b pdf.Resource) bool {
	return a.DefaultName() == b.DefaultName() && a.PDFObject() == b.PDFObject()
}
