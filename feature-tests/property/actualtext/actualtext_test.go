// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/cmd/pdf-extract/text"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/pagetree"
)

func TestActualText(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	err := writeTestPage(w)
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// get the page for extraction
	_, pageDict, err := pagetree.GetPage(w, 0)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	extractor := text.New(w, &buf)

	err = extractor.ExtractPage(pageDict)
	if err != nil {
		t.Fatal(err)
	}

	extracted := normalizeWhitespace(buf.String())
	expected := "normal text the replaced text some replaced example"
	if extracted != expected {
		t.Errorf("got:  %q\nwant: %q", extracted, expected)
	}
}

func normalizeWhitespace(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
