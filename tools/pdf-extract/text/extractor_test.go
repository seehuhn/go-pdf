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
	"io"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
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
	contentBuf := &bytes.Buffer{}
	content := graphics.NewWriter(contentBuf, rm)
	content.TextBegin()
	content.TextFirstLine(100, 700)
	content.TextSetFont(F, 12)
	content.TextShow("Hello World")
	content.TextEnd()

	// create stream from buffer
	contentRef := w.Alloc()
	stream, err := w.OpenStream(contentRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(stream, contentBuf)
	if err != nil {
		t.Fatal(err)
	}
	err = stream.Close()
	if err != nil {
		t.Fatal(err)
	}

	page := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Contents":  contentRef,
		"Resources": pdf.AsDict(content.Resources),
		"MediaBox":  &pdf.Rectangle{0, 0, 595, 842},
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
	contentBuf := &bytes.Buffer{}
	content := graphics.NewWriter(contentBuf, rm)

	// normal text
	content.TextBegin()
	content.TextFirstLine(100, 700)
	content.TextSetFont(F, 12)
	content.TextShow("the ")
	content.TextEnd()

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
	content.MarkedContentStart(mc)
	content.TextBegin()
	content.TextSetFont(F, 12)
	content.TextShow("original")
	content.TextEnd()
	content.MarkedContentEnd()

	// more normal text
	content.TextBegin()
	content.TextSetFont(F, 12)
	content.TextShow(" text")
	content.TextEnd()

	// create stream from buffer
	contentRef := w.Alloc()
	stream, err := w.OpenStream(contentRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(stream, contentBuf)
	if err != nil {
		t.Fatal(err)
	}
	err = stream.Close()
	if err != nil {
		t.Fatal(err)
	}

	page := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Contents":  contentRef,
		"Resources": pdf.AsDict(content.Resources),
		"MediaBox":  &pdf.Rectangle{0, 0, 595, 842},
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
