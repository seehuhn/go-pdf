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

package graphics

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
)

func TestSetColor(t *testing.T) {
	type col struct {
		C color.Color
		S string
	}

	calGray, err := color.CalGray(color.WhitePointD65, nil, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	calRGB, err := color.CalRGB(color.WhitePointD65, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	colors := []col{
		{color.DeviceGray(0.8), "0.8 G\n"},
		{color.DeviceRGB(0.1, 0.2, 0.3), "0.1 0.2 0.3 RG\n"},
		{color.DeviceCMYK(0.1, 0.2, 0.3, 0.4), "0.1 0.2 0.3 0.4 K\n"},
		{calGray.New(0.8), "/CG CS\n0.8 SC\n"},
		{calRGB.New(0.4, 0.5, 0.6), "/CR CS\n0.4 0.5 0.6 SC\n"},
	}
	for i, ci := range colors {
		for j, cj := range colors {
			t.Run(fmt.Sprintf("S-%d-%d", i, j), func(t *testing.T) {
				data := pdf.NewData(pdf.V2_0)
				rm := pdf.NewResourceManager(data)
				buf := &bytes.Buffer{}
				w := NewWriter(buf, rm)

				writerSetResourceName(w, calGray, catColorSpace, "CG")
				writerSetResourceName(w, calRGB, catColorSpace, "CR")

				w.SetStrokeColor(ci.C)
				w.SetStrokeColor(cj.C)

				if i == j {
					// test that the test string occurs exactly once
					count := strings.Count(buf.String(), ci.S)
					if count != 1 {
						fmt.Println(buf.String())
						t.Errorf("Expected test string %q to occur once, but it occurred %d times", ci.S, count)
					}
				} else {
					// test that the test strings occur in order
					if !strings.Contains(buf.String(), ci.S+cj.S) {
						fmt.Println(buf.String())
						t.Errorf("Expected test strings %q and %q to occur in order, but they didn't", ci.S, cj.S)
					}
				}
			})
		}
	}
}
