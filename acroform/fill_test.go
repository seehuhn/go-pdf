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
package acroform

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

type fakeWidget struct {
	states []pdf.Name
	as     pdf.Name
	f      Field
}

func (w *fakeWidget) Encode(rm *pdf.ResourceManager) (pdf.Native, error) { return nil, nil }
func (w *fakeWidget) ParentField() Field                                 { return w.f }
func (w *fakeWidget) AppearanceStateNames() []pdf.Name                   { return w.states }
func (w *fakeWidget) CurrentAppearanceState() pdf.Name                   { return w.as }
func (w *fakeWidget) SetAppearanceState(n pdf.Name)                      { w.as = n }

func TestOnStateFromAppearance(t *testing.T) {
	f := NewButtonField("r", ButtonRadio)
	f.Widgets = []Widget{
		&fakeWidget{states: []pdf.Name{"Off", "A"}, f: f},
		&fakeWidget{states: []pdf.Name{"B", "Off"}, f: f},
	}
	if got := f.OnState(0); got != "A" {
		t.Errorf("OnState(0) = %q, want A", got)
	}
	if got := f.OnState(1); got != "B" {
		t.Errorf("OnState(1) = %q, want B", got)
	}
}

func TestOnStateAppearanceBeatsOpt(t *testing.T) {
	f := NewButtonField("r", ButtonRadio)
	f.ExportValues = []string{"x", "y"}
	f.Widgets = []Widget{&fakeWidget{states: []pdf.Name{"Off", "A"}, f: f}}
	if got := f.OnState(0); got != "A" {
		t.Errorf("OnState(0) = %q, want A", got)
	}
}

// TestCheckBoxUnusableExportValue checks that a check box whose export value
// cannot name a state ("" or "Off") still toggles on, under the name "Yes",
// and exports its export value.
func TestCheckBoxUnusableExportValue(t *testing.T) {
	for _, ev := range []string{"", "Off"} {
		f := NewButtonField("c", ButtonCheckbox)
		f.ExportValues = []string{ev}
		f.Widgets = []Widget{&fakeWidget{f: f}}
		f.toggle(0)
		if f.State != "Yes" {
			t.Errorf("export value %q: State = %q, want Yes", ev, f.State)
		}
		if diff := cmp.Diff([]string{ev}, f.ExportData()); diff != "" {
			t.Errorf("export value %q: ExportData mismatch (-want +got):\n%s", ev, diff)
		}
	}
}

func TestOnStateFallback(t *testing.T) {
	f := NewButtonField("r", ButtonRadio)
	f.ExportValues = []string{"x"}
	f.Widgets = []Widget{&fakeWidget{f: f}}
	if got := f.OnState(0); got != "0" {
		t.Errorf("ExportValues: OnState(0) = %q, want 0", got)
	}

	c := NewButtonField("c", ButtonCheckbox)
	c.State = "Yes"
	c.Widgets = []Widget{&fakeWidget{f: c}}
	if got := c.OnState(0); got != "Yes" {
		t.Errorf("V: OnState(0) = %q, want Yes", got)
	}

	r := NewButtonField("r2", ButtonRadio)
	r.Widgets = []Widget{&fakeWidget{f: r}}
	if got := r.OnState(0); got != "0" {
		t.Errorf("default: OnState(0) = %q, want 0", got)
	}
	if got := r.OnState(5); got != "" {
		t.Errorf("out of range: OnState(5) = %q, want empty", got)
	}
}

func radioWith(flags FieldFlags, names ...pdf.Name) (*ButtonField, []*fakeWidget) {
	f := NewButtonField("r", ButtonRadio)
	f.Flags |= flags
	var ws []*fakeWidget
	for _, n := range names {
		w := &fakeWidget{states: []pdf.Name{"Off", n}, as: "Off", f: f}
		ws = append(ws, w)
		f.Widgets = append(f.Widgets, w)
	}
	return f, ws
}

func states(ws []*fakeWidget) []pdf.Name {
	var out []pdf.Name
	for _, w := range ws {
		out = append(out, w.as)
	}
	return out
}

