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

// Package forms implements the "form" query of pdf-extract, listing the
// terminal fields of a document's interactive form and their values.
package forms

import (
	"fmt"
	"io"
	"strings"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation/decode"
)

// List writes every terminal field of doc's interactive form to w, one field
// per line, in depth-first tree order. Each line has the form
//
//	name ["alt"] [export]: value
//
// where the alternate name (/TU) and export name (/TM) are shown only when set.
func List(doc pdf.Getter, w io.Writer) error {
	acroFormObj := doc.GetMeta().Catalog.AcroForm
	if acroFormObj == nil {
		fmt.Fprintln(w, "No interactive form in document.")
		return nil
	}

	x := pdf.NewExtractor(doc)
	form, err := pdf.Decode(pdf.CursorAt(x, nil), acroFormObj, decode.Form)
	if err != nil {
		return err
	}
	if form == nil {
		fmt.Fprintln(w, "No interactive form in document.")
		return nil
	}

	count := 0
	for name, field := range form.AllFields() {
		fmt.Fprintln(w, render(name, field))
		count++
	}
	if count == 0 {
		fmt.Fprintln(w, "Interactive form contains no fields.")
	}
	return nil
}

// render formats a single terminal field as a display line.
func render(name string, field acroform.Field) string {
	var b strings.Builder
	b.WriteString(name)
	if c := field.GetCommon(); c != nil {
		if c.AltName != "" {
			fmt.Fprintf(&b, " %q", c.AltName)
		}
		if c.ExportName != "" {
			fmt.Fprintf(&b, " [%s]", c.ExportName)
		}
	}
	b.WriteString(": ")
	b.WriteString(collapse(fieldValue(field)))
	return b.String()
}

// fieldValue returns the displayable value of a terminal field.
func fieldValue(field acroform.Field) string {
	switch f := field.(type) {
	case *acroform.TextField:
		if f.V != nil {
			return f.V.Value
		}
		return ""
	case *acroform.ChoiceField:
		return strings.Join(f.V, ", ")
	case *acroform.ButtonField:
		if f.Variant() == acroform.ButtonPush {
			return ""
		}
		if f.V == "" {
			return "Off"
		}
		return string(f.V)
	case *acroform.SignatureField:
		if f.V != nil {
			return "<signed>"
		}
		return "<unsigned>"
	default:
		return ""
	}
}

// collapse replaces line breaks with spaces so a value stays on one line.
func collapse(s string) string {
	return strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(s)
}
