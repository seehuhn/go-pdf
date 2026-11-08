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
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func buttonFillForm(t *testing.T) *acroform.InteractiveForm {
	blank := &form.Form{BBox: pdf.Rectangle{URx: 10, URy: 10}, Res: &content.Resources{}}
	radio := acroform.NewButtonField("r", acroform.ButtonRadio)
	for i, on := range []pdf.Name{"A", "A", "B"} {
		w := annotation.AddWidget(radio, pdf.Rectangle{LLx: float64(20 * i), URx: float64(20*i + 10), URy: 10})
		w.Appearance = &appearance.Dict{
			NormalMap: map[pdf.Name]*form.Form{on: blank, "Off": blank},
		}
		w.AppearanceState = "Off"
	}
	radio.State = "Off"
	return roundTripForm(t, pdf.V1_7, &acroform.InteractiveForm{Fields: []acroform.Node{radio}})
}

func TestButtonFillRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		apply func(*acroform.ButtonField) error
		wantV pdf.Name
		want  []pdf.Name
	}{
		{"toggle", func(f *acroform.ButtonField) error { return acroform.NewLogicalField("", f).Toggle(f.Widgets[1]) }, "A", []pdf.Name{"Off", "A", "Off"}},
		{"set value", func(f *acroform.ButtonField) error { return acroform.NewLogicalField("", f).SetState("B") }, "B", []pdf.Name{"Off", "Off", "B"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first := buttonFillForm(t)
			field := first.Fields[0].(*acroform.ButtonField)
			if err := tc.apply(field); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, appearanceStates(field)); diff != "" {
				t.Fatalf("setter states differ (-want +got):\n%s", diff)
			}

			second := roundTripForm(t, pdf.V1_7, first)
			got := second.Fields[0].(*acroform.ButtonField)
			if got.State != tc.wantV {
				t.Errorf("V = %q, want %q", got.State, tc.wantV)
			}
			if diff := cmp.Diff(tc.want, appearanceStates(got)); diff != "" {
				t.Errorf("appearance states differ (-want +got):\n%s", diff)
			}
		})
	}
}

func appearanceStates(f *acroform.ButtonField) []pdf.Name {
	var out []pdf.Name
	for _, w := range f.Widgets {
		out = append(out, w.CurrentAppearanceState())
	}
	return out
}

func TestTextChoiceFillRoundTrip(t *testing.T) {
	text := func(s string) func(*acroform.TextField) error {
		return func(f *acroform.TextField) error { return acroform.NewLogicalField("", f).SetText(s) }
	}
	for _, s := range []string{"Zoë 日本語", ""} {
		t.Run("text "+s, func(t *testing.T) {
			f := newTextField("t")
			f.Flags |= acroform.FieldMultiline
			f.Text = &pdf.StringOrStream{Value: pdf.Text("old")}
			first := roundTripForm(t, pdf.V1_7, &acroform.InteractiveForm{Fields: []acroform.Node{f}})
			field := first.Fields[0].(*acroform.TextField)
			if err := text(s)(field); err != nil {
				t.Fatal(err)
			}
			second := roundTripForm(t, pdf.V1_7, first)
			got := second.Fields[0].(*acroform.TextField)
			if got.Text == nil {
				t.Fatal("V lost")
			}
			if diff := cmp.Diff(field.Text, got.Text); diff != "" {
				t.Errorf("V differs (-want +got):\n%s", diff)
			}
		})
	}

	choices := []struct {
		name  string
		flags acroform.FieldFlags
		apply func(*acroform.ChoiceField) error
	}{
		{"selection", acroform.FieldMultiSelect, func(f *acroform.ChoiceField) error { return acroform.NewLogicalField("", f).Select(f, []int{2, 0}) }},
		{"no selection", acroform.FieldMultiSelect, func(f *acroform.ChoiceField) error { return acroform.NewLogicalField("", f).Select(f, nil) }},
		{"no selection single", 0, func(f *acroform.ChoiceField) error { return acroform.NewLogicalField("", f).Select(f, nil) }},
		{"edit text", acroform.FieldCombo | acroform.FieldEdit, func(f *acroform.ChoiceField) error { return acroform.NewLogicalField("", f).SetEditText("free") }},
	}
	for _, tc := range choices {
		t.Run(tc.name, func(t *testing.T) {
			f := newChoiceField("c")
			f.Flags |= tc.flags
			for _, s := range []string{"A", "B", "C"} {
				f.Options = append(f.Options, acroform.ChoiceOption{Export: s, Display: pdf.Text(s)})
			}
			f.Value = []pdf.TextString{pdf.Text("B")}
			f.Selected = []int{1}
			first := roundTripForm(t, pdf.V1_7, &acroform.InteractiveForm{Fields: []acroform.Node{f}})
			field := first.Fields[0].(*acroform.ChoiceField)
			if err := tc.apply(field); err != nil {
				t.Fatal(err)
			}
			second := roundTripForm(t, pdf.V1_7, first)
			got := second.Fields[0].(*acroform.ChoiceField)
			if (got.Value == nil) != (field.Value == nil) {
				t.Errorf("V nil-ness lost: got %#v, want %#v", got.Value, field.Value)
			}
			if diff := cmp.Diff(field.Value, got.Value); diff != "" {
				t.Errorf("V differs (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(field.Selected, got.Selected); diff != "" {
				t.Errorf("Selected differs (-want +got):\n%s", diff)
			}
		})
	}
}

// valueGoverned lists the field keys which ValueEntries governs.
var valueGoverned = []pdf.Name{"V", "I", "RV"}

// rawFile describes a file by name: build returns the objects to write, with
// ref giving the reference of a named object.
type rawFile func(ref func(string) pdf.Reference) map[string]pdf.Dict

// writeRaw writes the objects of build to a fresh file.  Before an object is
// written, patch (if not nil) may modify it.  The returned function gives the
// reference of a named object.
func writeRaw(t *testing.T, build rawFile, patch func(w *pdf.Writer, name string, d pdf.Dict)) (*pdf.Writer, func(string) pdf.Reference) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	refs := map[string]pdf.Reference{}
	ref := func(name string) pdf.Reference {
		r, ok := refs[name]
		if !ok {
			r = w.Alloc()
			refs[name] = r
		}
		return r
	}
	for name, d := range build(ref) {
		if patch != nil {
			patch(w, name, d)
		}
		if err := w.Put(ref(name), d); err != nil {
			t.Fatal(err)
		}
	}
	return w, ref
}

