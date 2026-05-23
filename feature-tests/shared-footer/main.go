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

// shared-footer demonstrates the canonical pattern for putting the same
// footer (or header / watermark / signature line / …) on every page of a
// multi-page document, while having the PDF file contain exactly one
// shared content-stream object referenced by every page.
//
// The recipe has three steps:
//
//  1. Build the footer's operators once and wrap them in a *content.Operators.
//     The wrapper carries pointer identity, which is what the resource
//     manager uses to deduplicate on write.
//
//  2. Register the footer's font under a fixed name with each page's
//     builder.  The shared footer bytes reference that name; the page's
//     /Resources must bind it.
//
//  3. Append the shared *content.Operators as an additional content-stream
//     segment on each page via Page.AppendContent.
package main

import (
	"fmt"
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
)

const numPages = 5

// footerFontName is the resource-dictionary key the footer's content
// stream references.  Every page that uses the shared footer must bind
// the same font under this name.
const footerFontName pdf.Name = "FootFont"

func main() {
	if err := doit("test.pdf"); err != nil {
		log.Fatal(err)
	}
}

func doit(fname string) error {
	doc, err := document.CreateMultiPage(fname, document.A4, pdf.V1_7, nil)
	if err != nil {
		return err
	}
	doc.Out.GetMeta().Info = &pdf.Info{
		Title:    "Shared footer demo",
		Subject:  "One content stream referenced from every page",
		Creator:  "seehuhn.de/go/pdf/feature-tests/shared-footer",
		Producer: "Quire",
	}

	footerFont := font.Must(standard.HelveticaOblique.New())
	bodyFont := font.Must(standard.TimesRoman.New())

	footer, err := buildFooter(footerFont, document.A4.URx, pdf.GetVersion(doc.Out))
	if err != nil {
		return err
	}

	for i := 1; i <= numPages; i++ {
		page := doc.AddPage()

		// Body content via the builder.  This becomes the page's first
		// content-stream segment.
		page.TextBegin()
		page.TextSetFont(bodyFont, 14)
		page.TextFirstLine(72, 720)
		page.TextShow(fmt.Sprintf("Page %d of %d", i, numPages))
		page.TextEnd()

		// Make footerFont available under the same name the footer's
		// bytes reference.  Without this, validation rejects the page:
		// the footer's "/FootFont 8 Tf" would refer to a missing
		// resource.
		if err := page.Builder.RegisterFont(footerFontName, footerFont); err != nil {
			return err
		}

		// Append the shared footer as a second content-stream segment.
		// The same *content.Operators pointer is appended on every page;
		// the resource manager writes the underlying stream object once
		// and references it from each page's /Contents array.
		page.AppendContent(footer)

		if err := page.Close(); err != nil {
			return err
		}
	}

	return doc.Close()
}

// buildFooter creates the shared footer content as a *content.Operators.
// The footer prints "Downloaded by Jochen Voss" centred at the bottom
// of the page in light grey.  The footer's content-stream bytes refer
// to a font under footerFontName; pages that include the footer must
// bind that name themselves.
func buildFooter(f font.Instance, pageWidth float64, version pdf.Version) (*content.Operators, error) {
	res := &content.Resources{}
	b := builder.New(content.Page, res, version)
	if err := b.RegisterFont(footerFontName, f); err != nil {
		return nil, err
	}

	ops := b.Build(func(b *builder.Builder) error {
		b.PushGraphicsState()
		b.SetFillColor(color.DeviceGray(0.4))
		b.TextBegin()
		b.TextSetFont(f, 8)
		// Centre the line by eye: 8pt Helvetica-Oblique "Downloaded
		// by Jochen Voss" is roughly 130pt wide.
		b.TextFirstLine(pageWidth/2-65, 36)
		b.TextShow("Downloaded by Jochen Voss")
		b.TextEnd()
		b.PopGraphicsState()
		return nil
	})
	if b.Err != nil {
		return nil, b.Err
	}
	return ops, nil
}
