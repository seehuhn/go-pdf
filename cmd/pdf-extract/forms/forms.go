// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package forms implements the "form" query of pdf-extract, listing the data
// a document's interactive form exports.
package forms

import (
	"fmt"
	"io"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation/decode"
)

// List writes the export data of doc's interactive form to w, one name and
// value pair per line, in the form
//
//	name: value
//
// See [seehuhn.de/go/pdf/acroform.InteractiveForm.ExportData] for the names
// and values included.  Line breaks in a value are replaced by spaces.
func List(doc pdf.Getter, w io.Writer) error {
	idx, err := decode.DocumentForm(pdf.NewExtractor(doc))
	if err != nil {
		return err
	}
	if idx == nil {
		fmt.Fprintln(w, "No interactive form in document.")
		return nil
	}

	count := 0
	for name, value := range idx.Form.ExportData() {
		fmt.Fprintf(w, "%s: %s\n", name, collapse(value))
		count++
	}
	if count == 0 {
		fmt.Fprintln(w, "Interactive form exports no data.")
	}
	return nil
}

// collapse replaces line breaks with spaces so a value stays on one line.
func collapse(s string) string {
	return strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(s)
}
