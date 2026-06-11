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
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/optional"
)

func TestFieldResolvedAttributes(t *testing.T) {
	parent := &FieldTx{
		FieldCommon: FieldCommon{Ff: optional.New(FieldRequired)},
		V:           pdf.String("pv"),
		DV:          pdf.String("dv"),
	}
	child := &FieldCommon{T: "c"}
	child.parent = parent

	if got := ResolvedFT(child); got != "Tx" {
		t.Errorf("ResolvedFT = %q, want %q", got, "Tx")
	}
	if got := ResolvedFf(child); got != FieldRequired {
		t.Errorf("ResolvedFf = %d, want %d", got, FieldRequired)
	}
	if got, ok := ResolvedV(child).(pdf.String); !ok || string(got) != "pv" {
		t.Errorf("ResolvedV = %v, want %q", ResolvedV(child), "pv")
	}
	if got, ok := ResolvedDV(child).(pdf.String); !ok || string(got) != "dv" {
		t.Errorf("ResolvedDV = %v, want %q", ResolvedDV(child), "dv")
	}

	// a local value overrides the inherited one
	typedChild := &FieldBtn{FieldCommon: FieldCommon{T: "c"}}
	typedChild.parent = parent
	if got := ResolvedFT(typedChild); got != "Btn" {
		t.Errorf("ResolvedFT after override = %q, want %q", got, "Btn")
	}

	// an explicit zero blocks inheritance of the ancestor's flags
	child.Ff = optional.New(FieldFlags(0))
	if got := ResolvedFf(child); got != 0 {
		t.Errorf("ResolvedFf with explicit zero = %d, want 0", got)
	}
}

func TestFieldBtnVariant(t *testing.T) {
	for _, tt := range []struct {
		name string
		ff   FieldFlags
		want ButtonVariant
	}{
		{"checkbox", 0, ButtonCheckbox},
		{"radio", FieldRadio, ButtonRadio},
		{"push", FieldPushbutton, ButtonPush},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &FieldBtn{}
			if tt.ff != 0 {
				f.Ff = optional.New(tt.ff)
			}
			if got := f.Variant(); got != tt.want {
				t.Errorf("Variant() = %d, want %d", got, tt.want)
			}
		})
	}

	// the variant flag may be inherited: a button sub-field with no flags of its
	// own takes the Radio flag from its parent
	child := &FieldBtn{}
	child.parent = &FieldCommon{Ff: optional.New(FieldRadio)}
	if got := child.Variant(); got != ButtonRadio {
		t.Errorf("inherited Variant() = %d, want ButtonRadio (%d)", got, ButtonRadio)
	}
}

func TestFieldFullyQualifiedName(t *testing.T) {
	root := &FieldCommon{T: "PersonalData"}
	mid := &FieldCommon{T: "Address"}
	mid.parent = root
	leaf := &FieldCommon{T: "ZipCode"}
	leaf.parent = mid

	if got := leaf.FullyQualifiedName(); got != "PersonalData.Address.ZipCode" {
		t.Errorf("FullyQualifiedName = %q, want %q", got, "PersonalData.Address.ZipCode")
	}

	// an ancestor without a partial name is skipped
	anon := &FieldCommon{}
	anon.parent = root
	leaf2 := &FieldCommon{T: "Phone"}
	leaf2.parent = anon
	if got := leaf2.FullyQualifiedName(); got != "PersonalData.Phone" {
		t.Errorf("FullyQualifiedName with anonymous ancestor = %q, want %q", got, "PersonalData.Phone")
	}
}

func TestEncodeFieldNameWithPeriod(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	field := &FieldTx{FieldCommon: FieldCommon{T: "a.b"}}
	if _, err := FieldEntries(rm, field); err == nil {
		t.Error("expected error for partial name containing a period, got nil")
	}
}

func TestEncodeFieldVersionGating(t *testing.T) {
	tests := []struct {
		name    string
		version pdf.Version
		field   Field
	}{
		{"field requires 1.2", pdf.V1_1, &FieldTx{FieldCommon: FieldCommon{T: "x"}}},
		{"TU requires 1.3", pdf.V1_2, &FieldTx{FieldCommon: FieldCommon{T: "x", TU: "label"}}},
		{"TM requires 1.3", pdf.V1_2, &FieldTx{FieldCommon: FieldCommon{T: "x", TM: "map"}}},
		{"AA requires 1.3", pdf.V1_2, &FieldTx{
			FieldCommon: FieldCommon{
				T:  "x",
				AA: &triggers.Form{Calculate: &action.JavaScript{JS: pdf.String("0;")}},
			},
		}},
		{"signature field requires 1.3", pdf.V1_2, &FieldSig{FieldCommon: FieldCommon{T: "x"}}},
		{"button Opt requires 1.4", pdf.V1_3, &FieldBtn{FieldCommon: FieldCommon{T: "x"}, Opt: []string{"A"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(tc.version, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := FieldEntries(rm, tc.field); !pdf.IsWrongVersion(err) {
				t.Errorf("expected version error, got %v", err)
			}
		})
	}
}
