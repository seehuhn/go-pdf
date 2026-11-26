// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package decode

import (
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestFieldAt(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rect := pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)}
	widget := func(parent pdf.Reference) pdf.Dict {
		return pdf.Dict{"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"), "Rect": rect, "Parent": parent}
	}
	put := func(ref pdf.Reference, obj pdf.Object) {
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
	}

	// a field of the form
	listed, lw := w.Alloc(), w.Alloc()
	put(listed, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("listed"), "Kids": pdf.Array{lw}})
	put(lw, widget(listed))

	// a field below a group which /Fields does not list; the field inherits
	// its type and value, and shares a widget with the listed field
	g, f, w1, w2 := w.Alloc(), w.Alloc(), w.Alloc(), w.Alloc()
	put(g, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("g"), "V": pdf.String("inherited"), "Kids": pdf.Array{f}})
	put(f, pdf.Dict{"T": pdf.String("f"), "Parent": g, "Kids": pdf.Array{w1, w2, lw}})
	put(w1, widget(f))
	put(w2, widget(f))

	// an unlisted field merged with its widget
	m := w.Alloc()
	mDict := widget(0)
	delete(mDict, "Parent")
	mDict["FT"] = pdf.Name("Btn")
	mDict["T"] = pdf.String("m")
	put(m, mDict)

	// an unlisted field whose widget lies below a kid without /T
	h, u, w3 := w.Alloc(), w.Alloc(), w.Alloc()
	put(h, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("h"), "Kids": pdf.Array{u}})
	put(u, pdf.Dict{"Parent": h, "Kids": pdf.Array{w3}})
	put(w3, widget(u))

	// a widget which names a field as its parent but is not among its kids,
	// and a widget without a parent
	stray, loose := w.Alloc(), w.Alloc()
	put(stray, widget(h))
	looseDict := widget(0)
	delete(looseDict, "Parent")
	put(loose, looseDict)

	formRef := w.Alloc()
	put(formRef, pdf.Dict{"Fields": pdf.Array{listed}})
	w.GetMeta().Catalog.AcroForm = formRef

	x := pdf.NewExtractor(w)
	idx, err := DocumentForm(x)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("listed", func(t *testing.T) {
		got, widgets, err := FieldAt(x, listed)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil || idx.FieldRefs[got] != listed {
			t.Fatalf("got %v, want the field of the form", got)
		}
		if len(widgets) != 1 || widgets[got.GetCommon().Widgets[0]] != lw {
			t.Errorf("got widgets %v, want one at %v", widgets, lw)
		}
	})

	t.Run("unlisted", func(t *testing.T) {
		got, widgets, err := FieldAt(x, f)
		if err != nil {
			t.Fatal(err)
		}
		tf, ok := got.(*acroform.TextField)
		if !ok {
			t.Fatalf("got %T, want *acroform.TextField", got)
		}
		if tf.Text == nil || tf.Text.Value.String() != "inherited" {
			t.Errorf("got value %v, want %q", tf.Text, "inherited")
		}
		var refs []pdf.Reference
		for _, w := range tf.Widgets {
			if w.ParentField() != got {
				t.Error("widget is not linked to the field")
			}
			refs = append(refs, widgets[w])
		}
		if len(refs) != 2 || refs[0] != w1 || refs[1] != w2 {
			t.Errorf("got widget references %v, want [%v %v]", refs, w1, w2)
		}

		again, _, err := FieldAt(x, f)
		if err != nil {
			t.Fatal(err)
		}
		if again != got {
			t.Error("second call returned a different field")
		}
	})

	t.Run("merged", func(t *testing.T) {
		got, widgets, err := FieldAt(x, m)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := got.(*acroform.ButtonField); !ok {
			t.Fatalf("got %T, want *acroform.ButtonField", got)
		}
		if len(widgets) != 1 || widgets[got.GetCommon().Widgets[0]] != m {
			t.Errorf("got widgets %v, want the merged dictionary", widgets)
		}
	})

	t.Run("widget below a kid without a name", func(t *testing.T) {
		got, widgets, err := FieldAt(x, h)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil || len(widgets) != 1 || widgets[got.GetCommon().Widgets[0]] != w3 {
			t.Errorf("got %v with widgets %v, want a field with the widget at %v", got, widgets, w3)
		}
	})

	t.Run("widgets and unnamed kids", func(t *testing.T) {
		for _, c := range []struct{ ref, owner pdf.Reference }{{w1, f}, {w2, f}, {u, h}, {w3, h}} {
			want, _, err := FieldAt(x, c.owner)
			if err != nil {
				t.Fatal(err)
			}
			got, widgets, err := FieldAt(x, c.ref)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || got != want {
				t.Errorf("FieldAt(%v) = %v, want the field at %v", c.ref, got, c.owner)
			}
			if len(widgets) != len(got.GetCommon().Widgets) {
				t.Errorf("FieldAt(%v) returned %d widgets, want %d", c.ref, len(widgets), len(got.GetCommon().Widgets))
			}
		}
	})

	t.Run("not part of a terminal field", func(t *testing.T) {
		for _, ref := range []pdf.Reference{g, stray, loose} {
			got, _, err := FieldAt(x, ref)
			if err != nil {
				t.Fatal(err)
			}
			if got != nil {
				t.Errorf("FieldAt(%v) = %T, want nil", ref, got)
			}
		}
	})
}

