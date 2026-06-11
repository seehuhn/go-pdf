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
	"maps"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

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
			f.Ff = tt.ff
			if got := f.Variant(); got != tt.want {
				t.Errorf("Variant() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestAllFields(t *testing.T) {
	leaf := NewTextField("ZipCode")
	form := &InteractiveForm{
		Fields: []TreeNode{
			&Group{Name: "PersonalData", Kids: []TreeNode{
				&Group{Name: "Address", Kids: []TreeNode{leaf}},
				// an anonymous group contributes no name component
				&Group{Kids: []TreeNode{NewTextField("Phone")}},
			}},
		},
	}

	got := maps.Collect(form.AllFields())

	if got["PersonalData.Address.ZipCode"] != leaf {
		t.Errorf("ZipCode field not found under its fully qualified name; got %v", got)
	}
	if _, ok := got["PersonalData.Phone"]; !ok {
		t.Errorf("Phone field not found under PersonalData.Phone; got %v", got)
	}
	if len(got) != 2 {
		t.Errorf("AllFields yielded %d fields, want 2", len(got))
	}
}

func TestEncodeFieldNameWithPeriod(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	field := NewTextField("a.b")
	if _, err := terminalEntries(rm, field); err == nil {
		t.Error("expected error for partial name containing a period, got nil")
	}
}

func TestEncodeFieldVersionGating(t *testing.T) {
	tx := func(setup func(*FieldTx)) *FieldTx {
		f := NewTextField("x")
		setup(f)
		return f
	}
	tests := []struct {
		name    string
		version pdf.Version
		field   Field
	}{
		{"field requires 1.2", pdf.V1_1, NewTextField("x")},
		{"TU requires 1.3", pdf.V1_2, tx(func(f *FieldTx) { f.TU = "label" })},
		{"TM requires 1.3", pdf.V1_2, tx(func(f *FieldTx) { f.TM = "map" })},
		{"AA requires 1.3", pdf.V1_2, tx(func(f *FieldTx) {
			f.AA = &triggers.Form{Calculate: &action.JavaScript{JS: pdf.String("0;")}}
		})},
		{"signature field requires 1.3", pdf.V1_2, NewSignatureField("x")},
		{"button Opt requires 1.4", pdf.V1_3, func() Field {
			f := NewButtonField("x")
			f.Opt = []string{"A"}
			return f
		}()},
		{"FileSelect flag requires 1.4", pdf.V1_3, tx(func(f *FieldTx) { f.Ff = FieldFileSelect })},
		{"DoNotSpellCheck flag requires 1.4", pdf.V1_3, tx(func(f *FieldTx) { f.Ff = FieldDoNotSpellCheck })},
		{"DoNotScroll flag requires 1.4", pdf.V1_3, tx(func(f *FieldTx) { f.Ff = FieldDoNotScroll })},
		{"MultiSelect flag requires 1.4", pdf.V1_3, func() Field {
			f := NewChoiceField("x")
			f.Ff = FieldMultiSelect
			return f
		}()},
		{"Comb flag requires 1.5", pdf.V1_4, tx(func(f *FieldTx) { f.Ff = FieldComb; f.MaxLen = 6 })},
		{"RichText flag requires 1.5", pdf.V1_4, tx(func(f *FieldTx) { f.Ff = FieldRichText })},
		{"RadiosInUnison flag requires 1.5", pdf.V1_4, func() Field {
			f := NewButtonField("x")
			f.Ff = FieldRadio | FieldRadiosInUnison
			return f
		}()},
		{"CommitOnSelChange flag requires 1.5", pdf.V1_4, func() Field {
			f := NewChoiceField("x")
			f.Ff = FieldCommitOnSelChange
			return f
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(tc.version, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := terminalEntries(rm, tc.field); !pdf.IsWrongVersion(err) {
				t.Errorf("expected version error, got %v", err)
			}
		})
	}

	// the same fields encode without error at the required version
	atVersion := []struct {
		name    string
		version pdf.Version
		field   Field
	}{
		{"FileSelect flag at 1.4", pdf.V1_4, tx(func(f *FieldTx) { f.Ff = FieldFileSelect })},
		{"Comb flag at 1.5", pdf.V1_5, tx(func(f *FieldTx) { f.Ff = FieldComb; f.MaxLen = 6 })},
		{"CommitOnSelChange flag at 1.5", pdf.V1_5, func() Field {
			f := NewChoiceField("x")
			f.Ff = FieldCommitOnSelChange
			return f
		}()},
	}
	for _, tc := range atVersion {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(tc.version, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := terminalEntries(rm, tc.field); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestEncodeCombValidation(t *testing.T) {
	comb := func(extra FieldFlags, maxLen int) *FieldTx {
		f := NewTextField("x")
		f.Ff = FieldComb | extra
		f.MaxLen = maxLen
		return f
	}
	tests := []struct {
		name    string
		field   Field
		wantErr bool
	}{
		{"with MaxLen", comb(0, 6), false},
		{"missing MaxLen", comb(0, 0), true},
		{"with Multiline", comb(FieldMultiline, 6), true},
		{"with Password", comb(FieldPassword, 6), true},
		{"with FileSelect", comb(FieldFileSelect, 6), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
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
