// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package acroform

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func logicalTestForm() (form *InteractiveForm, r1, r2 *ButtonField, txt, cb Field) {
	r1 = NewButtonField("r", ButtonRadio)
	txt = NewTextField("t")
	cb = NewButtonField("r", ButtonCheckbox)
	r2 = NewButtonField("r", ButtonRadio)
	form = &InteractiveForm{
		Fields: []Node{r1, txt, &Group{Name: "g", Children: []Node{cb}}, r2},
	}
	return form, r1, r2, txt, cb
}

func TestLogicalFields(t *testing.T) {
	form, r1, r2, txt, cb := logicalTestForm()

	got := slices.Collect(form.LogicalFields())
	want := []*LogicalField{
		{Name: "r", Fields: []Field{r1, r2}},
		{Name: "t", Fields: []Field{txt}},
		{Name: "g.r", Fields: []Field{cb}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("logical fields differ (-want +got):\n%s", diff)
	}

	n := 0
	for range form.LogicalFields() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("early break: got %d iterations, want 1", n)
	}
}

func TestLogicalFieldLookup(t *testing.T) {
	form, r1, r2, _, cb := logicalTestForm()

	want := NewLogicalField("r", r1, r2)
	if diff := cmp.Diff(want, form.LogicalField("r")); diff != "" {
		t.Errorf("lookup of r differs (-want +got):\n%s", diff)
	}
	want = NewLogicalField("g.r", cb)
	if diff := cmp.Diff(want, form.LogicalField("g.r")); diff != "" {
		t.Errorf("lookup of g.r differs (-want +got):\n%s", diff)
	}
	for _, name := range []string{"x", "g"} {
		if got := form.LogicalField(name); got != nil {
			t.Errorf("lookup of %q: got %v, want nil", name, got)
		}
	}
}

// fieldSnapshot holds copies of field structs and the appearance state of
// every widget, for checking that a failed setter changed nothing.
type fieldSnapshot struct {
	Fields []Field
	States [][]pdf.Name
}

func snapshot(fields ...Field) fieldSnapshot {
	var res fieldSnapshot
	for _, f := range fields {
		switch f := f.(type) {
		case *TextField:
			c := *f
			res.Fields = append(res.Fields, &c)
		case *ChoiceField:
			c := *f
			res.Fields = append(res.Fields, &c)
		case *ButtonField:
			c := *f
			res.Fields = append(res.Fields, &c)
		case *PushButtonField:
			c := *f
			res.Fields = append(res.Fields, &c)
		}
		var st []pdf.Name
		for _, w := range f.GetCommon().Widgets {
			st = append(st, w.CurrentAppearanceState())
		}
		res.States = append(res.States, st)
	}
	return res
}

func checkUnchanged(t *testing.T, before fieldSnapshot, fields ...Field) {
	t.Helper()
	// the copies share their widgets with the fields, which cmp must
	// look into
	opt := cmp.AllowUnexported(fakeWidget{})
	if diff := cmp.Diff(before, snapshot(fields...), opt); diff != "" {
		t.Errorf("failed call changed fields (-before +after):\n%s", diff)
	}
}

func optionsField(flags FieldFlags, opts ...string) *ChoiceField {
	f := NewChoiceField("c")
	f.Flags = flags
	for _, o := range opts {
		f.Options = append(f.Options, ChoiceOption{Export: o, Display: pdf.Text(o)})
	}
	return f
}

func TestLogicalFieldSetText(t *testing.T) {
	f1, f2 := NewTextField("t"), NewTextField("t")
	g := NewLogicalField("t", f1, f2)
	if err := g.SetText("abc"); err != nil {
		t.Fatal(err)
	}
	for i, f := range []*TextField{f1, f2} {
		if f.Text == nil || f.Text.Value != pdf.Text("abc") {
			t.Errorf("field %d: Text = %v, want abc", i, f.Text)
		}
	}
}

func TestLogicalFieldSetTextAtomic(t *testing.T) {
	f1, f2 := NewTextField("t"), NewTextField("t")
	f2.MaxLen = 2
	g := NewLogicalField("t", f1, f2)
	before := snapshot(f1, f2)
	if err := g.SetText("abc"); err == nil {
		t.Error("MaxLen violation accepted")
	}
	checkUnchanged(t, before, f1, f2)

	f2.MaxLen = 0
	f1.Flags |= FieldMultiline
	before = snapshot(f1, f2)
	if err := g.SetText("a\nb"); err == nil {
		t.Error("line break in single-line field accepted")
	}
	checkUnchanged(t, before, f1, f2)
}