func checkStates(t *testing.T, ws []*fakeWidget, want ...pdf.Name) {
	t.Helper()
	if diff := cmp.Diff(want, states(ws)); diff != "" {
		t.Errorf("appearance states differ (-want +got):\n%s", diff)
	}
}

func TestToggleCheckbox(t *testing.T) {
	f := NewButtonField("c", ButtonCheckbox)
	w := &fakeWidget{states: []pdf.Name{"Off", "Yes"}, as: "Off", f: f}
	f.Widgets = []Widget{w}
	f.toggle(0)
	if f.State != "Yes" || w.as != "Yes" {
		t.Errorf("on: V=%q AS=%q, want Yes", f.State, w.as)
	}
	f.toggle(0)
	if f.State != "Off" || w.as != "Off" {
		t.Errorf("off: V=%q AS=%q, want Off", f.State, w.as)
	}
}

func TestToggleRadioSelects(t *testing.T) {
	f, ws := radioWith(0, "A", "B", "C")
	f.toggle(1)
	if f.State != "B" {
		t.Errorf("V = %q, want B", f.State)
	}
	checkStates(t, ws, "Off", "B", "Off")
}

func TestToggleRadioClears(t *testing.T) {
	f, ws := radioWith(0, "A", "B", "C")
	if err := NewLogicalField("", f).SetState("B"); err != nil {
		t.Fatal(err)
	}
	f.toggle(1)
	if f.State != "Off" {
		t.Errorf("V = %q, want Off", f.State)
	}
	checkStates(t, ws, "Off", "Off", "Off")
}

func TestToggleRadioNoToggleToOff(t *testing.T) {
	f, ws := radioWith(FieldNoToggleToOff, "A", "B", "C")
	if err := NewLogicalField("", f).SetState("B"); err != nil {
		t.Fatal(err)
	}
	f.toggle(1)
	if f.State != "B" {
		t.Errorf("V = %q, want B", f.State)
	}
	checkStates(t, ws, "Off", "B", "Off")
}

func TestToggleRadioUnison(t *testing.T) {
	f, ws := radioWith(FieldRadiosInUnison, "A", "A", "B")
	f.toggle(0)
	checkStates(t, ws, "A", "A", "Off")
}

func TestToggleRadioSharedNameNotUnison(t *testing.T) {
	f, ws := radioWith(0, "A", "A", "B")
	f.toggle(1)
	if f.State != "A" {
		t.Errorf("V = %q, want A", f.State)
	}
	checkStates(t, ws, "Off", "A", "Off")
}

// TestToggleKeepsOnStateFromDV checks that a check box without an
// appearance keeps its on-state name through off and on again.
func TestToggleKeepsOnStateFromDV(t *testing.T) {
	f := NewButtonField("c", ButtonCheckbox)
	f.State, f.DefaultState = "Yes", "Yes"
	w := &fakeWidget{as: "Yes", f: f}
	f.Widgets = []Widget{w}
	for _, want := range []pdf.Name{"Off", "Yes"} {
		f.toggle(0)
		if f.State != want || w.as != want {
			t.Errorf("V=%q AS=%q, want %q", f.State, w.as, want)
		}
	}
	if err := NewLogicalField("", f).SetState("Off"); err != nil {
		t.Fatal(err)
	}
	if err := NewLogicalField("", f).SetState("Yes"); err != nil {
		t.Errorf("SetState(Yes) after Off: %v", err)
	}
}

// TestSetStateExportValuesBeyondWidgets checks that SetState accepts the
// made-up on-state of an ExportValues entry which has no widget, and not the
// export value itself.
func TestSetStateExportValuesBeyondWidgets(t *testing.T) {
	f, ws := radioWith(0, "A", "B")
	f.ExportValues = []string{"A", "B", "C"}
	if err := NewLogicalField("", f).SetState("C"); err == nil {
		t.Error("SetState accepted an export value")
	}
	if err := NewLogicalField("", f).SetState("0"); err != nil {
		t.Fatal(err)
	}
	if f.State != "0" {
		t.Errorf("V = %q, want 0", f.State)
	}
	checkStates(t, ws, "Off", "Off")
	if err := NewLogicalField("", f).SetState("Z"); err == nil {
		t.Error("SetState with unknown state did not fail")
	}
}

