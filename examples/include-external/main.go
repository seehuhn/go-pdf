// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"log"
	"math"
	"slices"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
	pdfpage "seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	paper := document.A4
	page, err := document.CreateSinglePage("test.pdf", paper, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	B := font.Must(standard.TimesBold.New())
	F := font.Must(standard.TimesRoman.New())

	figure, bbox, err := LoadFigure("fig.pdf")
	if err != nil {
		return err
	}

	width := bbox.Dx()
	height := bbox.Dy()
	base := paper.URy - 72 - height
	left := paper.LLx + 0.5*(paper.Dx()-width)
	page.PushGraphicsState()
	page.Transform(matrix.Translate(math.Round(left-bbox.LLx), math.Round(base-bbox.LLy)))
	page.DrawXObject(figure)
	page.PopGraphicsState()

	base -= 12
	page.TextBegin()
	page.TextFirstLine(72, base)
	page.TextSetFont(B, 10)
	page.TextShow("Figure 1.  ")
	page.TextSetFont(F, 10)
	page.TextShow("A grid of pair scatter plots for R's built-in iris dataset.  The plot illustrates the joint distribution")
	page.TextSecondLine(0, -12)
	page.TextShow("of sepal length, sepal width, petal length, and petal width for three species of iris.")
	page.TextEnd()

	err = page.Close()
	if err != nil {
		return err
	}
	return nil
}

// LoadFigure loads the first page of a PDF file as a form XObject.
func LoadFigure(fname string) (graphics.XObject, *pdf.Rectangle, error) {
	r, err := pdf.Open(fname, nil)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()

	_, pageDict, err := pagetree.GetPage(r, 0)
	if err != nil {
		return nil, nil, err
	}

	c := pdf.NewCursor(r)
	cropBox := pageDict["CropBox"]
	if cropBox == nil {
		cropBox = pageDict["MediaBox"]
	}
	bbox, err := c.Rectangle(cropBox)
	if err != nil {
		return nil, nil, err
	}

	x := pdf.NewExtractor(r)
	pg, err := pdf.CursorAt(x, nil).Decode(pageDict, pdfpage.Decode)
	if err != nil {
		return nil, nil, err
	}
	res := pg.Resources
	if res == nil {
		res = &content.Resources{}
	}

	// materialise the page's content stream into an Operators slice
	var ops []content.Operator
	it := pg.NewIter()
	for name, args := range it.All() {
		ops = append(ops, content.Operator{Name: name, Args: slices.Clone(args)})
	}
	if err := it.Err(); err != nil {
		return nil, nil, err
	}

	obj := &form.Form{
		Content: &content.Operators{Ops: ops},
		Res:     res,
		BBox:    *bbox,
	}

	return obj, bbox, nil
}
