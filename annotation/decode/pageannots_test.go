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

package decode

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// pageAnnotRect is the rectangle shared by the annotations in these tests.
var pageAnnotRect = pdf.Array{
	pdf.Integer(10), pdf.Integer(10), pdf.Integer(50), pdf.Integer(50),
}

// TestPageAnnotationsIRTRepair checks the page-scoped IRT repair: a reply
// whose target is an annotation on the same page keeps its InReplyTo entry,
// while one whose target is not (table 172 requires both on the same page)
// has the entry cleared and reads as an ordinary annotation.
func TestPageAnnotationsIRTRepair(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	parent := w.Alloc()
	w.Put(parent, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Text"),
		"Rect": pageAnnotRect,
	})
	reply := w.Alloc()
	w.Put(reply, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Text"),
		"Rect": pageAnnotRect, "IRT": parent,
	})
	// the target exists as an object but is not in the /Annots array
	stray := w.Alloc()
	w.Put(stray, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Text"),
		"Rect": pageAnnotRect,
	})
	dangling := w.Alloc()
	w.Put(dangling, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Text"),
		"Rect": pageAnnotRect, "IRT": stray,
	})

	c := pdf.CursorAt(pdf.NewExtractor(w), nil)
	refs, annots, err := PageAnnotations(c, pdf.Array{parent, reply, dangling})
	if err != nil {
		t.Fatal(err)
	}
	if len(annots) != 3 || len(refs) != 3 {
		t.Fatalf("got %d annotations and %d refs, want 3 and 3", len(annots), len(refs))
	}

	if got := annots[1].(*annotation.Text).InReplyTo; got != parent {
		t.Errorf("on-page reply: InReplyTo = %v, want %v", got, parent)
	}
	if got := annots[2].(*annotation.Text).InReplyTo; got != 0 {
		t.Errorf("dangling reply: InReplyTo = %v, want 0", got)
	}
}

// TestPageAnnotationsSkip checks that array entries which are not indirect
// references, and entries which do not decode to an annotation, are skipped,
// and that the returned slices stay aligned.
func TestPageAnnotationsSkip(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	a := w.Alloc()
	w.Put(a, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Text"),
		"Rect": pageAnnotRect,
	})
	// an annotation written directly into the array; the spec requires
	// indirect references, and the page decoder has always skipped these
	direct := pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Text"),
		"Rect": pageAnnotRect,
	}
	// a reference to something that is no annotation at all
	bogus := w.Alloc()
	w.Put(bogus, pdf.Integer(7))
	b := w.Alloc()
	w.Put(b, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Square"),
		"Rect": pageAnnotRect,
	})

	c := pdf.CursorAt(pdf.NewExtractor(w), nil)
	refs, annots, err := PageAnnotations(c, pdf.Array{a, direct, bogus, b})
	if err != nil {
		t.Fatal(err)
	}
	if len(annots) != 2 {
		t.Fatalf("got %d annotations, want 2", len(annots))
	}
	if refs[0] != a || refs[1] != b {
		t.Errorf("refs = %v, want [%v %v]", refs, a, b)
	}
	if _, ok := annots[0].(*annotation.Text); !ok {
		t.Errorf("annots[0] is %T, want *annotation.Text", annots[0])
	}
	if _, ok := annots[1].(*annotation.Square); !ok {
		t.Errorf("annots[1] is %T, want *annotation.Square", annots[1])
	}
}

// TestPageAnnotationsLinksWidgets checks that reading a page's annotations
// links each widget to its form field, in the layout where the field and the
// widget are separate objects.  The value of such a field is stored in the
// field dictionary, so a consumer holding only the widget would not see it.
func TestPageAnnotationsLinksWidgets(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	fieldRef := w.Alloc()
	widgetRef := w.Alloc()
	w.Put(fieldRef, pdf.Dict{
		"FT": pdf.Name("Tx"), "T": pdf.TextString("split"),
		"V":    pdf.TextString("hello"),
		"Kids": pdf.Array{widgetRef},
	})
	w.Put(widgetRef, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"),
		"Rect": pageAnnotRect, "Parent": fieldRef,
	})
	formRef := w.Alloc()
	w.Put(formRef, pdf.Dict{"Fields": pdf.Array{fieldRef}})
	w.GetMeta().Catalog.AcroForm = formRef

	c := pdf.CursorAt(pdf.NewExtractor(w), nil)
	_, annots, err := PageAnnotations(c, pdf.Array{widgetRef})
	if err != nil {
		t.Fatal(err)
	}
	if len(annots) != 1 {
		t.Fatalf("got %d annotations, want 1", len(annots))
	}
	wa, ok := annots[0].(*annotation.Widget)
	if !ok {
		t.Fatalf("annots[0] is %T, want *annotation.Widget", annots[0])
	}
	f, ok := wa.Field.(*acroform.TextField)
	if !ok {
		t.Fatalf("widget field is %T, want *acroform.TextField", wa.Field)
	}
	if f.V == nil || f.V.Value != "hello" {
		t.Errorf("field value = %v, want %q", f.V, "hello")
	}
}
