// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package acroform

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
)

type pair struct{ Name, Value string }

func collect(form *InteractiveForm) []pair {
	var res []pair
	for name, value := range form.ExportData() {
		res = append(res, pair{name, value})
	}
	return res
}

func TestExportData(t *testing.T) {
	text := NewTextField("given")
	text.Text = &pdf.StringOrStream{Value: pdf.Text("Ada")}

	empty := NewTextField("blank")
	empty.Text = &pdf.StringOrStream{}

	noValue := NewTextField("unset")

	hidden := NewTextField("secret")
	hidden.Flags = FieldNoExport
	hidden.Text = &pdf.StringOrStream{Value: pdf.Text("x")}

	mapped := NewTextField("family")
	mapped.ExportName = "surname"
	mapped.Text = &pdf.StringOrStream{Value: pdf.Text("Lovelace")}

	checked := NewButtonField("agree", ButtonCheckbox)
	checked.State = "Yes"

	unchecked := NewButtonField("optin", ButtonCheckbox)
	unchecked.State = "Off"

	push := NewPushButtonField("submit")

	// positional on-state names, with export values in ExportValues
	radio, _ := radioWith(0, "0", "1", "2")
	radio.Name = "size"
	radio.ExportValues = []string{"S", "M", "M"}
	radio.State = "2"

	// ExportValues too short for the button which is on
	short, _ := radioWith(0, "a", "b")
	short.Name = "short"
	short.ExportValues = []string{"A"}
	short.State = "b"

	choice := NewChoiceField("colours")
	choice.Flags = FieldMultiSelect
	choice.Options = []ChoiceOption{
		{Export: "r", Display: pdf.Text("Red")},
		{Export: "g", Display: pdf.Text("Green")},
	}
	choice.Value = []pdf.TextString{pdf.Text("Red"), pdf.Text("Green")}

	// two options share a display string; Selected picks the second
	dup := NewChoiceField("dup")
	dup.Options = []ChoiceOption{
		{Export: "1", Display: pdf.Text("Same")},
		{Export: "2", Display: pdf.Text("Same")},
	}
	dup.Value = []pdf.TextString{pdf.Text("Same")}
	dup.Selected = []int{1}

	combo := NewChoiceField("city")
	combo.Flags = FieldCombo | FieldEdit
	combo.Options = []ChoiceOption{{Export: "L", Display: pdf.Text("Leeds")}}
	combo.Value = []pdf.TextString{pdf.Text("York")}

	sig := NewSignatureField("sig")

	form := &InteractiveForm{Fields: []Node{
		text, empty, noValue, hidden, mapped,
		checked, unchecked, push, radio, short,
		&Group{Name: "g", Children: []Node{choice, dup}},
		combo, sig,
	}}

	want := []pair{
		{"given", "Ada"},
		{"blank", ""},
		{"surname", "Lovelace"},
		{"agree", "Yes"},
		{"size", "M"},
		{"short", "b"},
		{"g.colours", "r"},
		{"g.colours", "g"},
		{"g.dup", "2"},
		{"city", "York"},
	}
	if diff := cmp.Diff(want, collect(form)); diff != "" {
		t.Errorf("export data mismatch (-want +got):\n%s", diff)
	}
}

// fields sharing a fully qualified name are one field, exported once
func TestExportDataSharedNames(t *testing.T) {
	text1 := NewTextField("x")
	text1.Text = &pdf.StringOrStream{Value: pdf.Text("v")}
	text2 := NewTextField("x")
	text2.Text = &pdf.StringOrStream{Value: pdf.Text("v")}

	// the selected button belongs to the second field only
	radio1, _ := radioWith(0, "a", "b")
	radio1.Name = "r"
	radio1.ExportValues = []string{"A", "B"}
	radio1.State = "c"
	radio2, _ := radioWith(0, "c")
	radio2.Name = "r"
	radio2.ExportValues = []string{"C"}
	radio2.State = "c"

	// the export name comes from the second field
	named1 := NewTextField("n")
	named1.Text = &pdf.StringOrStream{Value: pdf.Text("1")}
	named2 := NewTextField("n")
	named2.ExportName = "mapped"
	named2.Text = &pdf.StringOrStream{Value: pdf.Text("1")}

	// one field with NoExport keeps the name out
	hidden1 := NewTextField("h")
	hidden1.Text = &pdf.StringOrStream{Value: pdf.Text("1")}
	hidden2 := NewTextField("h")
	hidden2.Flags = FieldNoExport
	hidden2.Text = &pdf.StringOrStream{Value: pdf.Text("1")}

	// the first field lists no options; the second maps the value
	combo1 := NewChoiceField("c")
	combo1.Flags = FieldCombo | FieldEdit
	combo1.Value = []pdf.TextString{pdf.Text("Red")}
	combo2 := NewChoiceField("c")
	combo2.Flags = FieldCombo | FieldEdit
	combo2.Options = []ChoiceOption{{Export: "r", Display: pdf.Text("Red")}}
	combo2.Value = []pdf.TextString{pdf.Text("Red")}

	form := &InteractiveForm{Fields: []Node{
		text1, radio1, named1, hidden1, combo1,
		&Group{Name: "g", Children: []Node{NewTextField("y")}},
		text2, radio2, named2, hidden2, combo2,
	}}

	want := []pair{
		{"x", "v"},
		{"r", "C"},
		{"mapped", "1"},
		{"c", "r"},
	}
	if diff := cmp.Diff(want, collect(form)); diff != "" {
		t.Errorf("export data mismatch (-want +got):\n%s", diff)
	}
}

func TestExportDataStop(t *testing.T) {
	a := NewTextField("a")
	a.Text = &pdf.StringOrStream{Value: pdf.Text("1")}
	b := NewTextField("b")
	b.Text = &pdf.StringOrStream{Value: pdf.Text("2")}
	form := &InteractiveForm{Fields: []Node{a, b}}

	n := 0
	for range form.ExportData() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("got %d pairs after break, want 1", n)
	}
}

// a field's ExportData reports its values even where FieldNoExport keeps them
// out of the form's ExportData
func TestExportDataIgnoresNoExport(t *testing.T) {
	f := NewTextField("secret")
	f.Flags = FieldNoExport
	f.Text = &pdf.StringOrStream{Value: pdf.Text("x")}
	if diff := cmp.Diff([]string{"x"}, f.ExportData()); diff != "" {
		t.Errorf("export values mismatch (-want +got):\n%s", diff)
	}
}

func TestExportDataMixedTypes(t *testing.T) {
	// the text fields share the names, but cannot map the values
	radio1, _ := radioWith(0, "a")
	radio1.State = "c"
	radio2, _ := radioWith(0, "c")
	radio2.ExportValues = []string{"C"}
	radio2.State = "c"

	combo1 := NewChoiceField("c")
	combo1.Flags = FieldCombo | FieldEdit
	combo1.Value = []pdf.TextString{pdf.Text("Red")}
	combo2 := NewChoiceField("c")
	combo2.Flags = FieldCombo | FieldEdit
	combo2.Options = []ChoiceOption{{Export: "r", Display: pdf.Text("Red")}}
	combo2.Value = []pdf.TextString{pdf.Text("Red")}

	form := &InteractiveForm{Fields: []Node{
		radio1, NewTextField("r"), radio2,
		combo1, NewTextField("c"), combo2,
	}}

	want := []pair{{"r", "C"}, {"c", "r"}}
	if diff := cmp.Diff(want, collect(form)); diff != "" {
		t.Errorf("export data mismatch (-want +got):\n%s", diff)
	}
}
