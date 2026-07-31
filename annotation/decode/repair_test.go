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
	"seehuhn.de/go/pdf/internal/debug/mock"
)

// TestMarkupDecodeRepair verifies that decodeMarkup snaps RT values the
// encoder would reject to the default.
func TestMarkupDecodeRepair(t *testing.T) {
	x := pdf.NewExtractor(mock.Getter)

	tests := []struct {
		name string
		dict pdf.Dict
		want pdf.Name
	}{
		{"invalid RT", pdf.Dict{"RT": pdf.Name("Bogus"), "IRT": pdf.NewReference(7, 0)}, ""},
		{"RT without IRT", pdf.Dict{"RT": pdf.Name("Group")}, ""},
		{"valid RT", pdf.Dict{"RT": pdf.Name("Group"), "IRT": pdf.NewReference(7, 0)}, "Group"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var m annotation.Markup
			if err := decodeMarkup(pdf.CursorAt(x, nil), tc.dict, &m); err != nil {
				t.Fatal(err)
			}
			if m.RT != tc.want {
				t.Errorf("RT: got %q, want %q", m.RT, tc.want)
			}
		})
	}
}

// TestFreeTextDecodeRepair verifies that decodeFreeText repairs entries the
// encoder would reject: a missing DA and an invalid intent.
func TestFreeTextDecodeRepair(t *testing.T) {
	x := pdf.NewExtractor(mock.Getter)
	dict := pdf.Dict{
		"Subtype": pdf.Name("FreeText"),
		"Rect":    &pdf.Rectangle{URx: 100, URy: 50},
		"IT":      pdf.Name("Bogus"),
	}
	ft, err := decodeFreeText(pdf.CursorAt(x, nil), dict)
	if err != nil {
		t.Fatal(err)
	}
	if ft.DefaultAppearance == "" {
		t.Error("expected a default appearance string")
	}
	if ft.Intent != "" {
		t.Errorf("expected invalid intent to be cleared, got %q", ft.Intent)
	}

	// the repaired annotation must encode without error
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	if _, err := ft.Encode(rm); err != nil {
		t.Errorf("encoding repaired free text annotation: %v", err)
	}
}

// TestMissingAppearanceStateRepair verifies that an annotation whose
// appearance dictionary holds a stream per state but names no state is given
// one, so that it can be written back.  With nothing else to go on the state
// chosen is the smallest name, taken from the normal appearance.
func TestMissingAppearanceStateRepair(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rect := pdf.Rectangle{URx: 100, URy: 50}
	dict := pdf.Dict{
		"Subtype": pdf.Name("Square"),
		"Rect":    &rect,
		"AP": pdf.Dict{
			"N": pdf.Dict{"On": appearanceStream(t, w, rect), "Off": appearanceStream(t, w, rect)},
		},
		// no AS entry
	}

	x := pdf.NewExtractor(w)
	a, err := Annotation(pdf.CursorAt(x, nil), dict, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.GetCommon().AppearanceState; got != "Off" {
		t.Errorf("appearance state = %q, want %q", got, "Off")
	}

	rm := pdf.NewResourceManager(w)
	if _, err := a.(pdf.Encoder).Encode(rm); err != nil {
		t.Errorf("encoding repaired annotation: %v", err)
	}
}

// TestMissingAppearanceStateFromFieldValue verifies that a check box or radio
// button whose file names no appearance state takes one from the field value
// rather than from the appearance dictionary alone.  Picking the smallest name
// instead would show a check box which is on as unchecked.
func TestMissingAppearanceStateFromFieldValue(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rect := pdf.Rectangle{URx: 20, URy: 20}
	apRef := w.Alloc()
	err := w.Put(apRef, pdf.Dict{"N": pdf.Dict{
		"Off": appearanceStream(t, w, rect),
		"Yes": appearanceStream(t, w, rect),
	}})
	if err != nil {
		t.Fatal(err)
	}

	// a field merged with its single widget carries the value itself
	merged := pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"), "Rect": &rect,
		"FT": pdf.Name("Btn"), "T": pdf.TextString("cb"), "V": pdf.Name("Yes"),
		"AP": apRef,
	}

	// a widget kid takes the value from its parent field
	fieldRef, kidRef := w.Alloc(), w.Alloc()
	err = w.Put(fieldRef, pdf.Dict{
		"FT": pdf.Name("Btn"), "T": pdf.TextString("cb"), "V": pdf.Name("Yes"),
		"Kids": pdf.Array{kidRef},
	})
	if err != nil {
		t.Fatal(err)
	}
	kid := pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"), "Rect": &rect,
		"Parent": fieldRef, "AP": apRef,
	}

	// a radio button which is not the selected one
	otherRadio := pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"), "Rect": &rect,
		"FT": pdf.Name("Btn"), "Ff": pdf.Integer(acroform.FieldRadio),
		"T": pdf.TextString("r"), "V": pdf.Name("Other"), "AP": apRef,
	}

	// a push button holds no value, so there is no state to take from it
	pushButton := pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Widget"), "Rect": &rect,
		"FT": pdf.Name("Btn"), "Ff": pdf.Integer(acroform.FieldPushbutton),
		"T": pdf.TextString("p"), "AP": apRef,
	}

	cases := []struct {
		name string
		dict pdf.Dict
		want pdf.Name
	}{
		{"merged", merged, "Yes"},
		{"kid", kid, "Yes"},
		{"unselectedRadio", otherRadio, "Off"},
		{"pushButton", pushButton, "Off"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref := w.Alloc()
			if err := w.Put(ref, tc.dict); err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(w)
			a, err := pdf.Decode(pdf.CursorAt(x, nil), ref, Annotation)
			if err != nil {
				t.Fatal(err)
			}
			if got := a.GetCommon().AppearanceState; got != tc.want {
				t.Errorf("appearance state = %q, want %q", got, tc.want)
			}
		})
	}
}

// appearanceStream writes an empty form XObject usable as an appearance
// stream and returns a reference to it.
func appearanceStream(t *testing.T, w *pdf.Writer, bbox pdf.Rectangle) pdf.Reference {
	t.Helper()

	ref := w.Alloc()
	stm, err := w.OpenStream(ref, pdf.Dict{
		"Subtype":   pdf.Name("Form"),
		"BBox":      &bbox,
		"Resources": pdf.Dict{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	return ref
}

// TestStampEncodeDefaultIntent verifies that a stamp annotation without an IT
// entry can be written back at PDF versions before 2.0, even though decoding
// normalises the absent intent to the default.
func TestStampEncodeDefaultIntent(t *testing.T) {
	x := pdf.NewExtractor(mock.Getter)
	dict := pdf.Dict{
		"Subtype": pdf.Name("Stamp"),
		"Rect":    &pdf.Rectangle{URx: 100, URy: 50},
	}
	a, err := Annotation(pdf.CursorAt(x, nil), dict, false)
	if err != nil {
		t.Fatal(err)
	}
	stamp, ok := a.(*annotation.Stamp)
	if !ok {
		t.Fatalf("expected *annotation.Stamp, got %T", a)
	}
	if stamp.Intent != annotation.StampIntentStamp {
		t.Errorf("expected default intent, got %q", stamp.Intent)
	}

	w, _ := memfile.NewPDFWriter(pdf.V1_3, nil)
	rm := pdf.NewResourceManager(w)
	out, err := stamp.Encode(rm)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.(pdf.Dict)["IT"]; ok {
		t.Error("expected no IT entry for the default intent")
	}
}
