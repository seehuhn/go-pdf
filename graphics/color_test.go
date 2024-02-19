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

	calGray, err := graphics.CalGray(graphics.WhitePointD65, nil, 1.0, "")
	if err != nil {
		t.Fatal(err)
	}

	if err != nil {
		t.Fatal(err)
	}
	w := (paper.Dx() - 72) / 10
	for i := 0; i < 2; i++ {
		y := paper.URy - 36 - float64(i+1)*w
		for j := 0; j < 10; j++ {
			x := 36 + float64(j)*w
			var col graphics.Color
			if i == 0 {
				col = graphics.DeviceGrayNew(float64(j) / 9)
			} else {
				col = calGray.New(float64(j) / 9)
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
