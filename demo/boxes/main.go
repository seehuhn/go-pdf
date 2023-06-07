package main

import (
	"fmt"
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/color"
	"seehuhn.de/go/pdf/font/builtin"
	"seehuhn.de/go/pdf/graphics"
)

func main() {
	err := doit()
	if err != nil {
		log.Fatal(err)
	}
}

func doit() error {
	w, err := pdf.Create("boxes.pdf", nil)
	if err != nil {
		return err
	}

	F, err := builtin.Embed(w, builtin.Helvetica, "F")
	if err != nil {
		return err
	}
	geom := F.GetGeometry()

	// Draw the contents of the page.
	cRef := w.Alloc()
	c, err := w.OpenStream(cRef, nil, pdf.FilterCompress{})
	if err != nil {
		return err
	}
	page := graphics.NewPage(c)
	// draw a grid to show page coordinates
	page.PushGraphicsState()
	page.SetStrokeColor(color.Gray(0.85))
	for z := 0.0; z <= 600+1e-6; z += 50 {
		page.MoveTo(z, 0)
		page.LineTo(z, 600)
		page.MoveTo(0, z)
		page.LineTo(600, z)
	}
	page.Stroke()
	page.SetFillColor(color.Gray(1))
	for _, x := range []float64{82, 532} {
		for i := 0; i <= 600; i += 50 {
			gg := F.Layout(fmt.Sprintf("%d", i), 9)
			b := geom.BoundingBox(9, gg)
			page.Rectangle(x-b.URx-1, float64(i)-3+b.LLy, b.URx-b.LLx+2, b.URy-b.LLy)
		}
	}
	for _, y := range []float64{72, 522} {
		for i := 0; i <= 600; i += 50 {
			gg := F.Layout(fmt.Sprintf("%d", i), 9)
			b := geom.BoundingBox(9, gg)
			w := b.URx - b.LLx
			page.Rectangle(float64(i)-0.5*w, y+b.LLy-1, w, b.URy-b.LLy+2)
		}
	}
	page.Fill()
	page.SetFillColor(color.Gray(0.6))
	page.TextSetFont(F, 9)
	for _, x := range []float64{82, 532} {
		page.TextStart()
		for i := 0; i <= 600; i += 50 {
			switch i {
			case 0:
				page.TextFirstLine(x, -3)
			case 50:
				page.TextSecondLine(0, 50)
			default:
				page.TextNewLine()
			}
			page.TextShowAligned(fmt.Sprintf("%d", i), 0, 1)
		}
		page.TextEnd()
	}
	for _, y := range []float64{72, 522} {
		page.TextStart()
		for i := 0; i <= 600; i += 50 {
			gg := F.Layout(fmt.Sprintf("%d", i), 9)
			switch i {
			case 0:
				page.TextFirstLine(0, y)
			default:
				page.TextFirstLine(50, 0)
			}
			page.TextShowGlyphsAligned(gg, 0, 0.5)
		}
		page.TextEnd()
	}
	page.PopGraphicsState()

	page.TextSetFont(F, 12)

	page.TextStart()
	page.TextFirstLine(60, 574)
	page.TextShow("This text is outside the CropBox.  It will not be visible on most PDF viewers.")
	page.TextEnd()

	page.TextStart()
	page.TextFirstLine(60, 480)
	page.TextShow("Every PDF page has a MediaBox.  The MediaBox is the largest page box.")
	page.TextSecondLine(0, -geom.ToPDF16(12, geom.BaseLineSkip))
	page.TextShow("On this page, the MediaBox is the rectangle [0,0]×[600,600].")
	page.TextEnd()

	page.TextStart()
	page.TextFirstLine(60, 430)
	page.TextShow("Inside the MediaBox is the CropBox.  PDF viewers should only display the CropBox.")
	page.TextSecondLine(0, -geom.ToPDF16(12, geom.BaseLineSkip))
	page.TextShow("On this page, the CropBox is the rectangle [50,50]×[550,550].")
	page.TextEnd()

	err = c.Close()
	if err != nil {
		return err
	}

	// Manually construct a page tree, so that we can test inheritance
	// of the /MediaBox and /CropBox attributes.
	rootRef := w.Alloc()
	midRef := w.Alloc()
	pageRef := w.Alloc()
	w.Put(rootRef, pdf.Dict{
		"Type":    pdf.Name("Pages"),
		"Kids":    pdf.Array{midRef},
		"Count":   pdf.Integer(1),
		"CropBox": &pdf.Rectangle{LLx: 50, LLy: 50, URx: 550, URy: 550},
	})
	w.Put(midRef, pdf.Dict{
		"Type":     pdf.Name("Pages"),
		"Parent":   rootRef,
		"Kids":     pdf.Array{pageRef},
		"Count":    pdf.Integer(1),
		"MediaBox": &pdf.Rectangle{LLx: 0, LLy: 0, URx: 600, URy: 600},
	})
	w.Put(pageRef, pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Parent":    midRef,
		"Contents":  cRef,
		"Resources": pdf.AsDict(page.Resources),

		"BleedBox": &pdf.Rectangle{LLx: 100, LLy: 50, URx: 550, URy: 550},
		"TrimBox":  &pdf.Rectangle{LLx: 50, LLy: 100, URx: 550, URy: 550},
		"ArtBox":   &pdf.Rectangle{LLx: 150, LLy: 150, URx: 550, URy: 550},

		"BoxColorInfo": pdf.Dict{
			"CropBox": pdf.Dict{
				"C": pdf.Array{pdf.Real(166.0 / 255), pdf.Real(97.0 / 255), pdf.Real(26.0 / 255)},
				"W": pdf.Integer(2),
			},
			"BleedBox": pdf.Dict{
				"C": pdf.Array{pdf.Real(223.0 / 255), pdf.Real(194.0 / 255), pdf.Real(125.0 / 255)},
				"W": pdf.Integer(2),
			},
			"TrimBox": pdf.Dict{
				"C": pdf.Array{pdf.Real(128.0 / 255), pdf.Real(205.0 / 255), pdf.Real(193.0 / 255)},
				"W": pdf.Integer(2),
			},
			"ArtBox": pdf.Dict{
				"C": pdf.Array{pdf.Real(1.0 / 255), pdf.Real(133.0 / 255), pdf.Real(113.0 / 255)},
				"W": pdf.Integer(2),
			},
		},
	})
	w.GetMeta().Catalog.Pages = rootRef

	return w.Close()
}
