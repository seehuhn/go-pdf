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
	f := NewButtonField("r")
	f.Flags |= FieldRadio
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
	f := NewButtonField("r")
	f.Flags |= FieldRadio
	f.Opt = []string{"x", "y"}
	f.Widgets = []Widget{&fakeWidget{states: []pdf.Name{"Off", "A"}, f: f}}
	if got := f.OnState(0); got != "A" {
		t.Errorf("OnState(0) = %q, want A", got)
	}
}

func TestOnStateFallback(t *testing.T) {
	f := NewButtonField("r")
	f.Flags |= FieldRadio
	f.Opt = []string{"x"}
	f.Widgets = []Widget{&fakeWidget{f: f}}
	if got := f.OnState(0); got != "x" {
		t.Errorf("Opt: OnState(0) = %q, want x", got)
	}

	c := NewButtonField("c")
	c.V = "Yes"
	c.Widgets = []Widget{&fakeWidget{f: c}}
	if got := c.OnState(0); got != "Yes" {
		t.Errorf("V: OnState(0) = %q, want Yes", got)
	}

	r := NewButtonField("r2")
	r.Flags |= FieldRadio
	r.Widgets = []Widget{&fakeWidget{f: r}}
	if got := r.OnState(0); got != "On" {
		t.Errorf("default: OnState(0) = %q, want On", got)
	}
	if got := r.OnState(5); got != "" {
		t.Errorf("out of range: OnState(5) = %q, want empty", got)
	}
}

func TestOnStateNonButton(t *testing.T) {
	f := NewButtonField("p")
	f.Flags |= FieldPushbutton
	f.Widgets = []Widget{&fakeWidget{states: []pdf.Name{"On"}, f: f}}
	if got := f.OnState(0); got != "" {
		t.Errorf("OnState(0) = %q, want empty", got)
	}
}

func radioWith(flags FieldFlags, names ...pdf.Name) (*ButtonField, []*fakeWidget) {
	f := NewButtonField("r")
	f.Flags |= FieldRadio | flags
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
	f := NewButtonField("c")
	w := &fakeWidget{states: []pdf.Name{"Off", "Yes"}, as: "Off", f: f}
	f.Widgets = []Widget{w}
	if err := f.Toggle(0); err != nil {
		t.Fatal(err)
	}
	if f.V != "Yes" || w.as != "Yes" {
		t.Errorf("on: V=%q AS=%q, want Yes", f.V, w.as)
	}
	if err := f.Toggle(0); err != nil {
		t.Fatal(err)
	}
	if f.V != "Off" || w.as != "Off" {
		t.Errorf("off: V=%q AS=%q, want Off", f.V, w.as)
	}
}

func TestToggleRadioSelects(t *testing.T) {
	f, ws := radioWith(0, "A", "B", "C")
	if err := f.Toggle(1); err != nil {
		t.Fatal(err)
	}
	if f.V != "B" {
		t.Errorf("V = %q, want B", f.V)
	}
	checkStates(t, ws, "Off", "B", "Off")
}

func TestToggleRadioClears(t *testing.T) {
	f, ws := radioWith(0, "A", "B", "C")
	if err := f.SetValue("B"); err != nil {
		t.Fatal(err)
	}
	if err := f.Toggle(1); err != nil {
		t.Fatal(err)
	}
	if f.V != "Off" {
		t.Errorf("V = %q, want Off", f.V)
	}
	checkStates(t, ws, "Off", "Off", "Off")
}

func TestToggleRadioNoToggleToOff(t *testing.T) {
	f, ws := radioWith(FieldNoToggleToOff, "A", "B", "C")
	if err := f.SetValue("B"); err != nil {
		t.Fatal(err)
	}
	if err := f.Toggle(1); err != nil {
		t.Fatal(err)
	}
	if f.V != "B" {
		t.Errorf("V = %q, want B", f.V)
	}
	checkStates(t, ws, "Off", "B", "Off")
}

func TestToggleRadioUnison(t *testing.T) {
	f, ws := radioWith(FieldRadiosInUnison, "A", "A", "B")
	if err := f.Toggle(0); err != nil {
		t.Fatal(err)
	}
	checkStates(t, ws, "A", "A", "Off")
}

