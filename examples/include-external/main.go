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
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/reader/scanner"
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

	// Create a resource manager for embedding external content
	rm := pdf.NewResourceManager(page.Out)
	defer rm.Close()

	figure, bbox, err := LoadFigure("fig.pdf", rm)
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
func LoadFigure(fname string, rm *pdf.ResourceManager) (graphics.XObject, *pdf.Rectangle, error) {
	r, err := pdf.Open(fname, nil)
	if err != nil {
		return nil, nil, err
	}
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

	// Copy resources from the original PDF
	copier := pdf.NewCopier(rm.Out, r)

	origResources, err := pdf.GetDict(r, pageDict["Resources"])
	if err != nil {
		return nil, nil, err
	}
	resourceObj, err := copier.Copy(origResources)
	if err != nil {
		return nil, nil, err
	}
	resources, err := pdf.ExtractResources(nil, resourceObj)
	if err != nil {
		return nil, nil, err
	}

	// Parse content stream operators
	contents, err := pagetree.ContentStream(r, pageDict)
	if err != nil {
		return nil, nil, err
	}

	s := scanner.NewScanner()
	s.SetInput(contents)

	var operators []graphics.Operator
	for s.Scan() {
		op := s.Operator()
		// Convert scanner.Operator to graphics.Operator
		// Note: All operator arguments in content streams should be Native types
		args := make([]pdf.Native, len(op.Args))
		for i, arg := range op.Args {
			// Content stream operators should only have Native arguments
			native, ok := arg.(pdf.Native)
			if !ok {
				return nil, nil, pdf.Errorf("unexpected non-native operator argument: %T", arg)
			}
			args[i] = native
		}
		operators = append(operators, graphics.Operator{
			Name: pdf.Name(op.Name),
			Args: args,
		})
	}
	if s.Error() != nil {
		return nil, nil, s.Error()
	}

	err = r.Close()
	if err != nil {
		return nil, nil, err
	}

	contentStream := &graphics.ContentStream{
		Resources: resources,
		Operators: operators,
	}

	obj := &form.Form{
		Content: contentStream,
		BBox:    *bbox,
	}

	return obj, bbox, nil
}
