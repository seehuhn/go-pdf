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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pages"
)

func main() {
	w, err := pdf.Create("clock.pdf")
	if err != nil {
		log.Fatal(err)
	}

	pageTree := pages.NewTree(w, &pages.DefaultAttributes{
		MediaBox: pages.A4,
	})

	rmax := (pages.A4.URy - 72) / 2
	rr := []float64{
		4 / 25.4 * 72, // 4mm
		8 / 25.4 * 72, // 8mm
		rmax,
	}

	page, err := pageTree.NewPage(nil)
	if err != nil {
		log.Fatal(err)
	}
	page.Println(".5 w")
	// page.Println("1 0 1 RG")
	for _, r := range rr {
		semicircle(page, page.BBox.URx-72, page.BBox.URy/2, r, 1)
	}

	for i := 0; i <= 6; i++ {
		radius(page, page.BBox.URx-72, page.BBox.URy/2, rmax, i, 1)
	}
	err = page.Close()
	if err != nil {
		log.Fatal(err)
	}

	page, err = pageTree.NewPage(nil)
	if err != nil {
		log.Fatal(err)
	}
	page.Println(".5 w")
	// page.Println("1 0 1 RG")
	for _, r := range rr {
		semicircle(page, 72, page.BBox.URy/2, r, -1)
	}

	for i := 0; i <= 6; i++ {
		radius(page, 72, page.BBox.URy/2, rmax, i, -1)
	}
	err = page.Close()
	if err != nil {
		log.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func semicircle(page *pages.Page, x, y, r, sigma float64) {
	pi := math.Pi
	page.Printf("%.2f %.2f m\n", x, y-r)
	for i := 0; i <= 100; i++ {
		xi := x - sigma*r*math.Sin(float64(i)*pi/100)
		yi := y - r*math.Cos(float64(i)*pi/100)
		page.Printf("%.2f %.2f l\n", xi, yi)
	}
	page.Println("S")
}

func radius(page *pages.Page, x, y, rmax float64, i int, sigma float64) {
	x0 := x - 0.7*sigma*rmax*math.Sin(float64(i)*math.Pi/6)
	y0 := y - 0.7*rmax*math.Cos(float64(i)*math.Pi/6)
	x1 := x - sigma*rmax*math.Sin(float64(i)*math.Pi/6)
	y1 := y - rmax*math.Cos(float64(i)*math.Pi/6)
	page.Printf("%.2f %.2f m\n", x0, y0)
	page.Printf("%.2f %.2f l\n", x1, y1)
	page.Println("S")
}