func TestToggleRadioSharedNameNotUnison(t *testing.T) {
	f, ws := radioWith(0, "A", "A", "B")
	if err := f.Toggle(1); err != nil {
		t.Fatal(err)
	}
	if f.V != "A" {
		t.Errorf("V = %q, want A", f.V)
	}
	checkStates(t, ws, "Off", "A", "Off")
}

func TestTogglePushButtonErrors(t *testing.T) {
	f := NewButtonField("p")
	f.Flags |= FieldPushbutton
	f.Widgets = []Widget{&fakeWidget{f: f}}
	if err := f.Toggle(0); err == nil {
		t.Error("Toggle on push button did not fail")
	}
	if err := f.SetValue("A"); err == nil {
		t.Error("SetValue on push button did not fail")
	}
}

func TestToggleOutOfRange(t *testing.T) {
	f, _ := radioWith(0, "A")
	for _, i := range []int{-1, 1} {
		if err := f.Toggle(i); err == nil {
			t.Errorf("Toggle(%d) did not fail", i)
		}
	}
}

// TestToggleWithoutWidget checks that a field without a widget at the
// clicked index switches on and off by its value.
func TestToggleWithoutWidget(t *testing.T) {
	c := NewButtonField("c")
	for _, want := range []pdf.Name{"On", "Off", "On"} {
		if err := c.Toggle(0); err != nil {
			t.Fatal(err)
		}
		if c.V != want {
			t.Errorf("check box: V = %q, want %q", c.V, want)
		}
	}

	r, ws := radioWith(0, "A", "B")
	r.Opt = []string{"A", "B", "C"}
	if err := r.Toggle(2); err != nil {
		t.Fatal(err)
	}
	if r.V != "C" {
		t.Errorf("radio: V = %q, want C", r.V)
	}
	checkStates(t, ws, "Off", "Off")
	if err := r.Toggle(2); err != nil {
		t.Fatal(err)
	}
	if r.V != "Off" {
		t.Errorf("radio: V = %q, want Off", r.V)
	}
}

