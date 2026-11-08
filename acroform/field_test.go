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

func TestButtonFieldVariant(t *testing.T) {
	for _, tt := range []struct {
		name string
		ff   FieldFlags
		want ButtonVariant
	}{
		{"checkbox", 0, ButtonCheckbox},
		{"radio", FieldRadio, ButtonRadio},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &ButtonField{}
			f.Flags = tt.ff
			if got := f.Variant(); got != tt.want {
				t.Errorf("Variant() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestAllFields(t *testing.T) {
	leaf := textField("ZipCode")
	form := &InteractiveForm{
		Fields: []Node{
			&Group{Name: "PersonalData", Children: []Node{
				&Group{Name: "Address", Children: []Node{leaf}},
				// an anonymous group contributes no name component
				&Group{Children: []Node{textField("Phone")}},
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
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	field := textField("a.b")
	if _, err := terminalEntries(rm, field); err == nil {
		t.Error("expected error for partial name containing a period, got nil")
	}
}

func TestEncodeFieldVersionGating(t *testing.T) {
	tx := func(setup func(*TextField)) *TextField {
		f := textField("x")
		setup(f)
		return f
	}
	tests := []struct {
		name    string
		version pdf.Version
		field   Field
	}{
		{"field requires 1.2", pdf.V1_1, textField("x")},
		{"TU requires 1.3", pdf.V1_2, tx(func(f *TextField) { f.AltName = pdf.Text("label") })},
		{"TM requires 1.3", pdf.V1_2, tx(func(f *TextField) { f.ExportName = "map" })},
		{"AA requires 1.3", pdf.V1_2, tx(func(f *TextField) {
			f.AA = &triggers.Form{Calculate: &action.JavaScript{JS: &pdf.StringOrStream{Value: pdf.Text("0;")}}}
		})},
		{"signature field requires 1.3", pdf.V1_2, NewSignatureField("x")},
		{"button Opt requires 1.4", pdf.V1_3, func() Field {
			f := NewButtonField("x", ButtonCheckbox)
			f.ExportValues = []string{"A"}
			f.Widgets = []Widget{&fakeWidget{f: f}}
			return f
		}()},
		{"FileSelect flag requires 1.4", pdf.V1_3, tx(func(f *TextField) { f.Flags = FieldFileSelect })},
		{"DoNotSpellCheck flag requires 1.4", pdf.V1_3, tx(func(f *TextField) { f.Flags = FieldDoNotSpellCheck })},
		{"DoNotScroll flag requires 1.4", pdf.V1_3, tx(func(f *TextField) { f.Flags = FieldDoNotScroll })},
		{"MultiSelect flag requires 1.4", pdf.V1_3, func() Field {
			f := choiceField("x")
			f.Flags = FieldMultiSelect
			return f
		}()},
		{"Comb flag requires 1.5", pdf.V1_4, tx(func(f *TextField) { f.Flags = FieldComb; f.MaxLen = 6 })},
		{"RichText flag requires 1.5", pdf.V1_4, tx(func(f *TextField) { f.Flags = FieldRichText })},
		{"RadiosInUnison flag requires 1.5", pdf.V1_4, func() Field {
			f := NewButtonField("x", ButtonRadio)
			f.Flags |= FieldRadiosInUnison
			return f
		}()},
		{"CommitOnSelChange flag requires 1.5", pdf.V1_4, func() Field {
			f := choiceField("x")
			f.Flags = FieldCommitOnSelChange
			return f
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, tc.version, nil)
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
		{"FileSelect flag at 1.4", pdf.V1_4, tx(func(f *TextField) { f.Flags = FieldFileSelect })},
		{"Comb flag at 1.5", pdf.V1_5, tx(func(f *TextField) { f.Flags = FieldComb; f.MaxLen = 6 })},
		{"CommitOnSelChange flag at 1.5", pdf.V1_5, func() Field {
			f := choiceField("x")
			f.Flags = FieldCommitOnSelChange
			return f
		}()},
	}
	for _, tc := range atVersion {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, tc.version, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := terminalEntries(rm, tc.field); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestEncodeCombValidation(t *testing.T) {
	comb := func(extra FieldFlags, maxLen int) *TextField {
		f := textField("x")
		f.Flags = FieldComb | extra
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

func TestEncodeFieldValidation(t *testing.T) {
	password := func(v, dv *pdf.StringOrStream) *TextField {
		f := textField("pw")
		f.Flags = FieldPassword
		f.Text, f.DefaultText = v, dv
		return f
	}
	value := &pdf.StringOrStream{Value: pdf.Text("secret")}
	richPassword := password(nil, nil)
	richPassword.RichText = value
	lines := func(flags FieldFlags, v, dv string) *TextField {
		f := textField("t")
		f.Flags = flags
		if v != "" {
			f.Text = &pdf.StringOrStream{Value: pdf.Text(v)}
		}
		if dv != "" {
			f.DefaultText = &pdf.StringOrStream{Value: pdf.Text(dv)}
		}
		return f
	}
	tests := []struct {
		name    string
		field   Field
		wantErr bool
	}{
		{"text field", textField("x"), false},
		{"unnamed field", textField(""), true},
		{"text field without DA", NewTextField("x"), true},
		{"choice field without DA", NewChoiceField("x"), true},
		{"button field without DA", NewButtonField("x", ButtonCheckbox), false},
		{"password without value", password(nil, nil), false},
		{"password with default value", password(nil, value), true},
		{"password with value", password(value, nil), false},
		{"password with rich value", richPassword, false},
		{"single-line text with LF", lines(0, "a\nb", ""), true},
		{"single-line text with CR", lines(0, "a\rb", ""), true},
		{"single-line default with LF", lines(0, "", "a\nb"), true},
		{"multiline text with LF", lines(FieldMultiline, "a\nb", "c\r\nd"), false},
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

// Fields sharing a fully qualified name must agree on type and value.
func TestEncodeDuplicateNames(t *testing.T) {
	text := func(v string) *TextField {
		f := textField("b")
		f.Text = &pdf.StringOrStream{Value: pdf.Text(v)}
		return f
	}
	tests := []struct {
		name    string
		a, b    Field
		wantErr bool
	}{
		{"same value", text("x"), text("x"), false},
		{"different value", text("x"), text("y"), true},
		{"different type", text("x"), choiceField("b"), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			rm := pdf.NewResourceManager(w)
			form := &InteractiveForm{Fields: []Node{
				&Group{Name: "a", Children: []Node{tc.a}},
				// an unnamed group contributes nothing to the name
				&Group{Children: []Node{&Group{Name: "a", Children: []Node{tc.b}}}},
			}}
			_, err := form.Encode(rm)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// the field constructors set the partial name and the field type
func TestConstructors(t *testing.T) {
	tests := []struct {
		field Field
		want  pdf.Name
	}{
		{NewTextField("a"), "Tx"},
		{NewButtonField("b", ButtonCheckbox), "Btn"},
		{NewChoiceField("c"), "Ch"},
		{NewSignatureField("d"), "Sig"},
	}
	for _, tc := range tests {
		if got := tc.field.FieldType(); got != tc.want {
			t.Errorf("FieldType = %q, want %q", got, tc.want)
		}
		if got := tc.field.PartialName(); got == "" {
			t.Errorf("%s field has no partial name", tc.want)
		}
	}
}

// the Pushbutton flag must agree with the field type
func TestEncodePushButtonFlag(t *testing.T) {
	check := NewButtonField("b", ButtonCheckbox)
	check.Flags = FieldPushbutton
	unflagged := NewPushButtonField("p")
	unflagged.Flags = 0
	tests := []struct {
		name    string
		field   Field
		wantErr bool
	}{
		{"push button", NewPushButtonField("p"), false},
		{"push button without flag", unflagged, true},
		{"check box with flag", check, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
			dict, err := terminalEntries(pdf.NewResourceManager(w), tc.field)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, want error %t", err, tc.wantErr)
			}
			if err == nil && dict["Ff"] != pdf.Integer(FieldPushbutton) {
				t.Errorf("/Ff = %v, want %d", dict["Ff"], FieldPushbutton)
			}
		})
	}
}

// a state or default state naming no on-state of any field of the same name
// cannot be written
func TestEncodeButtonStateUnknown(t *testing.T) {
	for _, set := range []func(*ButtonField){
		func(f *ButtonField) { f.State = "C" },
		func(f *ButtonField) { f.DefaultState = "C" },
	} {
		f, _ := radioWith(0, "A", "B")
		set(f)
		form := &InteractiveForm{Fields: []Node{f}}
		if err := form.checkButtonStates(); err == nil {
			t.Errorf("expected an error for State %q, DefaultState %q", f.State, f.DefaultState)
		}
	}
}

// fields of the same name are one field: a value may name an on-state which
// only one of them has
func TestCheckButtonStatesSameName(t *testing.T) {
	first, _ := radioWith(0, "Card", "Cash")
	second, _ := radioWith(0, "Card", "Cheque")
	first.Name, second.Name = "pay", "pay"
	first.State, second.State = "Cash", "Cash"
	form := &InteractiveForm{Fields: []Node{first, second}}
	if err := form.checkButtonStates(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	second.Name = "other"
	if err := form.checkButtonStates(); err == nil {
		t.Error("expected an error for a state of another field")
	}
}

// an empty state is written as "Off", so same-named fields with "" and "Off"
// agree
func TestSameTypeAndValueOff(t *testing.T) {
	a := NewButtonField("c", ButtonCheckbox)
	b := NewButtonField("c", ButtonCheckbox)
	b.State, b.DefaultState = "Off", "Off"
	if !sameTypeAndValue(a, b) {
		t.Error("empty and Off states differ")
	}
	b.State = "Yes"
	if sameTypeAndValue(a, b) {
		t.Error("empty and Yes states agree")
	}
}

// a push button and a check box of the same name are not one field
func TestSameTypeAndValuePushButton(t *testing.T) {
	push := NewPushButtonField("b")
	check := NewButtonField("b", ButtonCheckbox)
	if sameTypeAndValue(push, check) || sameTypeAndValue(check, push) {
		t.Error("push button and check box agree")
	}
	if !sameTypeAndValue(push, NewPushButtonField("b")) {
		t.Error("two push buttons differ")
	}
}