func TestSetState(t *testing.T) {
	f, ws := radioWith(0, "A", "B", "B")
	if err := NewLogicalField("", f).SetState("B"); err != nil {
		t.Fatal(err)
	}
	checkStates(t, ws, "Off", "B", "B")
	if err := NewLogicalField("", f).SetState("Off"); err != nil {
		t.Fatal(err)
	}
	if f.State != "Off" {
		t.Errorf("V = %q, want Off", f.State)
	}
	checkStates(t, ws, "Off", "Off", "Off")
	if err := NewLogicalField("", f).SetState(""); err == nil {
		t.Error("SetState(\"\") did not fail")
	}
}

func TestSetStateUnknownStateErrors(t *testing.T) {
	f, ws := radioWith(0, "A", "B")
	if err := NewLogicalField("", f).SetState("A"); err != nil {
		t.Fatal(err)
	}
	if err := NewLogicalField("", f).SetState("Z"); err == nil {
		t.Error("SetState with unknown state did not fail")
	}
	if f.State != "A" {
		t.Errorf("V = %q, want A", f.State)
	}
	checkStates(t, ws, "A", "Off")

	bare := NewButtonField("c", ButtonCheckbox)
	if err := NewLogicalField("", bare).SetState("Yes"); err != nil {
		t.Errorf("field without widgets rejected value: %v", err)
	}
}

func TestSetText(t *testing.T) {
	f := NewTextField("t")
	if err := NewLogicalField("", f).SetText("hello"); err != nil {
		t.Fatal(err)
	}
	if f.Text == nil || f.Text.Value != pdf.Text("hello") || f.Text.IsStream {
		t.Errorf("V = %v, want plain hello", f.Text)
	}
	if err := NewLogicalField("", f).SetText(""); err != nil {
		t.Fatal(err)
	}
	if f.Text == nil || !f.Text.Value.IsZero() {
		t.Errorf("V = %v, want explicit empty value", f.Text)
	}
}

func TestSetTextKeepsStreamForm(t *testing.T) {
	f := NewTextField("t")
	f.Text = &pdf.StringOrStream{Value: pdf.Text("old"), IsStream: true}
	if err := NewLogicalField("", f).SetText("new"); err != nil {
		t.Fatal(err)
	}
	if !f.Text.IsStream || f.Text.Value != pdf.Text("new") {
		t.Errorf("V = %v, want stream form with new", f.Text)
	}
}

func TestSetTextClearsRichText(t *testing.T) {
	f := NewTextField("t")
	f.RichText = &pdf.StringOrStream{Value: pdf.Text("<p>x</p>")}
	if err := NewLogicalField("", f).SetText("x"); err != nil {
		t.Fatal(err)
	}
	if f.RichText != nil {
		t.Errorf("RichText = %v, want nil", f.RichText)
	}
}

func TestSetTextErrors(t *testing.T) {
	limited := NewTextField("l")
	limited.MaxLen = 3
	single := NewTextField("s")
	for _, tc := range []struct {
		name string
		f    *TextField
		s    string
	}{
		{"too long", limited, "abcd"},
		{"newline", single, "a\nb"},
		{"carriage return", single, "a\rb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.f.Text = nil
			if err := NewLogicalField("", tc.f).SetText(tc.s); err == nil {
				t.Error("no error")
			}
			if tc.f.Text != nil {
				t.Errorf("V changed to %v", tc.f.Text)
			}
		})
	}

	multi := NewTextField("m")
	multi.Flags |= FieldMultiline
	if err := NewLogicalField("", multi).SetText("a\nb"); err != nil {
		t.Errorf("multiline: %v", err)
	}
}

func TestSetTextMaxLenCountsRunes(t *testing.T) {
	f := NewTextField("t")
	f.MaxLen = 3
	if err := NewLogicalField("", f).SetText("äöü"); err != nil {
		t.Error(err)
	}
}

