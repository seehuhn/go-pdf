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

package main

import (
	"log"

	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics"
)

func main() {
	page, err := document.CreateSinglePage("test.pdf", document.A4, nil)
	if err != nil {
		log.Fatal(err)
	}

	page.SetLineWidth(5)
	page.SetStrokeColor(graphics.DeviceGray.New(0.5))

	page.MoveTo(100, 100)
	page.LineTo(120, 200)
	page.LineTo(120, 300)
	page.LineTo(100, 400)
	page.ClosePath()
	page.LineTo(400, 250)
	page.Stroke()

	page.Rectangle(100, 500, 200, 200)
	page.LineTo(400, 600)
	page.Stroke()

	err = page.Close()
	if err != nil {
		log.Fatal(err)
	}
}
