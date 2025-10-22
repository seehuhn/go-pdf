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
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/tools/pdf-extract/text"
)

func TestActualText(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)

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

	// test without ActualText
	var buf1 bytes.Buffer
	extractor1 := text.New(w, &buf1)
	extractor1.UseActualText = false

	err = extractor1.ExtractPage(pageDict)
	if err != nil {
		t.Fatal(err)
	}

	extracted1 := normalizeWhitespace(buf1.String())
	expected1 := "normal text the original text some two-level nested text example"
	if extracted1 != expected1 {
		t.Errorf("without ActualText:\ngot:  %q\nwant: %q", extracted1, expected1)
	}

	// test with ActualText
	var buf2 bytes.Buffer
	extractor2 := text.New(w, &buf2)
	extractor2.UseActualText = true

	err = extractor2.ExtractPage(pageDict)
	if err != nil {
		t.Fatal(err)
	}

	extracted2 := normalizeWhitespace(buf2.String())
	expected2 := "normal text the replaced text some replaced example"
	if extracted2 != expected2 {
		t.Errorf("with ActualText:\ngot:  %q\nwant: %q", extracted2, expected2)
	}
}

func normalizeWhitespace(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