// A document without a form has no field of a form, but its fields can be
// read on their own.
func TestFieldAtNoForm(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	f, kid := w.Alloc(), w.Alloc()
	if err := w.Put(f, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("f"), "Kids": pdf.Array{kid}}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(kid, pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"), "Parent": f,
		"Rect": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)},
	}); err != nil {
		t.Fatal(err)
	}

	got, widgets, err := FieldAt(pdf.NewExtractor(w), f)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got.GetCommon().Widgets) != 1 || widgets[got.GetCommon().Widgets[0]] != kid {
		t.Errorf("got %v with widgets %v, want a field with the widget at %v", got, widgets, kid)
	}
}

// A kid without a partial name which several fields outside the form list
// gives its widgets to the field its /Parent entry names, and to no field if
// it has no such entry.
func TestFieldAtSharedKid(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rect := pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)}
	widget := func(parent pdf.Reference) pdf.Dict {
		d := pdf.Dict{"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"), "Rect": rect}
		if parent != 0 {
			d["Parent"] = parent
		}
		return d
	}
	field := func(name string, kids ...pdf.Object) pdf.Dict {
		return pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String(name), "Kids": pdf.Array(kids)}
	}
	put := func(ref pdf.Reference, obj pdf.Object) {
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
	}

	// a shared kid which names the second field as its parent
	s1, s2, s3, v, w1, w2 := w.Alloc(), w.Alloc(), w.Alloc(), w.Alloc(), w.Alloc(), w.Alloc()
	put(s1, field("s1", v))
	put(s2, field("s2", v))
	put(s3, field("s3", v))
	put(v, pdf.Dict{"Parent": s2, "Kids": pdf.Array{w1, w2}})
	put(w1, widget(v))
	put(w2, widget(v))

	// a kid without /T and a widget, neither of which names a parent
	n, vn, wv, wn := w.Alloc(), w.Alloc(), w.Alloc(), w.Alloc()
	put(n, field("n", vn, wn))
	put(vn, pdf.Dict{"Kids": pdf.Array{wv}})
	put(wv, widget(vn))
	put(wn, widget(0))

	x := pdf.NewExtractor(w)
	for _, c := range []struct {
		ref  pdf.Reference
		want []pdf.Reference
	}{
		{s1, nil},
		{s2, []pdf.Reference{w1, w2}},
		{s3, nil},
		{n, []pdf.Reference{wn}},
	} {
		f, widgets, err := FieldAt(x, c.ref)
		if err != nil {
			t.Fatal(err)
		}
		if f == nil {
			t.Errorf("FieldAt(%v) returned no field", c.ref)
			continue
		}
		var got []pdf.Reference
		for _, fw := range f.GetCommon().Widgets {
			got = append(got, widgets[fw])
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("FieldAt(%v) has widgets %v, want %v", c.ref, got, c.want)
		}
	}
}

