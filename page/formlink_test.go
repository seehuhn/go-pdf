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

package page_test

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/decode"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	pdfpage "seehuhn.de/go/pdf/page"
)

// TestPageDecodeLinksWidgets builds a document with an interactive form whose
// widgets are reached only through page /Annots — a merged single-widget field
// and a multi-widget field with widgets on two pages — and verifies that simply
// decoding a page links every widget to its field (Widget.Parent), without the
// caller decoding the AcroForm itself. It also checks that the merged field and
// its page widget are one shared object.
func TestPageDecodeLinksWidgets(t *testing.T) {
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	mergedRef := w.Alloc() // one object that is both a field and a widget
	multiRef := w.Alloc()  // a multi-widget field
	wA := w.Alloc()        // its widget on page 1
	wB := w.Alloc()        // its widget on page 2
	page1Ref := w.Alloc()
	page2Ref := w.Alloc()
	pagesRef := w.Alloc()
	formRef := w.Alloc()

	rect := pdf.Array{pdf.Real(0), pdf.Real(0), pdf.Real(100), pdf.Real(100)}
	put := func(ref pdf.Reference, d pdf.Dict) {
		if err := w.Put(ref, d); err != nil {
			t.Fatal(err)
		}
	}

	put(mergedRef, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"),
		"FT": pdf.Name("Tx"), "T": pdf.TextString("merged"), "Rect": rect,
	})
	put(multiRef, pdf.Dict{
		"FT": pdf.Name("Tx"), "T": pdf.TextString("multi"),
		"Kids": pdf.Array{wA, wB},
	})
	put(wA, pdf.Dict{"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"), "Rect": rect, "Parent": multiRef})
	put(wB, pdf.Dict{"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"), "Rect": rect, "Parent": multiRef})
	put(formRef, pdf.Dict{"Fields": pdf.Array{mergedRef, multiRef}})
	put(page1Ref, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef, "MediaBox": rect,
		"Annots": pdf.Array{mergedRef, wA},
	})
	put(page2Ref, pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef, "MediaBox": rect,
		"Annots": pdf.Array{wB},
	})
	put(pagesRef, pdf.Dict{
		"Type": pdf.Name("Pages"), "Kids": pdf.Array{page1Ref, page2Ref}, "Count": pdf.Integer(2),
	})

	w.GetMeta().Catalog.Pages = pagesRef
	w.GetMeta().Catalog.AcroForm = formRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Data), int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	x := pdf.NewExtractor(r)

	// decode page 1 ONLY; this must trigger the form decode and link the widgets
	p1, err := pdf.ExtractorGet(x, nil, page1Ref, pdfpage.Decode)
	if err != nil {
		t.Fatal(err)
	}
	if len(p1.Annots) != 2 {
		t.Fatalf("page 1: got %d annots, want 2", len(p1.Annots))
	}
	mergedW, ok := p1.Annots[0].(*annotation.Widget)
	if !ok {
		t.Fatalf("annot 0 is %T, want *Widget", p1.Annots[0])
	}
	multiW, ok := p1.Annots[1].(*annotation.Widget)
	if !ok {
		t.Fatalf("annot 1 is %T, want *Widget", p1.Annots[1])
	}
	if mergedW.Parent == nil {
		t.Error("merged widget: Parent not linked by page decode")
	}
	if multiW.Parent == nil {
		t.Error("multi-widget A: Parent not linked by page decode")
	}

	// the merged field and its page widget must be one shared object: the field's
	// single Kid is the very widget in the page's /Annots
	form, err := pdf.ExtractorGet(x, nil, formRef, decode.Form)
	if err != nil {
		t.Fatal(err)
	}
	mergedField, ok := form.Fields[0].(acroform.Field)
	if !ok {
		t.Fatalf("form field 0 is %T, want a terminal field", form.Fields[0])
	}
	if mergedW.Parent != mergedField {
		t.Error("merged widget.Parent is not the field in /Fields")
	}
	widgets := mergedField.Widgets()
	if len(widgets) != 1 || widgets[0] != acroform.Widget(mergedW) {
		t.Errorf("merged field widgets = %v, want the shared page widget", widgets)
	}

	// the multi-widget field links both widgets; widget A is the page's object
	multiField, ok := form.Fields[1].(acroform.Field)
	if !ok {
		t.Fatalf("form field 1 is %T, want a terminal field", form.Fields[1])
	}
	if multiW.Parent != multiField {
		t.Error("multi-widget A.Parent is not the multi field")
	}
	if len(multiField.Widgets()) != 2 {
		t.Errorf("multi field has %d widgets, want 2", len(multiField.Widgets()))
	}
}
