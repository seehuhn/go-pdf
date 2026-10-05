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
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

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

	c := pdf.NewCursor(w)
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
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

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

	c := pdf.NewCursor(w)
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
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

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

	c := pdf.NewCursor(w)
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

// TestAnnotationReadsFormFirst checks that decoding a widget on its own reads
// the interactive form first, so that the form's repairs apply to the widget's
// field. Two merged fields share a fully qualified name with different values;
// after the repair both carry the first value and the form can be written.
func TestAnnotationReadsFormFirst(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)

	var refs pdf.Array
	for _, v := range []string{"first", "second"} {
		ref := w.Alloc()
		w.Put(ref, pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"),
			"Rect": pageAnnotRect,
			"FT":   pdf.Name("Tx"), "T": pdf.TextString("a"),
			"V": pdf.TextString(v), "DA": pdf.String("/Helv 0 Tf"),
		})
		refs = append(refs, ref)
	}
	formRef := w.Alloc()
	w.Put(formRef, pdf.Dict{"Fields": refs})
	w.GetMeta().Catalog.AcroForm = formRef

	x := pdf.NewExtractor(w)
	for _, ref := range refs {
		a, err := pdf.Decode(pdf.CursorAt(x, nil), ref, Annotation)
		if err != nil {
			t.Fatal(err)
		}
		wa, ok := a.(*annotation.Widget)
		if !ok {
			t.Fatalf("annotation is %T, want *annotation.Widget", a)
		}
		f, ok := wa.Field.(*acroform.TextField)
		if !ok {
			t.Fatalf("widget field is %T, want *acroform.TextField", wa.Field)
		}
		if f.V == nil || f.V.Value != "first" {
			t.Errorf("field value = %v, want %q", f.V, "first")
		}
	}

	form, err := pdf.DecodeExclusive(pdf.CursorAt(x, nil), formRef, Form)
	if err != nil {
		t.Fatal(err)
	}
	roundTripForm(t, pdf.V1_7, form)
}

// A widget dictionary carrying field entries but no partial name is not a
// field, but it is still an annotation and gets the appearance repairs.
func TestDecodeUnnamedMergedWidgetAppearance(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"),
		"Rect": pageAnnotRect,
		"FT":   pdf.Name("Tx"), "DA": pdf.String("/Helv 0 Tf"),
	}); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	a, err := pdf.Decode(pdf.CursorAt(x, nil), ref, Annotation)
	if err != nil {
		t.Fatal(err)
	}
	wa, ok := a.(*annotation.Widget)
	if !ok {
		t.Fatalf("annotation is %T, want *annotation.Widget", a)
	}
	if wa.Field != nil {
		t.Errorf("unnamed widget has field %T, want nil", wa.Field)
	}
	if wa.Appearance == nil {
		t.Error("missing appearance was not repaired")
	}
}

// A widget given as a direct dictionary is decoded as itself, not as the
// object enclosing it.
func TestAnnotationDirectWidget(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	widget := pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"),
		"Rect": pageAnnotRect,
		"FT":   pdf.Name("Tx"), "T": pdf.TextString("a"),
		"DA": pdf.String("/Helv 0 Tf"),
	}
	pageRef := w.Alloc()
	if err := w.Put(pageRef, pdf.Dict{
		"Type": pdf.Name("Page"), "Annots": pdf.Array{widget},
	}); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	first := func(c pdf.Cursor, obj pdf.Object, _ bool) (annotation.Annotation, error) {
		dict, err := c.Dict(obj)
		if err != nil {
			return nil, err
		}
		annots, err := c.Array(dict["Annots"])
		if err != nil {
			return nil, err
		}
		return pdf.Decode(c, annots[0], Annotation)
	}
	a, err := pdf.Decode(pdf.CursorAt(x, nil), pageRef, first)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.(*annotation.Widget); !ok {
		t.Fatalf("annotation is %T, want *annotation.Widget", a)
	}
}