// patchValues returns a patch function which rewrites the dictionary named
// name: the keys returned by ValueEntries are set, other governed keys are
// deleted.  Further patches may be chained via also.
func patchValues(t *testing.T, name string, f acroform.Field, also func(name string, d pdf.Dict)) func(*pdf.Writer, string, pdf.Dict) {
	return func(w *pdf.Writer, n string, d pdf.Dict) {
		if also != nil {
			also(n, d)
		}
		if n != name {
			return
		}
		entries, err := acroform.ValueEntries(pdf.NewResourceManager(w), f)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range valueGoverned {
			delete(d, key)
		}
		maps.Copy(d, entries)
	}
}

// readFillForm decodes the form whose root fields are refs, through a fresh
// extractor.
func readFillForm(t *testing.T, w *pdf.Writer, refs ...pdf.Reference) *acroform.InteractiveForm {
	t.Helper()
	var fields pdf.Array
	for _, r := range refs {
		fields = append(fields, r)
	}
	x := pdf.NewExtractor(w)
	idx, err := readForm(pdf.CursorAt(x, nil), pdf.Dict{"Fields": fields}, true)
	if err != nil {
		t.Fatal(err)
	}
	return idx.Form
}

func widgetDict(extra pdf.Dict) pdf.Dict {
	d := pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"),
		"Rect": &pdf.Rectangle{URx: 50, URy: 20},
		"AP":   pdf.Dict{"N": pdf.Dict{"Off": pdf.Dict{}}},
	}
	maps.Copy(d, extra)
	return d
}

func TestValueEntriesPatchText(t *testing.T) {
	build := func(ref func(string) pdf.Reference) map[string]pdf.Dict {
		return map[string]pdf.Dict{"t": widgetDict(pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.Text("t"), "DA": pdf.String("/Helv 0 Tf"),
			"V": pdf.Text("old"),
		})}
	}
	w, ref := writeRaw(t, build, nil)
	field := readFillForm(t, w, ref("t")).Fields[0].(*acroform.TextField)
	if err := acroform.NewLogicalField("", field).SetText("Zoë new"); err != nil {
		t.Fatal(err)
	}

	w2, ref2 := writeRaw(t, build, patchValues(t, "t", field, nil))
	got := readFillForm(t, w2, ref2("t")).Fields[0].(*acroform.TextField)
	if diff := cmp.Diff(field.Text, got.Text); diff != "" {
		t.Errorf("V differs (-want +got):\n%s", diff)
	}
}

