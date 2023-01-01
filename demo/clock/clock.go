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
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
)

func main() {
	w, err := pdf.Create("clock.pdf")
	if err != nil {
		log.Fatal(err)
	}

	bbox := pages.A4
	pageTree := pages.InstallTree(w, &pages.InheritableAttributes{
		MediaBox: bbox,
	})

	rmax := (pages.A4.URy - 72) / 2
	rr := []float64{
		4 / 25.4 * 72, // 4mm
		8 / 25.4 * 72, // 8mm
		rmax,
	}

	page, err := graphics.NewPage(w)
	if err != nil {
		log.Fatal(err)
	}
	page.SetLineWidth(.5)
	for _, r := range rr {
		page.MoveToArc(bbox.URx-72, bbox.URy/2, r, 0.5*math.Pi, 1.5*math.Pi)
	}
	for i := 0; i <= 6; i++ {
		radius(page, bbox.URx-72, bbox.URy/2, rmax, i, 1)
	}
	page.Stroke()
	dict, err := page.Close()
	if err != nil {
		log.Fatal(err)
	}
	_, err = pageTree.AppendPage(dict)
	if err != nil {
		log.Fatal(err)
	}

	page, err = graphics.NewPage(w)
	if err != nil {
		log.Fatal(err)
	}
	page.SetLineWidth(.5)
	for _, r := range rr {
		page.MoveToArc(72, bbox.URy/2, r, 1.5*math.Pi, 2.5*math.Pi)
	}
	for i := 0; i <= 6; i++ {
		radius(page, 72, bbox.URy/2, rmax, i, -1)
	}
	page.Stroke()
	dict, err = page.Close()
	if err != nil {
		log.Fatal(err)
	}
	_, err = pageTree.AppendPage(dict)
	if err != nil {
		log.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func radius(page *graphics.Page, x, y, rmax float64, i int, sigma float64) {
	x0 := x - 0.7*sigma*rmax*math.Sin(float64(i)*math.Pi/6)
	y0 := y - 0.7*rmax*math.Cos(float64(i)*math.Pi/6)
	x1 := x - sigma*rmax*math.Sin(float64(i)*math.Pi/6)
	y1 := y - rmax*math.Cos(float64(i)*math.Pi/6)
	page.MoveTo(x0, y0)
	page.LineTo(x1, y1)
}
