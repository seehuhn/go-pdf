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

// Command name-braces writes a PDF 2.0 file which probes how a viewer's
// content stream lexer treats the characters "{" and "}".
//
// In PDF 1.x these characters are delimiters everywhere.  In PDF 2.0
// (ISO 32000-2 §7.2.3, footnote to Table 2) they are delimiters only inside
// Type 4 functions, and are regular characters elsewhere.  The page content
// contains the bytes "/A{} Do".  Under the 2.0 rule this is one name, "A{}";
// under the 1.x rule it is the name "A" followed by two stray tokens.
//
// The resource dictionary holds two form XObjects.  The key "A{}" is written
// escaped, as /A#7b#7d, so both rules read the dictionary the same way.
package main

import (
	"fmt"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/text"
)

// page geometry
const (
	margin    = 72.0
	pageWidth = 595.0 // A4
	wrapWidth = pageWidth - 2*margin
)

// the marked area in which the form XObject appears
const (
	areaX = margin
	areaY = 630.0
	areaW = 200.0
	areaH = 70.0
)

func main() {
	if err := createDocument("test.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func createDocument(filename string) error {
	page, err := document.CreateSinglePage(filename, document.A4, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	titleFont := font.Must(standard.HelveticaBold.New())
	bodyFont := font.Must(standard.Helvetica.New())

	green := color.DeviceRGB{0.75, 0.92, 0.75}
	red := color.DeviceRGB{0.95, 0.75, 0.75}
	if err := page.Builder.RegisterXObject("A{}", makeBox(bodyFont, green, "PDF 2.0 rule")); err != nil {
		return err
	}
	if err := page.Builder.RegisterXObject("A", makeBox(bodyFont, red, "PDF 1.x rule")); err != nil {
		return err
	}

	drawText(page, titleFont, bodyFont)

	// marked area
	page.SetLineWidth(0.5)
	page.SetStrokeColor(color.DeviceGray(0.4))
	page.Rectangle(areaX-4, areaY-4, areaW+8, areaH+8)
	page.Stroke()

	// The name must reach the file with unescaped braces, which the
	// builder would escape, so the Do operator is emitted as raw bytes.
	page.PushGraphicsState()
	page.Transform(matrix.Translate(areaX, areaY))
	page.Builder.Stream = append(page.Builder.Stream, content.Operator{
		Name: content.OpRawContent,
		Args: []pdf.Object{pdf.String("/A{} Do")},
	})
	page.PopGraphicsState()

	return page.Close()
}

// makeBox returns a form XObject which fills the marked area with a coloured
// box and a label.
func makeBox(F font.Layouter, fill color.DeviceRGB, label string) *form.Form {
	b := builder.New(content.Form, nil, pdf.V2_0)
	b.SetFillColor(fill)
	b.Rectangle(0, 0, areaW, areaH)
	b.Fill()

	const size = 16.0
	b.SetFillColor(color.DeviceGray(0))
	b.TextBegin()
	b.TextSetFont(F, size)
	b.TextFirstLine(0, pdf.Round((areaH-F.GetGeometry().CapHeight*size)/2, 2))
	b.TextShowAligned(label, areaW, 0.5)
	b.TextEnd()

	return &form.Form{
		Content: &content.Operators{Ops: b.Stream},
		Res:     b.Resources,
		BBox:    pdf.Rectangle{URx: areaW, URy: areaH},
	}
}

func drawText(page *document.Page, titleFont, bodyFont font.Instance) {
	title := text.F{Font: titleFont, Size: 14, Color: color.DeviceGray(0)}
	body := text.F{Font: bodyFont, Size: 10, Color: color.DeviceGray(0.1)}

	text.Show(page.Builder,
		text.M{X: margin, Y: 790},
		title, "Braces in names — viewer behaviour probe",
	)
	text.Show(page.Builder,
		text.M{X: margin, Y: 765},
		body,
		text.Wrap(wrapWidth,
			"The content stream of this PDF 2.0 page contains the bytes",
			"/A{} Do.  In PDF 1.x, { and } are delimiters, so this reads as",
			"the name /A followed by the tokens { and }.  In PDF 2.0, they",
			"are delimiters only inside Type 4 functions, so /A{} is a",
			"single name (ISO 32000-2, section 7.2.3).  The resources hold",
			"a different form XObject for each reading.",
		),
	)

	text.Show(page.Builder,
		text.M{X: margin, Y: 580},
		title, "How to read the result",
	)
	results := []string{
		"Green box: the viewer follows the PDF 2.0 rule and reads one name, /A{}.",
		"Red box: the viewer splits the name at the brace, skips the brace tokens and runs /A Do.",
		"No box: the viewer splits the name and treats the braces as unknown operators, which consume /A, or it abandons the content stream.",
	}
	for i, r := range results {
		text.Show(page.Builder,
			text.M{X: margin, Y: 560 - 30*float64(i)},
			body, text.Wrap(wrapWidth, r),
		)
	}
}