func TestValueEntriesPatchInheritedV(t *testing.T) {
	build := func(ref func(string) pdf.Reference) map[string]pdf.Dict {
		objs := map[string]pdf.Dict{
			"g": {
				"T": pdf.Text("g"), "FT": pdf.Name("Tx"), "V": pdf.Text("old"),
				"Kids": pdf.Array{ref("a"), ref("b")},
			},
		}
		for _, name := range []string{"a", "b"} {
			objs[name] = pdf.Dict{
				"T": pdf.Text(name), "DA": pdf.String("/Helv 0 Tf"),
				"Parent": ref("g"), "Kids": pdf.Array{ref("w" + name)},
			}
			objs["w"+name] = widgetDict(pdf.Dict{"Parent": ref(name)})
		}
		return objs
	}
	w, ref := writeRaw(t, build, nil)
	group := readFillForm(t, w, ref("g")).Fields[0].(*acroform.Group)
	a := group.Children[0].(*acroform.TextField)
	if a.Text == nil || a.Text.Value != pdf.Text("old") {
		t.Fatalf("a does not inherit V: %#v", a.Text)
	}
	if err := acroform.NewLogicalField("", a).SetText(""); err != nil {
		t.Fatal(err)
	}

	w2, ref2 := writeRaw(t, build, patchValues(t, "a", a, nil))
	got := readFillForm(t, w2, ref2("g")).Fields[0].(*acroform.Group)
	ga := got.Children[0].(*acroform.TextField)
	gb := got.Children[1].(*acroform.TextField)
	if ga.Text == nil || ga.Text.Value != pdf.Text("") {
		t.Errorf("a: V = %#v, want empty value", ga.Text)
	}
	if gb.Text == nil || gb.Text.Value != pdf.Text("old") {
		t.Errorf("b: V = %#v, want old", gb.Text)
	}
}

func TestValueEntriesPatchChoiceNone(t *testing.T) {
	build := func(ref func(string) pdf.Reference) map[string]pdf.Dict {
		return map[string]pdf.Dict{
			"g": {
				"T": pdf.Text("g"), "FT": pdf.Name("Ch"), "V": pdf.Text("x"),
				"Kids": pdf.Array{ref("c")},
			},
			"c": {
				"T": pdf.Text("c"), "Parent": ref("g"), "Kids": pdf.Array{ref("w")},
				"Ff":  pdf.Integer(acroform.FieldMultiSelect),
				"Opt": pdf.Array{pdf.Text("x"), pdf.Text("y")},
				"DA":  pdf.String("/Helv 0 Tf"),
			},
			"w": widgetDict(pdf.Dict{"Parent": ref("c")}),
		}
	}
	w, ref := writeRaw(t, build, nil)
	group := readFillForm(t, w, ref("g")).Fields[0].(*acroform.Group)
	c := group.Children[0].(*acroform.ChoiceField)
	if len(c.Value) != 1 {
		t.Fatalf("c does not inherit V: %#v", c.Value)
	}
	if err := acroform.NewLogicalField("", c).Select(c, nil); err != nil {
		t.Fatal(err)
	}

	w2, ref2 := writeRaw(t, build, patchValues(t, "c", c, nil))
	got := readFillForm(t, w2, ref2("g")).Fields[0].(*acroform.Group)
	gc := got.Children[0].(*acroform.ChoiceField)
	if len(gc.Value) != 0 || len(gc.Selected) != 0 {
		t.Errorf("selection not cleared: Value=%#v Selected=%#v", gc.Value, gc.Selected)
	}
}

