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
