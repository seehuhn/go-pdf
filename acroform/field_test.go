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
	child.Parent = parent

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
	typedChild.Parent = parent
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
	child.Parent = &FieldCommon{Ff: optional.New(FieldRadio)}
	if got := child.Variant(); got != ButtonRadio {
		t.Errorf("inherited Variant() = %d, want ButtonRadio (%d)", got, ButtonRadio)
	}
}

func TestFieldFullyQualifiedName(t *testing.T) {
	root := &FieldCommon{T: "PersonalData"}
	mid := &FieldCommon{T: "Address"}
	mid.Parent = root
	leaf := &FieldCommon{T: "ZipCode"}
	leaf.Parent = mid

	if got := leaf.FullyQualifiedName(); got != "PersonalData.Address.ZipCode" {
		t.Errorf("FullyQualifiedName = %q, want %q", got, "PersonalData.Address.ZipCode")
	}

	// an ancestor without a partial name is skipped
	anon := &FieldCommon{}
	anon.Parent = root
	leaf2 := &FieldCommon{T: "Phone"}
	leaf2.Parent = anon
	if got := leaf2.FullyQualifiedName(); got != "PersonalData.Phone" {
		t.Errorf("FullyQualifiedName with anonymous ancestor = %q, want %q", got, "PersonalData.Phone")
	}
}

func TestEncodeFieldNameWithPeriod(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	field := &FieldTx{FieldCommon: FieldCommon{T: "a.b"}}
	if _, err := fieldEntries(rm, field); err == nil {
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
		{"FileSelect flag requires 1.4", pdf.V1_3, &FieldTx{FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldFileSelect)}}},
		{"DoNotSpellCheck flag requires 1.4", pdf.V1_3, &FieldTx{FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldDoNotSpellCheck)}}},
		{"DoNotScroll flag requires 1.4", pdf.V1_3, &FieldTx{FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldDoNotScroll)}}},
		{"MultiSelect flag requires 1.4", pdf.V1_3, &FieldChoice{FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldMultiSelect)}}},
		{"Comb flag requires 1.5", pdf.V1_4, &FieldTx{FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldComb)}, MaxLen: 6}},
		{"RichText flag requires 1.5", pdf.V1_4, &FieldTx{FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldRichText)}}},
		{"RadiosInUnison flag requires 1.5", pdf.V1_4, &FieldBtn{FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldRadio | FieldRadiosInUnison)}}},
		{"CommitOnSelChange flag requires 1.5", pdf.V1_4, &FieldChoice{FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldCommitOnSelChange)}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(tc.version, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := fieldEntries(rm, tc.field); !pdf.IsWrongVersion(err) {
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
		{"FileSelect flag at 1.4", pdf.V1_4, &FieldTx{FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldFileSelect)}}},
		{"MultiSelect flag at 1.4", pdf.V1_4, &FieldChoice{FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldMultiSelect)}}},
		{"Comb flag at 1.5", pdf.V1_5, &FieldTx{FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldComb)}, MaxLen: 6}},
		{"CommitOnSelChange flag at 1.5", pdf.V1_5, &FieldChoice{FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldCommitOnSelChange)}}},
	}
	for _, tc := range atVersion {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(tc.version, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := fieldEntries(rm, tc.field); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestEncodeCombValidation(t *testing.T) {
	comb := func(extra FieldFlags, maxLen int) *FieldTx {
		return &FieldTx{
			FieldCommon: FieldCommon{T: "x", Ff: optional.New(FieldComb | extra)},
			MaxLen:      maxLen,
		}
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
			_, err := fieldEntries(rm, tc.field)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}

	// MaxLen is inheritable: a comb child whose MaxLen sits on an ancestor
	// must encode
	parent := &FieldTx{FieldCommon: FieldCommon{T: "p"}, MaxLen: 6}
	child := &FieldTx{FieldCommon: FieldCommon{T: "c", Ff: optional.New(FieldComb)}}
	parent.Kids = []Node{child}
	child.Parent = parent

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	if _, err := fieldEntries(rm, child); err != nil {
		t.Errorf("comb child with inherited MaxLen: %v", err)
	}
}

func TestResolvedMaxLen(t *testing.T) {
	parent := &FieldTx{FieldCommon: FieldCommon{T: "p"}, MaxLen: 6}
	child := &FieldTx{FieldCommon: FieldCommon{T: "c"}}
	parent.Kids = []Node{child}
	child.Parent = parent

	if got := ResolvedMaxLen(child); got != 6 {
		t.Errorf("inherited ResolvedMaxLen = %d, want 6", got)
	}

	// a local value overrides the inherited one
	child.MaxLen = 4
	if got := ResolvedMaxLen(child); got != 4 {
		t.Errorf("own ResolvedMaxLen = %d, want 4", got)
	}

	// no text-field ancestor sets a maximum
	if got := ResolvedMaxLen(&FieldTx{}); got != 0 {
		t.Errorf("ResolvedMaxLen without MaxLen = %d, want 0", got)
	}
}

// stubWidget is a minimal Node standing in for a widget annotation.
type stubWidget struct{ parent Field }

func (s *stubWidget) FieldParent() Field { return s.parent }

func (s *stubWidget) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	return pdf.Dict{}, nil
}

// encoding never repairs the tree: inconsistent or missing parent links abort
// with an error
func TestEncodeParentValidation(t *testing.T) {
	newRM := func() *pdf.ResourceManager {
		w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		return pdf.NewResourceManager(w)
	}

	t.Run("kid with missing Parent link", func(t *testing.T) {
		form := &InteractiveForm{}
		parent := form.NewField("p")
		child := &FieldTx{FieldCommon: FieldCommon{T: "c"}}
		parent.Kids = append(parent.Kids, child)
		if _, err := form.Encode(newRM()); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("kid with wrong Parent link", func(t *testing.T) {
		form := &InteractiveForm{}
		parent := form.NewField("p")
		child := &FieldTx{FieldCommon: FieldCommon{T: "c"}}
		child.Parent = NewField(nil, "other")
		parent.Kids = append(parent.Kids, child)
		if _, err := form.Encode(newRM()); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("root field with Parent link", func(t *testing.T) {
		form := &InteractiveForm{}
		root := form.NewTextField("r")
		root.Parent = NewField(nil, "other")
		if _, err := form.Encode(newRM()); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("single widget not linked to its field", func(t *testing.T) {
		tx := NewTextField(nil, "t")
		tx.Kids = append(tx.Kids, &stubWidget{})
		form := &InteractiveForm{Fields: []Field{tx}}
		if _, err := form.Encode(newRM()); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("widget kid not linked, multi-widget field", func(t *testing.T) {
		tx := NewTextField(nil, "t")
		linked := &stubWidget{parent: tx}
		tx.Kids = append(tx.Kids, linked, &stubWidget{})
		form := &InteractiveForm{Fields: []Field{tx}}
		if _, err := form.Encode(newRM()); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("correctly linked tree encodes", func(t *testing.T) {
		form := &InteractiveForm{}
		parent := form.NewField("p")
		tx := NewTextField(parent, "c")
		w := &stubWidget{parent: tx}
		tx.Kids = append(tx.Kids, w)
		if _, err := form.Encode(newRM()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
