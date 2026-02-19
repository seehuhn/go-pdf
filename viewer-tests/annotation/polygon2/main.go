// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
)

// This test creates a Polygon annotation with both Vertices and Path entries,
// to see which one viewers use for rendering.
//
// Layout: two light gray 120x120 background squares side by side.
// Vertices draws a 100x100 blue square centered on the left background square.
// Path draws a 100x100 blue square centered on the right background square.
// No appearance stream is included, so the viewer must synthesize one.

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	pageSize := &pdf.Rectangle{URx: 400, URy: 200}
	opt := &pdf.WriterOptions{HumanReadable: true}
	page, err := document.CreateSinglePage("test.pdf", pageSize, pdf.V1_7, opt)
	if err != nil {
		return err
	}

	// draw two light gray background squares
	page.SetFillColor(color.DeviceGray(0.85))
	page.Rectangle(50, 50, 120, 120)
	page.Rectangle(230, 50, 120, 120)
	page.Fill()

	// label the squares
	F := standard.Helvetica.New()
	page.SetFillColor(color.DeviceGray(0))
	page.TextBegin()
	page.TextSetFont(F, 12)
	page.TextSetMatrix(matrix.Translate(78, 177))
	page.TextShow("Vertices")
	page.TextSetMatrix(matrix.Translate(272, 177))
	page.TextShow("Path")
	page.TextEnd()

	// polygon annotation with both Vertices and Path
	page.Page.Annots = append(page.Page.Annots, &rawPolygon{})

	return page.Close()
}

// rawPolygon implements annotation.Annotation to produce a hand-built dict
// containing both Vertices and Path entries (which the library normally
// prevents).
type rawPolygon struct {
	annotation.Common
}

func (a *rawPolygon) AnnotationType() pdf.Name {
	return "Polygon"
}

func (a *rawPolygon) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	return pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Polygon"),
		"Rect":    &pdf.Rectangle{LLx: 50, LLy: 50, URx: 350, URy: 170},
		"F":       pdf.Integer(4), // Print
		"C":       pdf.Array{pdf.Number(0), pdf.Number(0), pdf.Number(1)},
		"IC":      pdf.Array{pdf.Number(0.8), pdf.Number(0.9), pdf.Number(1)},
		"Vertices": pdf.Array{
			pdf.Number(60), pdf.Number(60),
			pdf.Number(160), pdf.Number(60),
			pdf.Number(160), pdf.Number(160),
			pdf.Number(60), pdf.Number(160),
		},
		"Path": pdf.Array{
			pdf.Array{pdf.Number(240), pdf.Number(60)},
			pdf.Array{pdf.Number(340), pdf.Number(60)},
			pdf.Array{pdf.Number(340), pdf.Number(160)},
			pdf.Array{pdf.Number(240), pdf.Number(160)},
		},
		"BS": pdf.Dict{
			"W": pdf.Number(2),
			"S": pdf.Name("S"),
		},
	}, nil
}