func TestLogicalFieldSelect(t *testing.T) {
	f1 := optionsField(0, "A", "B", "C")
	f2 := optionsField(0, "C", "A", "A")
	g := NewLogicalField("c", f1, f2)
	if err := g.Select(f1, []int{0}); err != nil {
		t.Fatal(err)
	}
	want := []pdf.TextString{pdf.Text("A")}
	for i, f := range []*ChoiceField{f1, f2} {
		if diff := cmp.Diff(want, f.Value); diff != "" {
			t.Errorf("field %d: value differs (-want +got):\n%s", i, diff)
		}
	}
	if f1.Selected != nil {
		t.Errorf("field 0: Selected = %v, want nil", f1.Selected)
	}
	// the display string occurs twice in the second field
	if diff := cmp.Diff([]int{1}, f2.Selected); diff != "" {
		t.Errorf("field 1: selected differs (-want +got):\n%s", diff)
	}
	if !f2.SelectionAgrees() {
		t.Error("field 1: selection does not agree with value")
	}
}

// TestLogicalFieldSelectFromOther checks that the indices refer to the
// options of the given field, and that every field takes its value.
func TestLogicalFieldSelectFromOther(t *testing.T) {
	f1 := optionsField(FieldMultiSelect, "A", "B", "C")
	f2 := optionsField(FieldMultiSelect, "C", "B", "A")
	g := NewLogicalField("c", f1, f2)
	if err := g.Select(f2, []int{0, 1}); err != nil {
		t.Fatal(err)
	}
	want := []pdf.TextString{pdf.Text("C"), pdf.Text("B")}
	for i, f := range []*ChoiceField{f1, f2} {
		if diff := cmp.Diff(want, f.Value); diff != "" {
			t.Errorf("field %d: value differs (-want +got):\n%s", i, diff)
		}
		if !f.SelectionAgrees() {
			t.Errorf("field %d: selection does not agree with value", i)
		}
	}
	if diff := cmp.Diff([]int{1, 2}, f1.Selected); diff != "" {
		t.Errorf("field 0: selected differs (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]int{0, 1}, f2.Selected); diff != "" {
		t.Errorf("field 1: selected differs (-want +got):\n%s", diff)
	}
}

func TestLogicalFieldSelectForeignField(t *testing.T) {
	f1 := optionsField(0, "A", "B")
	f2 := optionsField(0, "A", "B")
	other := optionsField(0, "A", "B")
	g := NewLogicalField("c", f1, f2)
	before := snapshot(f1, f2, other)
	for _, src := range []*ChoiceField{other, nil} {
		if err := g.Select(src, []int{0}); err == nil {
			t.Errorf("field %p outside the logical field accepted", src)
		}
	}
	checkUnchanged(t, before, f1, f2, other)
}

func TestLogicalFieldSelectAtomic(t *testing.T) {
	f1 := optionsField(0, "A", "B")
	f2 := optionsField(0, "A", "C")
	g := NewLogicalField("c", f1, f2)
	before := snapshot(f1, f2)
	if err := g.Select(f1, []int{1}); err == nil {
		t.Error("missing option accepted")
	}
	checkUnchanged(t, before, f1, f2)

	if err := g.Select(f1, []int{5}); err == nil {
		t.Error("index out of range accepted")
	}
	checkUnchanged(t, before, f1, f2)
}

func TestLogicalFieldSelectOrder(t *testing.T) {
	f1 := optionsField(FieldMultiSelect, "A", "B")
	f2 := optionsField(FieldMultiSelect, "B", "A")
	f1.DefaultAppearance, f2.DefaultAppearance = testDA, testDA
	g := NewLogicalField("c", f1, f2)
	if err := g.Select(f1, []int{0, 1}); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(f1.Value, f2.Value); diff != "" {
		t.Errorf("values differ (-f1 +f2):\n%s", diff)
	}
	for i, f := range []*ChoiceField{f1, f2} {
		if !f.SelectionAgrees() {
			t.Errorf("field %d: selection does not agree with value", i)
		}
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	form := &InteractiveForm{Fields: []Node{f1, f2}}
	if _, err := form.Encode(rm); err != nil {
		t.Errorf("encode failed: %v", err)
	}
}

func TestLogicalFieldSelectMixedMultiSelect(t *testing.T) {
	f1 := optionsField(FieldMultiSelect, "A", "B")
	f2 := optionsField(0, "A", "B")
	g := NewLogicalField("c", f1, f2)
	before := snapshot(f1, f2)
	if err := g.Select(f1, []int{0, 1}); err == nil {
		t.Error("multiple selection accepted by single-select field")
	}
	checkUnchanged(t, before, f1, f2)
}

// TestLogicalFieldSelectNoneMixedMultiSelect checks that clearing the
// selection gives each field the empty value its own MultiSelect flag
// calls for.
func TestLogicalFieldSelectNoneMixedMultiSelect(t *testing.T) {
	for _, multiFirst := range []bool{true, false} {
		multi := optionsField(FieldMultiSelect, "A", "B")
		single := optionsField(0, "A", "B")
		fields := []Field{multi, single}
		if !multiFirst {
			fields = []Field{single, multi}
		}
		g := NewLogicalField("c", fields...)
		src := fields[0].(*ChoiceField)
		if err := g.Select(src, []int{0}); err != nil {
			t.Fatal(err)
		}
		if err := g.Select(src, nil); err != nil {
			t.Fatal(err)
		}
		if multi.Value == nil || len(multi.Value) != 0 {
			t.Errorf("multiFirst=%v: multi-select Value = %#v, want explicit empty", multiFirst, multi.Value)
		}
		if single.Value != nil {
			t.Errorf("multiFirst=%v: single-select Value = %#v, want nil", multiFirst, single.Value)
		}
	}
}

func TestLogicalFieldSelectMultiset(t *testing.T) {
	f1 := optionsField(FieldMultiSelect, "A", "A", "B")
	f2 := optionsField(FieldMultiSelect, "A", "B")
	g := NewLogicalField("c", f1, f2)
	before := snapshot(f1, f2)
	if err := g.Select(f1, []int{0, 1}); err == nil {
		t.Error("selection of two A accepted by field with one A")
	}
	checkUnchanged(t, before, f1, f2)

	f2 = optionsField(FieldMultiSelect, "A", "B", "A")
	g.Fields[1] = f2
	if err := g.Select(f1, []int{0, 1}); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]int{0, 2}, f2.Selected); diff != "" {
		t.Errorf("selected differs (-want +got):\n%s", diff)
	}
	if !f2.SelectionAgrees() {
		t.Error("selection does not agree with value")
	}
}

