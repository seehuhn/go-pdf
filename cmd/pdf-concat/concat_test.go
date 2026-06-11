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

package main

import (
	"path/filepath"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/document"
)

// makeFormPDF writes a one-page PDF with a single text field named fieldName,
// merged with its single widget on the page.
func makeFormPDF(t *testing.T, path, fieldName string) {
	t.Helper()
	page, err := document.CreateSinglePage(path, document.A4, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := &annotation.Widget{
		Common: annotation.Common{Rect: pdf.Rectangle{LLx: 100, LLy: 700, URx: 300, URy: 720}},
	}
	f := &acroform.FieldTx{
		FieldCommon: acroform.FieldCommon{T: fieldName, Kids: []acroform.Node{w}},
	}
	page.Page.Annots = append(page.Page.Annots, w)
	form := &acroform.InteractiveForm{Fields: []acroform.Field{f}}
	formRef, err := page.RM.Store(form)
	if err != nil {
		t.Fatal(err)
	}
	page.Out.GetMeta().Catalog.AcroForm = formRef
	if err := page.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestConcatMergesForms concatenates two documents whose forms both have a field
// named "name" and checks that the result has both fields, with the collision
// renamed, and that each field is still a merged widget reachable from a page.
func TestConcatMergesForms(t *testing.T) {
	dir := t.TempDir()
	in1 := filepath.Join(dir, "a.pdf")
	in2 := filepath.Join(dir, "b.pdf")
	out := filepath.Join(dir, "out.pdf")
	makeFormPDF(t, in1, "name")
	makeFormPDF(t, in2, "name")

	c, err := NewConcat(out, pdf.V1_7)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Append(in1); err != nil {
		t.Fatal(err)
	}
	if err := c.Append(in2); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.Open(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	acro, err := pdf.GetDict(r, r.GetMeta().Catalog.AcroForm)
	if err != nil || acro == nil {
		t.Fatalf("no merged AcroForm: %v", err)
	}
	fields, err := pdf.GetArray(r, acro["Fields"])
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 {
		t.Fatalf("merged form has %d root fields, want 2", len(fields))
	}

	names := map[string]bool{}
	for _, el := range fields {
		fd, err := pdf.GetDict(r, el)
		if err != nil {
			t.Fatal(err)
		}
		name, _ := pdf.GetTextString(r, fd["T"])
		names[string(name)] = true
		if subtype, _ := pdf.GetName(r, fd["Subtype"]); subtype != "Widget" {
			t.Errorf("field %q is not a merged widget (Subtype=%q)", name, subtype)
		}
	}
	if !names["name"] || !names["name_2"] {
		t.Errorf("field names = %v, want name and name_2", names)
	}
}

func TestRewriteDA(t *testing.T) {
	rename := map[pdf.Name]pdf.Name{"Helv": "Helv_2"}
	cases := []struct {
		in, want string
	}{
		{"/Helv 12 Tf 0 g", "/Helv_2 12 Tf 0 g"},           // basic substitution
		{"/Helvetica 12 Tf", "/Helvetica 12 Tf"},           // prefix must not match
		{"/Cour 12 Tf", "/Cour 12 Tf"},                     // unrelated font untouched
		{"/He#6cv 12 Tf 0 g", "/He#6cv_2 12 Tf 0 g"},       // escaped name matches
		{"0 0 1 rg /Helv 10 Tf", "0 0 1 rg /Helv_2 10 Tf"}, // name after color ops
	}
	for _, tc := range cases {
		got := string(rewriteDA(pdf.String(tc.in), rename))
		if got != tc.want {
			t.Errorf("rewriteDA(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRewriteDANoRename(t *testing.T) {
	da := pdf.String("/Helv 12 Tf 0 g")
	if got := rewriteDA(da, nil); string(got) != string(da) {
		t.Errorf("rewriteDA with no renames = %q, want unchanged", got)
	}
}
