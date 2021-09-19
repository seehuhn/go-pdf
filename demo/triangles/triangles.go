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
	"fmt"
	"io"
	"log"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pages"
)

func norm(x, y float64) float64 {
	return math.Sqrt(x*x + y*y)
}

func arc(w io.Writer, x, y, radius, startAngle, endAngle float64) {
	// thanks to https://www.tinaja.com/glib/bezcirc2.pdf
	// TODO(voss): also have a look at https://pomax.github.io/bezierinfo/
	nSegment := int(math.Ceil(math.Abs(endAngle-startAngle) / (0.5 * math.Pi)))
	dPhi := (endAngle - startAngle) / float64(nSegment)
	x0 := math.Cos(dPhi / 2)
	y0 := math.Sin(dPhi / 2)
	x1 := (4 - x0) / 3
	y1 := (1 - x0) * (3 - x0) / (3 * y0)
	x2 := x1
	y2 := -y1
	x3 := x0
	y3 := -y0

	for i := 0; i < nSegment; i++ {
		// we need to rotate -dPhi/2 to startAngle+i*dPhi
		rot := startAngle + (float64(i)+.5)*dPhi
		cr := math.Cos(rot)
		sr := math.Sin(rot)
		// the rotation matrix is
		//    / cr -sr \
		//    \ sr  cr /
		fmt.Fprintf(w, "%f %f m %f %f %f %f %f %f c\n",
			x+(cr*x3-sr*y3)*radius, y+(sr*x3+cr*y3)*radius,
			x+(cr*x2-sr*y2)*radius, y+(sr*x2+cr*y2)*radius,
			x+(cr*x1-sr*y1)*radius, y+(sr*x1+cr*y1)*radius,
			x+(cr*x0-sr*y0)*radius, y+(sr*x0+cr*y0)*radius)
	}
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

	page, err := pages.SinglePage(w, &pages.Attributes{
		Resources: map[pdf.Name]pdf.Object{},
		MediaBox: &pdf.Rectangle{
			URx: 440,
			URy: mu + 60,
		},
		Rotate: 0,
	})
	if err != nil {
		log.Fatal(err)
	}

	page.Println("1 0 0 1 20 20 cm 1 j 1 J")
	page.Printf("2 w %f %f m 0 0 l 400 0 l %f %f l S\n", xL, yL, xR, yR)
	page.Printf("1 w 200 0 m 200 %f l S\n", mu+20)

	page.Println(".1 .1 1 RG")
	page.Printf("2 w %f %f m %f %f l S\n", xL, yL, xR, yR)
	page.Printf("1 w %f %f m %f %f l S\n",
		xM, yM, xM+(lambda+20)*nx, yM+(lambda+20)*ny)

	page.Printf("1 0 0 RG 2 w 0 0 m 200 %f l 400 0 l S\n", mu)
	page.Printf("0 0.8 0 RG 2 w %f %f m 200 %f l %f %f l S\n",
		xL, yL, mu, xR, yR)

	phi := math.Atan(mu / 200)
	page.Println("q 0 J 1 0 0 RG 5 w")
	arc(page, 0, 0, 20, 0, phi)
	arc(page, 400, 0, 20, math.Pi-phi, math.Pi)
	page.Println("S Q")

	psi := math.Pi/2 - math.Atan(lambda/(d/2))
	page.Println("q 0 J 0 0.8 0 RG 5 w")
	arc(page, 0, 0, 20, phi, phi+psi)
	arc(page, 400, 0, 30, math.Pi-phi, math.Pi-phi+psi)
	page.Println("S Q")

	// Draw black circles over the joining points of differently coloured
	// lines.
	pp := []struct{ x, y float64 }{
		{0, 0},
		{400, 0},
		{xL, yL},
		{xR, yR},
		{200, mu},
	}
	page.Println("0 G 2 w")
	for _, p := range pp {
		page.Printf("%f %f m %f %f l\n", p.x, p.y, p.x, p.y)
	}
	page.Println("S")

	err = page.Close()
	if err != nil {
		log.Fatal(err)
	}
}
