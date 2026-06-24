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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// encodeForm encodes a form to a fresh in-memory writer and returns the writer
// (also a Getter) and the form dictionary.
func encodeForm(t *testing.T, form *InteractiveForm) (*pdf.Writer, pdf.Dict) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	native, err := form.Encode(rm)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	dict, ok := native.(pdf.Dict)
	if !ok {
		t.Fatalf("form did not encode to a dictionary")
	}
	return w, dict
}

// rootDicts reads the dictionaries named by the form's /Fields array.
func rootDicts(t *testing.T, w *pdf.Writer, form pdf.Dict) []pdf.Dict {
	t.Helper()
	arr, ok := form["Fields"].(pdf.Array)
	if !ok {
		t.Fatalf("/Fields is not an array")
	}
	dicts := make([]pdf.Dict, len(arr))
	for i, ref := range arr {
		d, err := pdf.NewCursor(w).Dict(ref)
		if err != nil {
			t.Fatalf("read field %d: %v", i, err)
		}
		dicts[i] = d
	}
	return dicts
}

func kidDicts(t *testing.T, w *pdf.Writer, group pdf.Dict) []pdf.Dict {
	t.Helper()
	arr, ok := group["Kids"].(pdf.Array)
	if !ok {
		t.Fatalf("/Kids is not an array")
	}
	dicts := make([]pdf.Dict, len(arr))
	for i, ref := range arr {
		d, err := pdf.NewCursor(w).Dict(ref)
		if err != nil {
			t.Fatalf("read kid %d: %v", i, err)
		}
		dicts[i] = d
	}
	return dicts
}

// a common field type and default appearance shared by a group's children are
// hoisted into the group; the children no longer carry them
func TestFactorHoistUnanimous(t *testing.T) {
	mk := func(name, da string) *TextField {
		f := NewTextField(name)
		f.DefaultAppearance = da
		return f
	}
	form := &InteractiveForm{
		Fields: []Node{
			&Group{Name: "g", Children: []Node{
				mk("a", "/Helv 12 Tf"),
				mk("b", "/Helv 12 Tf"),
			}},
		},
	}
	w, dict := encodeForm(t, form)
	groups := rootDicts(t, w, dict)
	group := groups[0]

	if group["FT"] != pdf.Name("Tx") {
		t.Errorf("group FT = %v, want Tx", group["FT"])
	}
	if s, _ := pdf.NewCursor(w).String(group["DA"]); string(s) != "/Helv 12 Tf" {
		t.Errorf("group DA = %v, want hoisted value", group["DA"])
	}
	for i, kid := range kidDicts(t, w, group) {
		if _, ok := kid["FT"]; ok {
			t.Errorf("kid %d still carries FT", i)
		}
		if _, ok := kid["DA"]; ok {
			t.Errorf("kid %d still carries DA", i)
		}
	}
}

// the document-wide DA shared by all roots is hoisted into the form dictionary
func TestFactorHoistFormDA(t *testing.T) {
	mk := func(name string) *TextField {
		f := NewTextField(name)
		f.DefaultAppearance = "/Helv 0 Tf"
		return f
	}
	form := &InteractiveForm{Fields: []Node{mk("a"), mk("b")}}
	w, dict := encodeForm(t, form)

	if s, _ := pdf.NewCursor(w).String(dict["DA"]); string(s) != "/Helv 0 Tf" {
		t.Errorf("form DA = %v, want hoisted value", dict["DA"])
	}
	for i, root := range rootDicts(t, w, dict) {
		if _, ok := root["DA"]; ok {
			t.Errorf("root %d still carries DA", i)
		}
	}
}

// V and DV are never hoisted, even when every child shares them
func TestFactorNeverHoistValue(t *testing.T) {
	mk := func(name string) *TextField {
		f := NewTextField(name)
		f.V = &pdf.StringOrStream{Value: "same"}
		return f
	}
	form := &InteractiveForm{
		Fields: []Node{
			&Group{Name: "g", Children: []Node{mk("a"), mk("b")}},
		},
	}
	w, dict := encodeForm(t, form)
	group := rootDicts(t, w, dict)[0]

	if _, ok := group["V"]; ok {
		t.Error("V was hoisted into the group")
	}
	for i, kid := range kidDicts(t, w, group) {
		if _, ok := kid["V"]; !ok {
			t.Errorf("kid %d lost its V", i)
		}
	}
}

// an explicit zero flag survives hoisting as an override when a non-zero flag is
// hoisted into the group
func TestFactorFlagsOverride(t *testing.T) {
	a := NewTextField("a")
	a.Flags = FieldReadOnly
	b := NewTextField("b")
	b.Flags = FieldReadOnly
	c := NewTextField("c") // Ff == 0, must keep inheriting 0
	form := &InteractiveForm{
		Fields: []Node{
			&Group{Name: "g", Children: []Node{a, b, c}},
		},
	}
	w, dict := encodeForm(t, form)
	group := rootDicts(t, w, dict)[0]

	if ff, _ := pdf.NewCursor(w).Integer(group["Ff"]); ff != pdf.Integer(FieldReadOnly) {
		t.Errorf("group Ff = %v, want %d", group["Ff"], FieldReadOnly)
	}
	kids := kidDicts(t, w, group)
	// a and b inherit; c must carry an explicit 0 to override the inherited flag
	if _, ok := kids[2]["Ff"]; !ok {
		t.Error("the zero-flag child did not get an explicit override")
	}
}

func TestEncodeEmptyGroup(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	form := &InteractiveForm{Fields: []Node{&Group{Name: "g"}}}
	if _, err := form.Encode(rm); err == nil {
		t.Error("expected error for empty group, got nil")
	}
}

func TestEncodeDuplicateNode(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	f := NewTextField("f")
	form := &InteractiveForm{Fields: []Node{f, f}}
	if _, err := form.Encode(rm); err == nil {
		t.Error("expected error for a field used twice, got nil")
	}
}

func TestEncodeCalculationOrderNotInTree(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	form := &InteractiveForm{
		Fields:           []Node{NewTextField("a")},
		CalculationOrder: []Field{NewTextField("orphan")},
	}
	if _, err := form.Encode(rm); err == nil {
		t.Error("expected error for CO field not in the tree, got nil")
	}
}
