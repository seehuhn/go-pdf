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

package text

import (
	"bytes"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/property"
)

func TestTextExtractorBasic(t *testing.T) {
	// create test PDF with simple text
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	F := standard.Helvetica.New()

	pageTree := pagetree.NewWriter(w)

	b := builder.New(content.Page, nil)
	b.TextBegin()
	b.TextFirstLine(100, 700)
	b.TextSetFont(F, 12)
	b.TextShow("Hello World")
	b.TextEnd()

	if b.Err != nil {
		t.Fatal(b.Err)
	}

	// create stream from builder
	contentRef := w.Alloc()
	stream, err := w.OpenStream(contentRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = content.Write(stream, b.Stream, pdf.V2_0, content.Page, b.Resources)
	if err != nil {
		t.Fatal(err)
	}
	err = stream.Close()
	if err != nil {
		t.Fatal(err)
	}

	// embed resources
	resRef, err := rm.Embed(b.Resources)
	if err != nil {
		t.Fatal(err)
	}

	page := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Contents":  contentRef,
		"Resources": resRef,
		"MediaBox":  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 595, URy: 842},
	}
	err = pageTree.AppendPage(page)
	if err != nil {
		t.Fatal(err)
	}

	treeRef, err := pageTree.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	w.GetMeta().Catalog.Pages = treeRef
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// extract text
	var buf bytes.Buffer
	extractor := New(w, &buf)
	extractor.UseActualText = false

	_, pageDict, err := pagetree.GetPage(w, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = extractor.ExtractPage(pageDict)
	if err != nil {
		t.Fatal(err)
	}

	extracted := buf.String()
	if !strings.Contains(extracted, "Hello World") {
		t.Errorf("extracted text %q does not contain \"Hello World\"", extracted)
	}
}

func TestTextExtractorActualText(t *testing.T) {
	// create test PDF with ActualText
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	F := standard.Helvetica.New()

	pageTree := pagetree.NewWriter(w)

	b := builder.New(content.Page, nil)

	// normal text
	b.TextBegin()
	b.TextFirstLine(100, 700)
	b.TextSetFont(F, 12)
	b.TextShow("the ")
	b.TextEnd()

	// text with ActualText
	actualText := &property.ActualText{
		Text:      "replaced",
		SingleUse: true,
	}
	mc := &graphics.MarkedContent{
		Tag:        "Span",
		Properties: actualText,
		Inline:     true,
	}
	b.MarkedContentStart(mc)
	b.TextBegin()
	b.TextSetFont(F, 12)
	b.TextShow("original")
	b.TextEnd()
	b.MarkedContentEnd()

	// more normal text
	b.TextBegin()
	b.TextSetFont(F, 12)
	b.TextShow(" text")
	b.TextEnd()

	if b.Err != nil {
		t.Fatal(b.Err)
	}

	// create stream from builder
	contentRef := w.Alloc()
	stream, err := w.OpenStream(contentRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = content.Write(stream, b.Stream, pdf.V2_0, content.Page, b.Resources)
	if err != nil {
		t.Fatal(err)
	}
	err = stream.Close()
	if err != nil {
		t.Fatal(err)
	}

	// embed resources
	resRef, err := rm.Embed(b.Resources)
	if err != nil {
		t.Fatal(err)
	}

	page := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Contents":  contentRef,
		"Resources": resRef,
		"MediaBox":  &pdf.Rectangle{LLx: 0, LLy: 0, URx: 595, URy: 842},
	}
	err = pageTree.AppendPage(page)
	if err != nil {
		t.Fatal(err)
	}

	treeRef, err := pageTree.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatal(err)
	}

	w.GetMeta().Catalog.Pages = treeRef
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// extract without ActualText
	var buf1 bytes.Buffer
	extractor1 := New(w, &buf1)
	extractor1.UseActualText = false

	_, pageDict, err := pagetree.GetPage(w, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = extractor1.ExtractPage(pageDict)
	if err != nil {
		t.Fatal(err)
	}

	extracted1 := strings.TrimSpace(buf1.String())
	if !strings.Contains(extracted1, "the original text") {
		t.Errorf("without ActualText: got %q, want to contain \"the original text\"", extracted1)
	}

	// extract with ActualText
	var buf2 bytes.Buffer
	extractor2 := New(w, &buf2)
	extractor2.UseActualText = true

	err = extractor2.ExtractPage(pageDict)
	if err != nil {
		t.Fatal(err)
	}

	extracted2 := strings.TrimSpace(buf2.String())
	if !strings.Contains(extracted2, "the replaced text") {
		t.Errorf("with ActualText: got %q, want to contain \"the replaced text\"", extracted2)
	}
	if strings.Contains(extracted2, "original") {
		t.Errorf("with ActualText: got %q, should not contain \"original\"", extracted2)
	}
}
