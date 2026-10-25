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
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/pdf/optional"
)

func TestLangRoundTrip(t *testing.T) {
	for _, lang := range []language.Tag{language.Und, language.German} {
		roundTripValue(t, pdf.V2_0, &annotation.Text{
			Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 10, URy: 10},
			Lang: optional.New(lang),
			Icon: "Note",
		})
	}
}

func TestLangDecode(t *testing.T) {
	tests := []struct {
		name string
		lang pdf.Object
		want optional.Value[language.Tag]
	}{
		{"absent", nil, optional.Value[language.Tag]{}},
		{"empty", pdf.Text(""), optional.New(language.Und)},
		{"und", pdf.Text("und"), optional.New(language.Und)},
		{"tag", pdf.Text("de-CH"), optional.New(language.MustParse("de-CH"))},
		{"unknown language", pdf.Text("qq"), optional.Value[language.Tag]{}},
		{"malformed", pdf.Text("en--US"), optional.Value[language.Tag]{}},
		{"wrong type", pdf.Integer(1), optional.Value[language.Tag]{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dict := pdf.Dict{
				"Subtype": pdf.Name("Text"),
				"Rect":    pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)},
			}
			if tt.lang != nil {
				dict["Lang"] = tt.lang
			}
			a, err := Annotation(pdf.CursorAt(pdf.NewExtractor(mock.Getter), nil), dict, false)
			if err != nil {
				t.Fatal(err)
			}
			if got := a.GetCommon().Lang; !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestInvalidEntryIsAbsent checks that a reference to a nonexistent object,
// and a value of the wrong type, are read like an absent entry.
func TestInvalidEntryIsAbsent(t *testing.T) {
	invalid := map[string]pdf.Object{
		"dangling": pdf.NewReference(99, 0),
		"name":     pdf.Name("x"),
		"integer":  pdf.Integer(5),
	}
	// boolean entries; an integer is the wrong type for these
	boolKeys := map[pdf.Name]bool{"3DI": true, "TB": true, "NP": true}
	rect := pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)}

	cases := []struct {
		name string
		base pdf.Dict
		key  pdf.Name
		// sub, if set, names the sub-dictionary of base which holds key
		sub pdf.Name
	}{
		{"StructParent", pdf.Dict{"Subtype": pdf.Name("Text")}, "StructParent", ""},
		{"CA", pdf.Dict{"Subtype": pdf.Name("Text")}, "CA", ""},
		{"ca", pdf.Dict{"Subtype": pdf.Name("Text"), "CA": pdf.Real(0.25)}, "ca", ""},
		{"Lang", pdf.Dict{"Subtype": pdf.Name("Text")}, "Lang", ""},
		{"3DI", pdf.Dict{"Subtype": pdf.Name("3D"), "3DD": pdf.NewReference(98, 0)}, "3DI", ""},
		{"3DA", pdf.Dict{"Subtype": pdf.Name("3D"), "3DD": pdf.NewReference(98, 0)}, "3DA", ""},
		{
			"3DA TB",
			pdf.Dict{"Subtype": pdf.Name("3D"), "3DD": pdf.NewReference(98, 0), "3DA": pdf.Dict{"A": pdf.Name("PO")}},
			"TB", "3DA",
		},
		{
			"3DA NP",
			pdf.Dict{"Subtype": pdf.Name("3D"), "3DD": pdf.NewReference(98, 0), "3DA": pdf.Dict{"A": pdf.Name("PO")}},
			"NP", "3DA",
		},
	}
	for _, tc := range cases {
		for variant, value := range invalid {
			if variant == "integer" && !boolKeys[tc.key] {
				continue
			}
			t.Run(tc.name+" "+variant, func(t *testing.T) {
				c := pdf.NewCursor(mock.Getter)

				absent := maps.Clone(tc.base)
				absent["Rect"] = rect
				withInvalid := maps.Clone(absent)
				if tc.sub != "" {
					inner := maps.Clone(absent[tc.sub].(pdf.Dict))
					inner[tc.key] = value
					withInvalid[tc.sub] = inner
				} else {
					withInvalid[tc.key] = value
				}

				want, err := Annotation(c, absent, false)
				if err != nil {
					t.Fatal(err)
				}
				got, err := Annotation(c, withInvalid, false)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("%s value read differently from absent entry (-absent +got):\n%s", variant, diff)
				}
			})
		}
	}
}

// TestAnnot3DActivationDefaults checks the defaults for absent entries of the
// 3D activation dictionary.
func TestAnnot3DActivationDefaults(t *testing.T) {
	dict := pdf.Dict{
		"Subtype": pdf.Name("3D"),
		"Rect":    pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)},
		"3DD":     pdf.NewReference(98, 0),
		"3DA":     pdf.Dict{"A": pdf.Name("PO")},
	}
	a, err := Annotation(pdf.NewCursor(mock.Getter), dict, false)
	if err != nil {
		t.Fatal(err)
	}
	act := a.(*annotation.Annot3D).A
	if act == nil || !act.TB || act.NP || !a.(*annotation.Annot3D).I {
		t.Errorf("wrong defaults: %+v", act)
	}
}
