// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
)

func norm(x, y float64) float64 {
	return math.Sqrt(x*x + y*y)
}

func main() {
	w, err := pdf.Create("triangles.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	R := 150.0
	alpha := 60. / 360. * 2. * math.Pi

	xL := 0.0
	yL := R
	xR := 400 - R*math.Cos(alpha)
	yR := R * math.Sin(alpha)

	d := norm(xR-xL, yR-yL)
	nx := -(yR - yL) / d
	ny := (xR - xL) / d
	xM := (xL + xR) / 2
	yM := (yL + yR) / 2

	// find intersection
	// 200 == xM + lambda * nx
	// mu == yM + lambda * ny
	lambda := (200 - xM) / nx
	mu := yM + lambda*ny

	pageTree := pages.NewPageTree(w, nil)
	page, err := pageTree.NewPage(&pages.Attributes{
		MediaBox: &pdf.Rectangle{
			URx: 440,
			URy: mu + 60,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	g := graphics.NewPage(page)
	g.Translate(20, 20)
	g.SetLineJoin(graphics.LineJoinRound)
	g.SetLineCap(graphics.LineCapRound)

	g.SetLineWidth(2)
	g.MoveTo(xL, yL)
	g.LineTo(0, 0)
	g.LineTo(400, 0)
	g.LineTo(xR, yR)
	g.Stroke()

	g.SetLineWidth(1)
	g.MoveTo(200, 0)
	g.LineTo(200, mu+20)
	g.Stroke()

	g.SetStrokeRGB(.1, .1, 1)
	g.SetLineWidth(2)
	g.MoveTo(xL, yL)
	g.LineTo(xR, yR)
	g.Stroke()
	g.SetLineWidth(1)
	g.MoveTo(xM, yM)
	g.LineTo(xM+(lambda+20)*nx, yM+(lambda+20)*ny)
	g.Stroke()

	g.SetStrokeRGB(1, 0, 0)
	g.SetLineWidth(2)
	g.MoveTo(0, 0)
	g.LineTo(200, mu)
	g.LineTo(400, 0)
	g.Stroke()
	g.SetStrokeRGB(0, 0.8, 0)
	g.SetLineWidth(2)
	g.MoveTo(xL, yL)
	g.LineTo(200, mu)
	g.LineTo(xR, yR)
	g.Stroke()

	phi := math.Atan(mu / 200)
	g.PushGraphicsState()
	g.SetLineCap(graphics.LineCapButt)
	g.SetStrokeRGB(1, 0, 0)
	g.SetLineWidth(5)
	g.MoveToArc(0, 0, 20, 0, phi)
	g.MoveToArc(400, 0, 20, math.Pi-phi, math.Pi)
	g.Stroke()
	g.PopGraphicsState()

	psi := math.Pi/2 - math.Atan(lambda/(d/2))
	g.PushGraphicsState()
	g.SetLineCap(graphics.LineCapButt)
	g.SetStrokeRGB(0, 0.8, 0)
	g.SetLineWidth(5)
	g.MoveToArc(0, 0, 20, phi, phi+psi)
	g.MoveToArc(400, 0, 30, math.Pi-phi, math.Pi-phi+psi)
	g.Stroke()
	g.PopGraphicsState()

	// Draw black circles over the joining points of differently coloured
	// lines.
	pp := []struct{ x, y float64 }{
		{0, 0},
		{400, 0},
		{xL, yL},
		{xR, yR},
		{200, mu},
	}
	g.SetStrokeGray(0)
	g.SetLineWidth(2)
	for _, p := range pp {
		g.MoveTo(p.x, p.y)
		g.LineTo(p.x, p.y)
	}
	g.Stroke()

	err = g.Close()
	if err != nil {
		log.Fatal(err)
	}

	err = page.Close()
	if err != nil {
		log.Fatal(err)
	}
}