func selectField(flags FieldFlags) *ChoiceField {
	f := NewChoiceField("c")
	f.Flags = flags
	for _, s := range []string{"A", "B", "C"} {
		f.Options = append(f.Options, ChoiceOption{Export: s, Display: pdf.Text(s)})
	}
	return f
}

func TestSelect(t *testing.T) {
	f := selectField(FieldMultiSelect)
	idx := []int{2, 0, 2}
	if err := NewLogicalField("", f).Select(f, idx); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]int{0, 2}, f.Selected); diff != "" {
		t.Errorf("Selected differs (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]pdf.TextString{pdf.Text("A"), pdf.Text("C")}, f.Value); diff != "" {
		t.Errorf("V differs (-want +got):\n%s", diff)
	}
	if !f.SelectionAgrees() {
		t.Error("selection disagrees")
	}
	if diff := cmp.Diff([]int{2, 0, 2}, idx); diff != "" {
		t.Errorf("argument modified (-want +got):\n%s", diff)
	}
}

func TestSelectErrors(t *testing.T) {
	single := selectField(0)
	if err := NewLogicalField("", single).Select(single, []int{3}); err == nil {
		t.Error("index out of range: no error")
	}
	if err := NewLogicalField("", single).Select(single, []int{-1}); err == nil {
		t.Error("negative index: no error")
	}
	if err := NewLogicalField("", single).Select(single, []int{0, 1}); err == nil {
		t.Error("two selections in single-select field: no error")
	}
	if err := NewLogicalField("", single).Select(single, []int{1, 1}); err != nil {
		t.Errorf("duplicates collapse to one selection: %v", err)
	}
	failed := selectField(0)
	if err := NewLogicalField("", failed).Select(failed, []int{5}); err == nil {
		t.Fatal("no error")
	}
	if failed.Value != nil || failed.Selected != nil {
		t.Error("failed Select changed the field")
	}
}

func TestSelectNone(t *testing.T) {
	f := selectField(FieldMultiSelect)
	if err := NewLogicalField("", f).Select(f, []int{1}); err != nil {
		t.Fatal(err)
	}
	if err := NewLogicalField("", f).Select(f, nil); err != nil {
		t.Fatal(err)
	}
	if f.Value == nil || len(f.Value) != 0 {
		t.Errorf("V = %#v, want non-nil empty", f.Value)
	}
	if f.Selected != nil {
		t.Errorf("Selected = %v, want nil", f.Selected)
	}
	if !f.SelectionAgrees() {
		t.Error("selection disagrees")
	}
}

// TestSelectSelectedOnlyWhereNeeded checks that Select records the indices
// only where Value does not identify the selected options.
func TestSelectSelectedOnlyWhereNeeded(t *testing.T) {
	f := selectField(FieldMultiSelect)
	if err := NewLogicalField("", f).Select(f, []int{1}); err != nil {
		t.Fatal(err)
	}
	if f.Selected != nil {
		t.Errorf("one unique option: Selected = %v, want nil", f.Selected)
	}

	f = selectField(0)
	f.Options = append(f.Options, ChoiceOption{Export: "x", Display: pdf.Text("B")})
	if err := NewLogicalField("", f).Select(f, []int{3}); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]int{3}, f.Selected); diff != "" {
		t.Errorf("shared display string: Selected differs (-want +got):\n%s", diff)
	}
	if err := NewLogicalField("", f).Select(f, []int{0}); err != nil {
		t.Fatal(err)
	}
	if f.Selected != nil {
		t.Errorf("unique option: Selected = %v, want nil", f.Selected)
	}
}

// TestSelectWritablePDF13 checks that a selection made with Select in a
// single-select field can be written to a file without /I support.
func TestSelectWritablePDF13(t *testing.T) {
	f := selectField(0)
	if err := NewLogicalField("", f).Select(f, []int{1}); err != nil {
		t.Fatal(err)
	}
	w, _ := memfile.NewPDFWriter(t, pdf.V1_3, nil)
	dict, err := ValueEntries(pdf.NewResourceManager(w), f)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(pdf.Dict{"V": pdf.Text("B")}, dict); diff != "" {
		t.Errorf("value entries differ (-want +got):\n%s", diff)
	}
}

