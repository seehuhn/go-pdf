// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
	f := acroform.NewTextField(fieldName)
	f.DefaultAppearance = "/Helv 0 Tf 0 g"
	w := annotation.AddWidget(f, pdf.Rectangle{LLx: 100, LLy: 700, URx: 300, URy: 720})
	page.Page.AddAnnots(w)
	form := &acroform.InteractiveForm{Fields: []acroform.Node{f}}
	page.Out.GetMeta().Catalog.AcroForm = page.RM.StoreDeferred(form)
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

	acro, err := pdf.NewCursor(r).Dict(r.GetMeta().Catalog.AcroForm)
	if err != nil || acro == nil {
		t.Fatalf("no merged AcroForm: %v", err)
	}
	fields, err := pdf.NewCursor(r).Array(acro["Fields"])
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 {
		t.Fatalf("merged form has %d root fields, want 2", len(fields))
	}

	names := map[string]bool{}
	for _, el := range fields {
		fd, err := pdf.NewCursor(r).Dict(el)
		if err != nil {
			t.Fatal(err)
		}
		name, _ := pdf.NewCursor(r).TextString(fd["T"])
		names[name.String()] = true
		if subtype, _ := pdf.NewCursor(r).Name(fd["Subtype"]); subtype != "Widget" {
			t.Errorf("field %q is not a merged widget (Subtype=%q)", name, subtype)
		}
	}
	if !names["name"] || !names["name_2"] {
		t.Errorf("field names = %v, want name and name_2", names)
	}
}