// TestToggleKeepsOnStateFromDV checks that a check box without an
// appearance keeps its on-state name through off and on again.
func TestToggleKeepsOnStateFromDV(t *testing.T) {
	f := NewButtonField("c")
	f.V, f.DV = "Yes", "Yes"
	w := &fakeWidget{as: "Yes", f: f}
	f.Widgets = []Widget{w}
	for _, want := range []pdf.Name{"Off", "Yes"} {
		if err := f.Toggle(0); err != nil {
			t.Fatal(err)
		}
		if f.V != want || w.as != want {
			t.Errorf("V=%q AS=%q, want %q", f.V, w.as, want)
		}
	}
	if err := f.SetValue("Off"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetValue("Yes"); err != nil {
		t.Errorf("SetValue(Yes) after Off: %v", err)
	}
}

// TestSetValueOptBeyondWidgets checks that SetValue accepts the on-state of
// an Opt entry which has no widget.
func TestSetValueOptBeyondWidgets(t *testing.T) {
	f, ws := radioWith(0, "A", "B")
	f.Opt = []string{"A", "B", "C"}
	if err := f.SetValue("C"); err != nil {
		t.Fatal(err)
	}
	if f.V != "C" {
		t.Errorf("V = %q, want C", f.V)
	}
	checkStates(t, ws, "Off", "Off")
	if err := f.SetValue("Z"); err == nil {
		t.Error("SetValue with unknown state did not fail")
	}
}

func TestSetValue(t *testing.T) {
	f, ws := radioWith(0, "A", "B", "B")
	if err := f.SetValue("B"); err != nil {
		t.Fatal(err)
	}
	checkStates(t, ws, "Off", "B", "B")
	if err := f.SetValue("Off"); err != nil {
		t.Fatal(err)
	}
	if f.V != "Off" {
		t.Errorf("V = %q, want Off", f.V)
	}
	checkStates(t, ws, "Off", "Off", "Off")
	if err := f.SetValue(""); err == nil {
		t.Error("SetValue(\"\") did not fail")
	}
}

func TestSetValueUnknownStateErrors(t *testing.T) {
	f, ws := radioWith(0, "A", "B")
	if err := f.SetValue("A"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetValue("Z"); err == nil {
		t.Error("SetValue with unknown state did not fail")
	}
	if f.V != "A" {
		t.Errorf("V = %q, want A", f.V)
	}
	checkStates(t, ws, "A", "Off")

	bare := NewButtonField("c")
	if err := bare.SetValue("Yes"); err != nil {
		t.Errorf("field without widgets rejected value: %v", err)
	}
}

func TestSetText(t *testing.T) {
	f := NewTextField("t")
	if err := f.SetText("hello"); err != nil {
		t.Fatal(err)
	}
	if f.V == nil || f.V.Value != pdf.Text("hello") || f.V.IsStream {
		t.Errorf("V = %v, want plain hello", f.V)
	}
	if err := f.SetText(""); err != nil {
		t.Fatal(err)
	}
	if f.V == nil || !f.V.Value.IsZero() {
		t.Errorf("V = %v, want explicit empty value", f.V)
	}
}

func TestSetTextKeepsStreamForm(t *testing.T) {
	f := NewTextField("t")
	f.V = &pdf.StringOrStream{Value: pdf.Text("old"), IsStream: true}
	if err := f.SetText("new"); err != nil {
		t.Fatal(err)
	}
	if !f.V.IsStream || f.V.Value != pdf.Text("new") {
		t.Errorf("V = %v, want stream form with new", f.V)
	}
}

func TestSetTextClearsRichValue(t *testing.T) {
	f := NewTextField("t")
	f.RichValue = &pdf.StringOrStream{Value: pdf.Text("<p>x</p>")}
	if err := f.SetText("x"); err != nil {
		t.Fatal(err)
	}
	if f.RichValue != nil {
		t.Errorf("RichValue = %v, want nil", f.RichValue)
	}
}

func TestSetTextErrors(t *testing.T) {
	pw := NewTextField("pw")
	pw.Flags |= FieldPassword
	limited := NewTextField("l")
	limited.MaxLen = 3
	single := NewTextField("s")
	for _, tc := range []struct {
		name string
		f    *TextField
		s    string
	}{
		{"password", pw, "x"},
		{"too long", limited, "abcd"},
		{"newline", single, "a\nb"},
		{"carriage return", single, "a\rb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.f.V = nil
			if err := tc.f.SetText(tc.s); err == nil {
				t.Error("no error")
			}
			if tc.f.V != nil {
				t.Errorf("V changed to %v", tc.f.V)
			}
		})
	}

	multi := NewTextField("m")
	multi.Flags |= FieldMultiline
	if err := multi.SetText("a\nb"); err != nil {
		t.Errorf("multiline: %v", err)
	}
}

func TestSetTextMaxLenCountsRunes(t *testing.T) {
	f := NewTextField("t")
	f.MaxLen = 3
	if err := f.SetText("äöü"); err != nil {
		t.Error(err)
	}
}

func selectField(flags FieldFlags) *ChoiceField {
	f := NewChoiceField("c")
	f.Flags = flags
	for _, s := range []string{"A", "B", "C"} {
		f.Opt = append(f.Opt, ChoiceOption{Export: pdf.Text(s), Display: pdf.Text(s)})
	}
	return f
}

func TestSelect(t *testing.T) {
	f := selectField(FieldMultiSelect)
	idx := []int{2, 0, 2}
	if err := f.Select(idx); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]int{0, 2}, f.Selected); diff != "" {
		t.Errorf("Selected differs (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]pdf.TextString{pdf.Text("A"), pdf.Text("C")}, f.V); diff != "" {
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
	if err := single.Select([]int{3}); err == nil {
		t.Error("index out of range: no error")
	}
	if err := single.Select([]int{-1}); err == nil {
		t.Error("negative index: no error")
	}
	if err := single.Select([]int{0, 1}); err == nil {
		t.Error("two selections in single-select field: no error")
	}
	if err := single.Select([]int{1, 1}); err != nil {
		t.Errorf("duplicates collapse to one selection: %v", err)
	}
	failed := selectField(0)
	if err := failed.Select([]int{5}); err == nil {
		t.Fatal("no error")
	}
	if failed.V != nil || failed.Selected != nil {
		t.Error("failed Select changed the field")
	}
}

func TestSelectNone(t *testing.T) {
	f := selectField(FieldMultiSelect)
	if err := f.Select([]int{1}); err != nil {
		t.Fatal(err)
	}
	if err := f.Select(nil); err != nil {
		t.Fatal(err)
	}
	if f.V == nil || len(f.V) != 0 {
		t.Errorf("V = %#v, want non-nil empty", f.V)
	}
	if f.Selected != nil {
		t.Errorf("Selected = %v, want nil", f.Selected)
	}
	if !f.SelectionAgrees() {
		t.Error("selection disagrees")
	}
}

// TestSelectSelectedOnlyWhereNeeded checks that Select records the indices
// only where V does not identify the selected options.
func TestSelectSelectedOnlyWhereNeeded(t *testing.T) {
	f := selectField(FieldMultiSelect)
	if err := f.Select([]int{1}); err != nil {
		t.Fatal(err)
	}
	if f.Selected != nil {
		t.Errorf("one unique option: Selected = %v, want nil", f.Selected)
	}

	f = selectField(0)
	f.Opt = append(f.Opt, ChoiceOption{Export: pdf.Text("x"), Display: pdf.Text("B")})
	if err := f.Select([]int{3}); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]int{3}, f.Selected); diff != "" {
		t.Errorf("shared display string: Selected differs (-want +got):\n%s", diff)
	}
	if err := f.Select([]int{0}); err != nil {
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
	if err := f.Select([]int{1}); err != nil {
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
	if err := f.Select([]int{1}); err != nil {
		t.Fatal(err)
	}
	if err := f.Select([]int{}); err != nil {
		t.Fatal(err)
	}
	if f.V != nil || f.Selected != nil {
		t.Errorf("V, Selected = %#v, %v, want nil, nil", f.V, f.Selected)
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

func TestChoiceSettersClearRichValue(t *testing.T) {
	rv := &pdf.StringOrStream{Value: pdf.Text("<p>x</p>")}

	f := selectField(0)
	f.RichValue = rv
	if err := f.Select([]int{1}); err != nil {
		t.Fatal(err)
	}
	if f.RichValue != nil {
		t.Errorf("Select: RichValue = %v, want nil", f.RichValue)
	}

	f = selectField(FieldCombo | FieldEdit)
	f.RichValue = rv
	if err := f.SetEditText("free"); err != nil {
		t.Fatal(err)
	}
	if f.RichValue != nil {
		t.Errorf("SetEditText: RichValue = %v, want nil", f.RichValue)
	}
}

func TestSetEditText(t *testing.T) {
	f := selectField(FieldCombo | FieldEdit)
	if err := f.SetEditText("B"); err != nil {
		t.Fatal(err)
	}
	if f.Selected != nil {
		t.Errorf("unique option: Selected = %v, want nil", f.Selected)
	}
	f.Opt = append(f.Opt, ChoiceOption{Export: pdf.Text("b2"), Display: pdf.Text("B")})
	if err := f.SetEditText("B"); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]int{1}, f.Selected); diff != "" {
		t.Errorf("shared display string: Selected differs (-want +got):\n%s", diff)
	}
	if err := f.SetEditText("free"); err != nil {
		t.Fatal(err)
	}
	if f.Selected != nil {
		t.Errorf("Selected = %v, want nil", f.Selected)
	}
	if diff := cmp.Diff([]pdf.TextString{pdf.Text("free")}, f.V); diff != "" {
		t.Errorf("V differs (-want +got):\n%s", diff)
	}

	for _, flags := range []FieldFlags{0, FieldCombo, FieldEdit} {
		if err := selectField(flags).SetEditText("B"); err == nil {
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
	text.DV = &pdf.StringOrStream{Value: pdf.Text("default"), IsStream: true}
	text.V = &pdf.StringOrStream{Value: pdf.Text("value")}
	text.Flags = FieldComb // without MaxLen, which Encode refuses

	choice := NewChoiceField("c")
	choice.DefaultAppearance = testDA
	choice.Opt = []ChoiceOption{{Export: pdf.Text("x"), Display: pdf.Text("x")}}
	choice.TopIndex = 5 // out of range, which Encode refuses
	choice.AA = &triggers.Form{Calculate: js}
	choice.V = []pdf.TextString{pdf.Text("x")}
	choice.Selected = []int{0}

	button := NewButtonField("b")
	button.AA = &triggers.Form{Calculate: js}
	button.V = "Off"

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
