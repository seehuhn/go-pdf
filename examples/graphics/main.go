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
	"fmt"
	"log"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/graphics"
)

func main() {
	bbox := document.A4

	w, err := document.CreateSinglePage("graphics.pdf", bbox, pdf.V1_7, nil)
	if err != nil {
		log.Fatal(err)
	}

	F, err := type1.Helvetica.Embed(w.Out, nil)
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
	w.TextBegin()
	w.TextSetFont(F, 12)
	w.TextFirstLine(x, y)
	w.TextShow("AWAY again")
	w.TextSecondLine(0, -15)
	w.TextShow("line 2")
	w.TextNextLine()
	w.TextShow("line 3")
	w.TextEnd()

	y -= 80
	w.PushGraphicsState()
	w.SetLineWidth(10)
	w.SetLineJoin(graphics.LineJoinMiter)
	w.SetMiterLimit(1.414)
	for i, phi := range []float64{85, 87, 89, 91, 93, 95} {
		x = 72 + float64(i)*72
		w.MoveTo(x-30, y)
		w.LineTo(x, y)
		phiRad := phi * math.Pi / 180
		w.LineTo(x+30*math.Cos(phiRad), y+30*math.Sin(phiRad))
	}
	w.Stroke()
	w.PopGraphicsState()
	w.TextBegin()
	w.TextSetFont(F, 9)
	for i, phi := range []float64{85, 87, 89, 91, 93, 95} {
		switch i {
		case 0:
			w.TextFirstLine(42, y-17)
		default:
			w.TextFirstLine(72, 0)
		}
		w.TextShow(fmt.Sprintf("phi = %gº", phi))
	}
	w.TextEnd()

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}