func TestLogicalFieldSetEditText(t *testing.T) {
	f1 := optionsField(FieldCombo|FieldEdit, "A", "B")
	f2 := optionsField(FieldCombo|FieldEdit, "B")
	g := NewLogicalField("c", f1, f2)
	if err := g.SetEditText("B"); err != nil {
		t.Fatal(err)
	}
	want := []pdf.TextString{pdf.Text("B")}
	for i, f := range []*ChoiceField{f1, f2} {
		if diff := cmp.Diff(want, f.Value); diff != "" {
			t.Errorf("field %d: value differs (-want +got):\n%s", i, diff)
		}
	}
}

func TestLogicalFieldSetEditTextAtomic(t *testing.T) {
	f1 := optionsField(FieldCombo|FieldEdit, "A", "B")
	f2 := optionsField(FieldCombo, "A", "B")
	g := NewLogicalField("c", f1, f2)
	before := snapshot(f1, f2)
	if err := g.SetEditText("x"); err == nil {
		t.Error("non-editable field accepted")
	}
	checkUnchanged(t, before, f1, f2)
}

func TestLogicalFieldSetState(t *testing.T) {
	f1, ws1 := radioWith(0, "A", "B")
	f2, ws2 := radioWith(0, "C")
	g := NewLogicalField("r", f1, f2)

	if err := g.SetState("C"); err != nil {
		t.Fatal(err)
	}
	if f1.State != "C" || f2.State != "C" {
		t.Errorf("states %q, %q, want C", f1.State, f2.State)
	}
	checkStates(t, ws1, "Off", "Off")
	checkStates(t, ws2, "C")

	before := snapshot(f1, f2)
	for _, v := range []pdf.Name{"Z", ""} {
		if err := g.SetState(v); err == nil {
			t.Errorf("SetState(%q) did not fail", v)
		}
	}
	checkUnchanged(t, before, f1, f2)
	checkStates(t, ws1, "Off", "Off")
	checkStates(t, ws2, "C")

	if err := g.SetState("Off"); err != nil {
		t.Fatal(err)
	}
	if f1.State != "Off" || f2.State != "Off" {
		t.Errorf("states %q, %q, want Off", f1.State, f2.State)
	}
	checkStates(t, ws2, "Off")
}

