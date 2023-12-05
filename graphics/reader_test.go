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
			getNumber := func() (float64, bool) {
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

			err := r.UpdateState(op, args)
			if err != nil {
				t.Fatal(err)
			}

			switch op {
			case "w":
				x, ok := getNumber()
				if ok {
					w.SetLineWidth(x)
				}
			case "Tf":
				name, ok1 := getName()
				size, ok2 := getNumber()
				ref, _ := res.Font[name].(pdf.Reference)
				if ok1 && ok2 {
					w.TextSetFont(&Res{DefName: name, Data: ref}, size)
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
			fmt.Println(state1.TextFont)
			fmt.Println(state2.TextFont)
			fmt.Println(buf.String())
			t.Errorf("state: %s", d)
		}
	})
}