// A widget of a field of the form leads to this field, also where its /Parent
// entry is missing or names a dictionary which does not list the widget.
func TestFieldAtFormWidget(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rect := pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)}
	widget := func(parent pdf.Reference) pdf.Dict {
		d := pdf.Dict{"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"), "Rect": rect}
		if parent != 0 {
			d["Parent"] = parent
		}
		return d
	}
	put := func(ref pdf.Reference, obj pdf.Object) {
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
	}

	field, other := w.Alloc(), w.Alloc()
	good, bare, astray := w.Alloc(), w.Alloc(), w.Alloc()
	put(field, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("a"), "Kids": pdf.Array{good, bare, astray}})
	put(other, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.String("b")})
	put(good, widget(field))
	put(bare, widget(0))
	put(astray, widget(other))

	formRef := w.Alloc()
	put(formRef, pdf.Dict{"Fields": pdf.Array{field, other}})
	w.GetMeta().Catalog.AcroForm = formRef

	x := pdf.NewExtractor(w)
	want, _, err := FieldAt(x, field)
	if err != nil {
		t.Fatal(err)
	}
	if want == nil || len(want.GetCommon().Widgets) != 3 {
		t.Fatalf("form field %v does not have three widgets", want)
	}

	for _, ref := range []pdf.Reference{good, bare, astray} {
		got, widgets, err := FieldAt(x, ref)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("FieldAt(%v) returned %v, want the field which lists the widget", ref, got)
			continue
		}
		var refs []pdf.Reference
		for _, fw := range got.GetCommon().Widgets {
			refs = append(refs, widgets[fw])
		}
		if !slices.Equal(refs, []pdf.Reference{good, bare, astray}) {
			t.Errorf("FieldAt(%v) has widgets %v", ref, refs)
		}
	}
}

// TestFieldAtConcurrent reads fields with FieldAt while the form is being
// read, from a file whose /Parent and /Kids entries form cycles.  Every
// caller for one reference must get the same field, and the reads must not
// hang.  Run with -race.
func TestFieldAtConcurrent(t *testing.T) {
	objs := map[string]pdf.Dict{
		"a":  {"FT": pdf.Name("Tx"), "T": pdf.String("a"), "Kids": pdf.Array{pdf.String("@b")}, "Parent": pdf.String("@b")},
		"b":  {"T": pdf.String("b"), "Kids": pdf.Array{pdf.String("@a"), pdf.String("@wb")}, "Parent": pdf.String("@a")},
		"wb": rawWidget(pdf.Dict{"Parent": pdf.String("@b")}),
		"f": {
			"T": pdf.String("f"), "V": pdf.String("x"), "Parent": pdf.String("@g"),
			"Kids": pdf.Array{pdf.String("@w1"), pdf.String("@w2"), pdf.String("@wb")},
		},
		"g":  {"FT": pdf.Name("Tx"), "T": pdf.String("g"), "Parent": pdf.String("@f"), "Kids": pdf.Array{pdf.String("@f")}},
		"w1": rawWidget(pdf.Dict{"Parent": pdf.String("@f")}),
		"w2": rawWidget(pdf.Dict{"Parent": pdf.String("@f"), "Popup": pdf.String("@w1")}),
		"m":  rawWidget(pdf.Dict{"FT": pdf.Name("Btn"), "T": pdf.String("m"), "Parent": pdf.String("@m")}),
	}
	order := []string{"a", "b", "wb", "f", "g", "w1", "w2", "m"}
	r, refs := writeRawForm(t, objs, order, func(refs map[string]pdf.Reference) pdf.Dict {
		return pdf.Dict{"Fields": pdf.Array{refs["a"]}}
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 50 {
			x := pdf.NewExtractor(r)
			got := make([][]acroform.Field, len(order))
			var wg sync.WaitGroup
			wg.Go(func() {
				if _, err := DocumentForm(x); err != nil && !pdf.IsMalformed(err) {
					t.Error(err)
				}
			})
			for j, name := range order {
				got[j] = make([]acroform.Field, 2)
				for k := range got[j] {
					wg.Go(func() {
						// stagger the start, so that some runs meet the walk midway
						for range (i + j + k) * 10 {
							runtime.Gosched()
						}
						f, _, err := FieldAt(x, refs[name])
						if err != nil {
							t.Error(err)
						}
						got[j][k] = f
					})
				}
				wg.Go(func() {
					if _, err := pdf.Optional(pdf.CursorAt(x, nil).Decode(refs[name], Annotation)); err != nil {
						t.Error(err)
					}
				})
			}
			wg.Wait()
			for j, fs := range got {
				if fs[0] != fs[1] {
					t.Errorf("FieldAt(%s) returned two different fields", order[j])
				}
				if name := order[j]; (name == "f" || name == "m") && fs[0] == nil {
					t.Errorf("FieldAt(%s) found no field", name)
				}
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Minute):
		t.Fatal("concurrent reads did not finish")
	}
}
