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
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func choiceWith(flags FieldFlags, v []pdf.TextString, selected []int, topIndex int) *ChoiceField {
	f := NewChoiceField("c")
	f.DefaultAppearance = testDA
	f.Flags = flags
	f.Options = []ChoiceOption{{Export: "a", Display: pdf.Text("A")}, {Export: "b", Display: pdf.Text("B")}, {Export: "c", Display: pdf.Text("C")}}
	f.Value = v
	f.Selected = selected
	f.TopIndex = topIndex
	return f
}

func withDV(f *ChoiceField, dv ...pdf.TextString) *ChoiceField {
	f.DefaultValue = dv
	return f
}

func TestEncodeChoiceValidation(t *testing.T) {
	tests := []struct {
		name    string
		field   *ChoiceField
		wantErr bool
	}{
		{"nothing selected", choiceWith(0, nil, nil, 0), false},
		{"consistent Value and I", choiceWith(0, []pdf.TextString{pdf.Text("B")}, []int{1}, 0), false},
		{"multi-select consistent", choiceWith(FieldMultiSelect, []pdf.TextString{pdf.Text("A"), pdf.Text("C")}, []int{0, 2}, 0), false},
		{"index out of range", choiceWith(0, nil, []int{3}, 0), true},
		{"negative index", choiceWith(0, nil, []int{-1}, 0), true},
		{"descending indices", choiceWith(FieldMultiSelect, nil, []int{2, 0}, 0), true},
		{"duplicate indices", choiceWith(FieldMultiSelect, nil, []int{1, 1}, 0), true},
		{"top index out of range", choiceWith(0, nil, nil, 3), true},
		{"top index last option", choiceWith(0, nil, nil, 2), false},
		{"negative top index", choiceWith(0, nil, nil, -1), true},
		{"two values without MultiSelect", choiceWith(0, []pdf.TextString{pdf.Text("A"), pdf.Text("B")}, nil, 0), true},
		{"two indices without MultiSelect", choiceWith(0, nil, []int{0, 1}, 0), true},
		{"two defaults without MultiSelect", withDV(choiceWith(0, nil, nil, 0), pdf.Text("A"), pdf.Text("B")), true},
		{"two defaults with MultiSelect", withDV(choiceWith(FieldMultiSelect, nil, nil, 0), pdf.Text("A"), pdf.Text("B")), false},
		{"Value disagrees with I", choiceWith(0, []pdf.TextString{pdf.Text("A")}, []int{1}, 0), true},
		{"Value in another order than I", choiceWith(FieldMultiSelect, []pdf.TextString{pdf.Text("C"), pdf.Text("A")}, []int{0, 2}, 0), false},
		{"Value repeats one option of I", choiceWith(FieldMultiSelect, []pdf.TextString{pdf.Text("A"), pdf.Text("A")}, []int{0, 2}, 0), true},
		{"Value not an option", choiceWith(0, []pdf.TextString{pdf.Text("Z")}, nil, 0), true},
		{"one of several values not an option", choiceWith(FieldMultiSelect, []pdf.TextString{pdf.Text("A"), pdf.Text("Z")}, nil, 0), true},
		{"default not an option", withDV(choiceWith(0, nil, nil, 0), pdf.Text("Z")), true},
		{"combo value not an option", choiceWith(FieldCombo, []pdf.TextString{pdf.Text("Z")}, nil, 0), true},
		{"editable combo free text", withDV(choiceWith(FieldCombo|FieldEdit, []pdf.TextString{pdf.Text("Z")}, nil, 0), pdf.Text("Y")), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			_, err := terminalEntries(rm, tc.field)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// a nil Value is derived from Selected on write
func TestEncodeChoiceDerivedValue(t *testing.T) {
	tests := []struct {
		name     string
		flags    FieldFlags
		selected []int
		want     pdf.Object
	}{
		{"single", 0, []int{1}, pdf.Text("B")},
		{"multiple", FieldMultiSelect, []int{0, 2}, pdf.Array{pdf.Text("A"), pdf.Text("C")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			dict, err := terminalEntries(rm, choiceWith(tc.flags, nil, tc.selected, 0))
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, dict["V"]); diff != "" {
				t.Errorf("Value mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// an option whose display string carries a language tag is written as an
// [export display] pair, so that the export value stays untagged
func TestChoiceOptionTaggedDisplay(t *testing.T) {
	f := NewChoiceField("c")
	f.DefaultAppearance = testDA
	f.Options = []ChoiceOption{
		{Export: "x", Display: pdf.Text("x")},
		{Export: "y", Display: pdf.Text(language.German, "y")},
	}
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	dict, err := terminalEntries(pdf.NewResourceManager(w), f)
	if err != nil {
		t.Fatal(err)
	}
	opt := dict["Opt"].(pdf.Array)
	if _, ok := opt[0].(pdf.Array); ok {
		t.Error("untagged option written as a pair")
	}
	if _, ok := opt[1].(pdf.Array); !ok {
		t.Error("tagged option not written as a pair")
	}
}
