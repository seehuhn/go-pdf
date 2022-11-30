package main

import (
	"log"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pages"
)

func main() {
	w, err := pdf.Create("graphics.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	pageTree := pages.NewPageTree(w, nil)
	page, err := pageTree.NewPage(&pages.Attributes{
		MediaBox: pages.A4,
	})
	if err != nil {
		log.Fatal(err)
	}

	g := graphics.NewPage(page)

	x := 72.0
	y := page.BBox.URy - 72.0
	r := 50.0
	g.Circle(x, y, r)
	g.Stroke()

	x += 120
	g.MoveTo(x, y)
	g.LineToArc(x, y, r, 0, 1.5*math.Pi)
	g.CloseAndStroke()

	err = g.Close()
	if err != nil {
		log.Fatal(err)
	}

	err = page.Close()
	if err != nil {
		log.Fatal(err)
	}
}