func TestSelectNoneSingle(t *testing.T) {
	f := selectField(0)
	if err := NewLogicalField("", f).Select(f, []int{1}); err != nil {
		t.Fatal(err)
	}
	if err := NewLogicalField("", f).Select(f, []int{}); err != nil {
		t.Fatal(err)
	}
	if f.Value != nil || f.Selected != nil {
		t.Errorf("V, Selected = %#v, %v, want nil, nil", f.Value, f.Selected)
	}
	if !f.SelectionAgrees() {
		t.Error("selection disagrees")
	}
}

func TestChoiceValueObject(t *testing.T) {
	empty := []pdf.TextString{}
	one := []pdf.TextString{pdf.Text("a")}
	for _, tc := range []struct {
		name  string
		vals  []pdf.TextString
		multi bool
		want  pdf.Object
	}{
		{"nil multi", nil, true, nil},
		{"nil single", nil, false, nil},
		{"empty multi", empty, true, pdf.Array{}},
		{"empty single", empty, false, nil},
		{"one", one, false, pdf.Text("a")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := choiceValueObject(tc.vals, tc.multi)
			if tc.want == nil {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("differs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChoiceSettersClearRichText(t *testing.T) {
	rv := &pdf.StringOrStream{Value: pdf.Text("<p>x</p>")}

	f := selectField(0)
	f.RichText = rv
	if err := NewLogicalField("", f).Select(f, []int{1}); err != nil {
		t.Fatal(err)
	}
	if f.RichText != nil {
		t.Errorf("Select: RichText = %v, want nil", f.RichText)
	}

	f = selectField(FieldCombo | FieldEdit)
	f.RichText = rv
	if err := NewLogicalField("", f).SetEditText("free"); err != nil {
		t.Fatal(err)
	}
	if f.RichText != nil {
		t.Errorf("SetEditText: RichText = %v, want nil", f.RichText)
	}
}

func TestSetEditTextSelected(t *testing.T) {
	f := selectField(FieldCombo | FieldEdit)
	if err := NewLogicalField("", f).SetEditText("B"); err != nil {
		t.Fatal(err)
	}
	if f.Selected != nil {
		t.Errorf("unique option: Selected = %v, want nil", f.Selected)
	}
	f.Options = append(f.Options, ChoiceOption{Export: "b2", Display: pdf.Text("B")})
	if err := NewLogicalField("", f).SetEditText("B"); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]int{1}, f.Selected); diff != "" {
		t.Errorf("shared display string: Selected differs (-want +got):\n%s", diff)
	}
	if err := NewLogicalField("", f).SetEditText("free"); err != nil {
		t.Fatal(err)
	}
	if f.Selected != nil {
		t.Errorf("Selected = %v, want nil", f.Selected)
	}
	if diff := cmp.Diff([]pdf.TextString{pdf.Text("free")}, f.Value); diff != "" {
		t.Errorf("V differs (-want +got):\n%s", diff)
	}

	for _, flags := range []FieldFlags{0, FieldCombo, FieldEdit} {
		if err := NewLogicalField("", selectField(flags)).SetEditText("B"); err == nil {
			t.Errorf("flags %v: no error", flags)
		}
	}
}

// TestValueEntriesOnlyValue checks that ValueEntries encodes the value
// entries alone: it embeds no other object of the field, and checks of
// entries other than the value do not make it fail.
func TestValueEntriesOnlyValue(t *testing.T) {
	js := &action.JavaScript{JS: &pdf.StringOrStream{Value: pdf.Text("app.alert(1)"), IsStream: true}}

	text := textField("t")
	text.AA = &triggers.Form{Validate: js}
	text.DefaultText = &pdf.StringOrStream{Value: pdf.Text("default"), IsStream: true}
	text.Text = &pdf.StringOrStream{Value: pdf.Text("value")}
	text.Flags = FieldComb // without MaxLen, which Encode refuses

	choice := NewChoiceField("c")
	choice.DefaultAppearance = testDA
	choice.Options = []ChoiceOption{{Export: "x", Display: pdf.Text("x")}}
	choice.TopIndex = 5 // out of range, which Encode refuses
	choice.AA = &triggers.Form{Calculate: js}
	choice.Value = []pdf.TextString{pdf.Text("x")}
	choice.Selected = []int{0}

	button := NewButtonField("b", ButtonCheckbox)
	button.AA = &triggers.Form{Calculate: js}
	button.State = "Off"

	cases := []struct {
		f    Field
		want pdf.Dict
	}{
		{text, pdf.Dict{"V": pdf.String("value")}},
		{choice, pdf.Dict{"V": pdf.Text("x"), "I": pdf.Array{pdf.Integer(0)}}},
		{button, pdf.Dict{"V": pdf.Name("Off")}},
	}
	for _, c := range cases {
		t.Run(string(c.f.FieldType()), func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			before := w.Alloc()
			got, err := ValueEntries(pdf.NewResourceManager(w), c.f)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("entries differ (-want +got):\n%s", diff)
			}
			if after := w.Alloc(); after.Number() != before.Number()+1 {
				t.Errorf("%d objects allocated", after.Number()-before.Number()-1)
			}
		})
	}
}

