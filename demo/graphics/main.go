// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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
	"math"

	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/pagetree"
)

func main() {
	bbox := pagetree.A4

	w, err := document.CreateSinglePage("graphics.pdf", bbox.URx, bbox.URy, nil)
	if err != nil {
		log.Fatal(err)
	}

	F, err := builtin.Embed(w.Out, builtin.Helvetica, "F")
	if err != nil {
		log.Fatal(err)
	}

	x := 72.0
	y := bbox.URy - 72.0
	r := 50.0
	w.Circle(x, y, r)
	w.Stroke()

	x += 120
	w.MoveTo(x, y)
	w.LineToArc(x, y, r, 0, 1.5*math.Pi)
	w.CloseAndStroke()

	x = 72
	y -= 72
	w.BeginText()
	w.SetFont(F, 12)
	w.StartLine(x, y)
	w.ShowText("AWAY again")
	w.StartNextLine(0, -15)
	w.ShowText("line 2")
	w.NewLine()
	w.ShowText("line 3")
	w.EndText()

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}
