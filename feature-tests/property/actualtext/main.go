// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"bytes"
	"io"
	"log"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/property"
)

func main() {
	err := generateTestPDF("test.pdf")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Generated test.pdf")
}

func generateTestPDF(filename string) error {
	fd, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer fd.Close()

	w, err := pdf.NewWriter(fd, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	err = writeTestPage(w)
	if err != nil {
		return err
	}

	return w.Close()
}

// writeTestPage creates a page with ActualText test cases.
func writeTestPage(w *pdf.Writer) error {
	rm := pdf.NewResourceManager(w)
	F := standard.Helvetica.New()

	pageTree := pagetree.NewWriter(w)
	contentBuf := &bytes.Buffer{}
	content := graphics.NewWriter(contentBuf, rm)

	writeTestContent(content, F)

	// create content stream
	contentRef := w.Alloc()
	stream, err := w.OpenStream(contentRef, nil)
	if err != nil {
		return err
	}
	_, err = io.Copy(stream, contentBuf)
	if err != nil {
		return err
	}
	err = stream.Close()
	if err != nil {
		return err
	}

	// create page
	page := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Contents":  contentRef,
		"Resources": pdf.AsDict(content.Resources),
		"MediaBox":  &pdf.Rectangle{0, 0, 595, 842},
	}
	err = pageTree.AppendPage(page)
	if err != nil {
		return err
	}

	treeRef, err := pageTree.Close()
	if err != nil {
		return err
	}

	err = rm.Close()
	if err != nil {
		return err
	}

	w.GetMeta().Catalog.Pages = treeRef
	return nil
}

// writeTestContent writes three test cases to the content stream:
//  1. Normal text without ActualText
//  2. Simple ActualText replacement
//  3. Nested ActualText (inner should be suppressed)
func writeTestContent(content *graphics.Writer, F font.Layouter) {
	y := 800.0

	// 1. normal text
	content.TextBegin()
	content.TextFirstLine(100, y)
	content.TextSetFont(F, 12)
	content.TextShow("normal text")
	content.TextEnd()
	y -= 30

	// 2. simple ActualText: "the original text" -> "the replaced text"
	content.TextBegin()
	content.TextFirstLine(100, y)
	content.TextSetFont(F, 12)
	content.TextShow("the ")

	content.MarkedContentStart(&graphics.MarkedContent{
		Tag: "Span",
		Properties: &property.ActualText{
			Text:      "replaced",
			SingleUse: true,
		},
		Inline: true,
	})
	content.TextShow("original")
	content.MarkedContentEnd()

	content.TextShow(" text")
	content.TextEnd()
	y -= 30

	// 3. nested ActualText: outer wins, inner suppressed
	// "some two-level nested text example" -> "some replaced example"
	content.TextBegin()
	content.TextFirstLine(100, y)
	content.TextSetFont(F, 12)
	content.TextShow("some ")

	// outer ActualText
	content.MarkedContentStart(&graphics.MarkedContent{
		Tag: "Span",
		Properties: &property.ActualText{
			Text:      "replaced",
			SingleUse: true,
		},
		Inline: true,
	})
	content.TextShow("two-level ")

	// inner ActualText (suppressed by outer)
	content.MarkedContentStart(&graphics.MarkedContent{
		Tag: "Span",
		Properties: &property.ActualText{
			Text:      "inner",
			SingleUse: true,
		},
		Inline: true,
	})
	content.TextShow("nested")
	content.MarkedContentEnd()

	content.TextShow(" text")
	content.MarkedContentEnd()

	content.TextShow(" example")
	content.TextEnd()
}