func TestLogicalFieldToggle(t *testing.T) {
	f1, ws1 := radioWith(0, "A", "B")
	f2, ws2 := radioWith(0, "A", "C")
	g := NewLogicalField("r", f1, f2)

	if err := g.Toggle(ws2[0]); err != nil {
		t.Fatal(err)
	}
	if f1.State != "A" || f2.State != "A" {
		t.Errorf("states %q, %q, want A", f1.State, f2.State)
	}
	checkStates(t, ws1, "A", "Off")
	checkStates(t, ws2, "A", "Off")

	if err := g.Toggle(ws1[0]); err != nil {
		t.Fatal(err)
	}
	if f1.State != "Off" || f2.State != "Off" {
		t.Errorf("states %q, %q, want Off", f1.State, f2.State)
	}
	checkStates(t, ws1, "Off", "Off")
	checkStates(t, ws2, "Off", "Off")
}

func TestLogicalFieldToggleForeignWidget(t *testing.T) {
	f1, ws1 := radioWith(0, "A", "B")
	f2, ws2 := radioWith(0, "A")
	_, wo := radioWith(0, "A")
	g := NewLogicalField("r", f1, f2)
	before := snapshot(f1, f2)
	if err := g.Toggle(wo[0]); err == nil {
		t.Error("foreign widget accepted")
	}
	if err := g.Toggle(nil); err == nil {
		t.Error("nil widget accepted")
	}
	checkUnchanged(t, before, f1, f2)
	checkStates(t, ws1, "Off", "Off")
	checkStates(t, ws2, "Off")
	checkStates(t, wo, "Off")
}

func TestLogicalFieldWrongType(t *testing.T) {
	text := NewLogicalField("t", NewTextField("t"))
	choice := NewLogicalField("c", NewChoiceField("c"))
	push := NewLogicalField("p", NewPushButtonField("p"))
	mixed := NewLogicalField("m", NewTextField("m"), NewChoiceField("m"))
	w := &fakeWidget{}

	if err := choice.SetText("x"); err == nil {
		t.Error("SetText on choice fields accepted")
	}
	if err := mixed.SetText("x"); err == nil {
		t.Error("SetText on mixed fields accepted")
	}
	if err := text.Select(NewChoiceField("t"), []int{0}); err == nil {
		t.Error("Select on text fields accepted")
	}
	if err := text.SetEditText("x"); err == nil {
		t.Error("SetEditText on text fields accepted")
	}
	for _, g := range []*LogicalField{text, push} {
		if err := g.SetState("Off"); err == nil {
			t.Errorf("SetState on %s accepted", g.Name)
		}
		if err := g.Toggle(w); err == nil {
			t.Errorf("Toggle on %s accepted", g.Name)
		}
	}
}

func TestLogicalFieldEmpty(t *testing.T) {
	g := &LogicalField{}
	if err := g.SetState("Off"); err == nil {
		t.Error("empty logical field accepted a value")
	}
	if err := g.Toggle(&fakeWidget{}); err == nil {
		t.Error("empty logical field accepted a click")
	}
	if err := g.SetText("x"); err == nil {
		t.Error("empty logical field accepted text")
	}
	if err := g.Select(NewChoiceField("c"), nil); err == nil {
		t.Error("empty logical field accepted a selection")
	}
	if err := g.SetEditText("x"); err == nil {
		t.Error("empty logical field accepted edit text")
	}
}

func TestLogicalFieldNilWidget(t *testing.T) {
	f, ws := radioWith(0, "A", "B")
	f.Widgets = []Widget{ws[0], nil, ws[1]}
	g := NewLogicalField("r", f)

	if err := g.Toggle(ws[1]); err != nil {
		t.Fatal(err)
	}
	if f.State != "B" {
		t.Errorf("state %q, want B", f.State)
	}
	checkStates(t, ws, "Off", "B")

	if err := g.SetState("A"); err != nil {
		t.Fatal(err)
	}
	if f.State != "A" {
		t.Errorf("state %q, want A", f.State)
	}
	checkStates(t, ws, "A", "Off")
}
