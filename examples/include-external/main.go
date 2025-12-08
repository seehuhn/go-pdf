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
	"math"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/form"
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

	B := standard.TimesBold.New()
	F := standard.TimesRoman.New()

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

	cropBox := pageDict["CropBox"]
	if cropBox == nil {
		cropBox = pageDict["MediaBox"]
	}
	bbox, err := pdf.GetRectangle(r, cropBox)
	if err != nil {
		return nil, nil, err
	}

	// extract resources
	x := pdf.NewExtractor(r)
	var res *content.Resources
	if resObj := pageDict["Resources"]; resObj != nil {
		res, err = extract.Resources(x, resObj)
		if err != nil {
			return nil, nil, err
		}
	}
	if res == nil {
		res = &content.Resources{}
	}

	// read content stream
	stm, err := pagetree.ContentStream(r, pageDict)
	if err != nil {
		return nil, nil, err
	}

	stream, err := content.ReadStream(stm, pdf.GetVersion(r), content.Form)
	if err != nil {
		return nil, nil, err
	}

	obj := &form.Form{
		Content: stream,
		Res:     res,
		BBox:    *bbox,
	}

	return obj, bbox, nil
}