func TestValueEntriesPatchButton(t *testing.T) {
	build := func(ref func(string) pdf.Reference) map[string]pdf.Dict {
		objs := map[string]pdf.Dict{
			"f": {
				"FT": pdf.Name("Btn"), "T": pdf.Text("r"), "Ff": pdf.Integer(acroform.FieldRadio),
				"V": pdf.Name("A"), "Kids": pdf.Array{ref("w0"), ref("w1")},
			},
		}
		for i, on := range []pdf.Name{"A", "B"} {
			as := pdf.Name("Off")
			if i == 0 {
				as = on
			}
			objs[fmt.Sprintf("w%d", i)] = widgetDict(pdf.Dict{
				"Parent": ref("f"), "AS": as,
				"AP": pdf.Dict{"N": pdf.Dict{on: pdf.Dict{}, "Off": pdf.Dict{}}},
			})
		}
		return objs
	}
	w, ref := writeRaw(t, build, nil)
	field := readFillForm(t, w, ref("f")).Fields[0].(*acroform.ButtonField)
	if field.State != "A" {
		t.Fatalf("V = %q, want A", field.State)
	}
	if err := acroform.NewLogicalField("", field).SetState("Off"); err != nil {
		t.Fatal(err)
	}

	setAS := func(name string, d pdf.Dict) {
		for i, w := range field.Widgets {
			if name == fmt.Sprintf("w%d", i) {
				d["AS"] = w.CurrentAppearanceState()
			}
		}
	}
	w2, ref2 := writeRaw(t, build, patchValues(t, "f", field, setAS))

	// the reader would take Off from the appearance states alone, so the
	// stored dictionary is checked as well
	raw, err := w2.Get(ref2("f"), true)
	if err != nil {
		t.Fatal(err)
	}
	if v := raw.(pdf.Dict)["V"]; v != pdf.Name("Off") {
		t.Errorf("stored /V = %v, want /Off", v)
	}

	got := readFillForm(t, w2, ref2("f")).Fields[0].(*acroform.ButtonField)
	if got.State != "Off" {
		t.Errorf("V = %q, want Off", got.State)
	}
	if diff := cmp.Diff([]pdf.Name{"Off", "Off"}, appearanceStates(got)); diff != "" {
		t.Errorf("appearance states differ (-want +got):\n%s", diff)
	}
}

// TestValueEntriesPatchChoiceStaleI checks that patching a single-select
// choice field removes a selection list which no longer applies.
func TestValueEntriesPatchChoiceStaleI(t *testing.T) {
	build := func(ref func(string) pdf.Reference) map[string]pdf.Dict {
		return map[string]pdf.Dict{"c": widgetDict(pdf.Dict{
			"FT": pdf.Name("Ch"), "T": pdf.Text("c"), "DA": pdf.String("/Helv 0 Tf"),
			"Ff":  pdf.Integer(acroform.FieldCombo | acroform.FieldEdit),
			"Opt": pdf.Array{pdf.Text("x"), pdf.Text("y")},
			"V":   pdf.Text("x"), "I": pdf.Array{pdf.Integer(0)},
		})}
	}
	w, ref := writeRaw(t, build, nil)
	field := readFillForm(t, w, ref("c")).Fields[0].(*acroform.ChoiceField)
	if !slices.Equal(field.Selected, []int{0}) {
		t.Fatalf("Selected = %v, want [0]", field.Selected)
	}
	if err := acroform.NewLogicalField("", field).SetEditText("custom"); err != nil {
		t.Fatal(err)
	}

	w2, ref2 := writeRaw(t, build, patchValues(t, "c", field, nil))
	raw, err := w2.Get(ref2("c"), true)
	if err != nil {
		t.Fatal(err)
	}
	if i, ok := raw.(pdf.Dict)["I"]; ok {
		t.Errorf("stale /I %v kept", i)
	}
	got := readFillForm(t, w2, ref2("c")).Fields[0].(*acroform.ChoiceField)
	if len(got.Value) != 1 || got.Value[0] != pdf.Text("custom") || got.Selected != nil {
		t.Errorf("Value = %v, Selected = %v, want [custom] and none", got.Value, got.Selected)
	}
}

func TestValueEntriesRichTextDeleted(t *testing.T) {
	build := func(ref func(string) pdf.Reference) map[string]pdf.Dict {
		return map[string]pdf.Dict{"t": widgetDict(pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.Text("t"), "DA": pdf.String("/Helv 0 Tf"),
			"Ff": pdf.Integer(acroform.FieldRichText),
			"V":  pdf.Text("old"), "RV": pdf.Text("<body><p>old</p></body>"),
		})}
	}
	w, ref := writeRaw(t, build, nil)
	field := readFillForm(t, w, ref("t")).Fields[0].(*acroform.TextField)
	if field.RichText == nil {
		t.Fatal("RichText not read")
	}
	if err := acroform.NewLogicalField("", field).SetText("new"); err != nil {
		t.Fatal(err)
	}

	w2, ref2 := writeRaw(t, build, patchValues(t, "t", field, nil))
	got := readFillForm(t, w2, ref2("t")).Fields[0].(*acroform.TextField)
	if got.RichText != nil {
		t.Errorf("RichText = %#v, want nil", got.RichText)
	}
	if got.Text == nil || got.Text.Value != pdf.Text("new") {
		t.Errorf("V = %#v, want new", got.Text)
	}
}