// ValueEntries writes "Off" explicitly, so that a patched field does not
// inherit a value from an ancestor
func TestValueEntriesButtonOff(t *testing.T) {
	tests := []struct {
		name    string
		variant ButtonVariant
		state   pdf.Name
		want    pdf.Object
	}{
		{"empty", ButtonCheckbox, "", pdf.Name("Off")},
		{"off", ButtonRadio, "Off", pdf.Name("Off")},
		{"on", ButtonCheckbox, "Yes", pdf.Name("Yes")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := NewButtonField("b", tc.variant)
			f.State = tc.state
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			dict, err := ValueEntries(pdf.NewResourceManager(w), f)
			if err != nil {
				t.Fatal(err)
			}
			if got := dict["V"]; got != tc.want {
				t.Errorf("/V = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOnStateFromCurrentState(t *testing.T) {
	f := NewButtonField("r", ButtonRadio)
	f.Widgets = []Widget{&fakeWidget{as: "A", f: f}, &fakeWidget{as: "Off", f: f}}
	if got := f.OnState(0); got != "A" {
		t.Errorf("OnState(0) = %q, want A", got)
	}
	if got := f.OnState(1); got != "0" {
		t.Errorf("OnState(1) = %q, want 0", got)
	}
}

// TestPositionalOnStateDistinct checks that made-up radio on-states avoid
// the names which the widgets give themselves and each other.
func TestPositionalOnStateDistinct(t *testing.T) {
	f := NewButtonField("r", ButtonRadio)
	f.Widgets = []Widget{
		&fakeWidget{f: f},
		&fakeWidget{states: []pdf.Name{"Off", "1"}, f: f},
		&fakeWidget{f: f},
		&fakeWidget{states: []pdf.Name{"Off", "0"}, f: f},
		&fakeWidget{f: f},
	}
	want := []pdf.Name{"2", "1", "3", "0", "4"}
	for i, w := range want {
		if got := f.OnState(i); got != w {
			t.Errorf("OnState(%d) = %q, want %q", i, got, w)
		}
	}
}

// TestPositionalOnStateMany checks that made-up radio on-states stay
// distinct in a field with many buttons, and that OnState agrees with
// OnStates.
func TestPositionalOnStateMany(t *testing.T) {
	const n = 1000
	f := NewButtonField("r", ButtonRadio)
	f.Widgets = []Widget{&fakeWidget{states: []pdf.Name{"Off", "7"}, f: f}}
	for range n {
		f.Widgets = append(f.Widgets, &fakeWidget{f: f})
	}

	states := f.OnStates()
	if len(states) != n+1 {
		t.Fatalf("got %d on-states, want %d", len(states), n+1)
	}
	seen := map[pdf.Name]bool{}
	for i, s := range states {
		if seen[s] {
			t.Errorf("on-state %q used twice", s)
		}
		seen[s] = true
		if got := f.OnState(i); got != s {
			t.Errorf("OnState(%d) = %q, want %q", i, got, s)
		}
	}
	if states[7] != "6" || states[8] != "8" {
		t.Errorf("states[7:9] = %q, want [6 8]", states[7:9])
	}
}

// TestSetStateNamesBeforeChange checks that SetState identifies the buttons
// before it changes their states: a widget naming its on-state only through
// its current state loses that name when it is turned off.
func TestSetStateNamesBeforeChange(t *testing.T) {
	f := NewButtonField("r", ButtonRadio)
	w0 := &fakeWidget{as: "A", f: f}
	w1 := &fakeWidget{f: f}
	f.Widgets = []Widget{w0, w1}

	if err := NewLogicalField("", f).SetState("0"); err != nil {
		t.Fatal(err)
	}
	if w0.as != "Off" || w1.as != "0" {
		t.Errorf("AS = %q, %q, want Off, 0", w0.as, w1.as)
	}
}

// a radio button without widgets or export values has the single on-state
// "0"; a check box without them takes any value as its on-state
func TestSetStateWithoutWidgets(t *testing.T) {
	r := NewButtonField("r", ButtonRadio)
	if err := NewLogicalField("", r).SetState("Foo"); err == nil {
		t.Error("radio button accepted an unknown value")
	}
	if err := NewLogicalField("", r).SetState("0"); err != nil {
		t.Errorf("radio button rejected 0: %v", err)
	}

	c := NewButtonField("c", ButtonCheckbox)
	if err := NewLogicalField("", c).SetState("Yes"); err != nil {
		t.Errorf("check box rejected Yes: %v", err)
	}
	if got := c.OnState(0); got != "Yes" {
		t.Errorf("OnState(0) = %q, want Yes", got)
	}
}

// ValueEntries applies the single-line rule to the text value.
func TestValueEntriesLineBreak(t *testing.T) {
	f := textField("t")
	f.Text = &pdf.StringOrStream{Value: pdf.Text("a\nb")}
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	if _, err := ValueEntries(pdf.NewResourceManager(w), f); err == nil {
		t.Error("line break in single-line field accepted")
	}
}

// A password field holds its value in memory, and writing omits it.
func TestPasswordValueNotWritten(t *testing.T) {
	f := textField("pw")
	f.Flags = FieldPassword
	if err := NewLogicalField("", f).SetText("secret"); err != nil {
		t.Fatal(err)
	}
	f.RichText = &pdf.StringOrStream{Value: pdf.Text("<p>secret</p>")}
	if got := f.ExportData(); len(got) != 1 || got[0] != "secret" {
		t.Errorf("ExportData = %q, want [secret]", got)
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	entries, err := ValueEntries(rm, f)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("ValueEntries = %v, want none", entries)
	}
	dict, err := terminalEntries(rm, f)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []pdf.Name{"V", "RV"} {
		if _, ok := dict[key]; ok {
			t.Errorf("field dictionary has /%s", key)
		}
	}
}

// Same-named password fields agree on the value which is written, whatever
// they hold in memory.
func TestPasswordSameNameDifferentValues(t *testing.T) {
	a, b := textField("pw"), textField("pw")
	a.Flags, b.Flags = FieldPassword, FieldPassword
	a.Text = &pdf.StringOrStream{Value: pdf.Text("one")}
	b.Text = &pdf.StringOrStream{Value: pdf.Text("two")}
	if !sameTypeAndValue(a, b) {
		t.Error("password fields with different values do not agree")
	}
}

// a push button has no value entries
func TestValueEntriesPushButton(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	dict, err := ValueEntries(pdf.NewResourceManager(w), NewPushButtonField("p"))
	if err != nil {
		t.Fatal(err)
	}
	if len(dict) != 0 {
		t.Errorf("ValueEntries = %v, want none", dict)
	}
}
