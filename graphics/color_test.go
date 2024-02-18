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

package graphics_test

import (
	"testing"

	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics"
)

func TestCalGray(t *testing.T) {
	paper := document.A4

	page, err := document.CreateSinglePage("test.pdf", paper, nil)
	if err != nil {
		t.Fatal(err)
	}

	cs1 := graphics.DeviceGray
	cs2, err := graphics.CalGray(graphics.WhitePointD65, nil, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	w := (paper.Dx() - 72) / 10
	for i, cs := range []graphics.ColorSpace{cs1, cs2} {
		y := paper.URy - 36 - float64(i+1)*w
		for j := 0; j < 10; j++ {
			x := 36 + float64(j)*w
			col := graphics.Color{
				CS:     cs,
				Values: []float64{float64(j) / 9},
			}
			page.SetFillColor(col)
			page.Rectangle(x, y, w, w)
			page.Fill()
		}
	}

	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestCalRGB(t *testing.T) {
	paper := document.A4

	page, err := document.CreateSinglePage("test.pdf", paper, nil)
	if err != nil {
		t.Fatal(err)
	}

	cs1 := graphics.DeviceRGB
	cs2, err := graphics.CalRGB(graphics.WhitePointD65, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := (paper.Dx() - 72) / 10
	for i, cs := range []graphics.ColorSpace{cs1, cs2} {
		y := paper.URy - 36 - float64(i+1)*w
		for j := 0; j < 10; j++ {
			x := 36 + float64(j)*w
			col := graphics.Color{
				CS:     cs,
				Values: []float64{float64(j) / 19, 0, 0},
			}
			page.SetFillColor(col)
			page.Rectangle(x, y, w, w)
			page.Fill()
		}
	}

	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}
}
