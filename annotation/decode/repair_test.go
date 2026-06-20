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
