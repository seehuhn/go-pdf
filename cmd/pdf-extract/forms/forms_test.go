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

package forms_test

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/cmd/pdf-extract/forms"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// dump writes form to an in-memory PDF, reads it back, and returns the output
// of forms.List. A nil form leaves the catalog without an AcroForm entry.
func dump(t *testing.T, form *acroform.InteractiveForm) string {
	t.Helper()

	w, buf := memfile.NewPDFWriter(pdf.V2_0, nil)
	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatalf("add blank page: %v", err)
	}

	rm := pdf.NewResourceManager(w)
	if form != nil {
		ref, err := rm.Store(form)
		if err != nil {
			t.Fatalf("encode form: %v", err)
		}
		w.GetMeta().Catalog.AcroForm = ref
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("resource manager close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer close: %v", err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Data), int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatalf("open document: %v", err)
	}
	t.Cleanup(func() { r.Close() })

	var out bytes.Buffer
	if err := forms.List(r, &out); err != nil {
		t.Fatalf("list form: %v", err)
	}
	return out.String()
}

func textField(name, value string) *acroform.TextField {
	f := acroform.NewTextField(name)
	f.V = &pdf.StringOrStream{Value: value}
	return f
}

func formWith(roots ...acroform.Node) *acroform.InteractiveForm {
	return &acroform.InteractiveForm{Fields: roots}
}

func TestListValues(t *testing.T) {
	checkbox := acroform.NewButtonField("agree")
	checkbox.V = "Yes"

	unchecked := acroform.NewButtonField("optin")

	push := acroform.NewButtonField("submit")
	push.Flags = acroform.FieldPushbutton

	choice := acroform.NewChoiceField("colors")
	choice.V = []string{"Red", "Green"}

	sig := acroform.NewSignatureField("signature1")

	tests := []struct {
		name string
		form *acroform.InteractiveForm
		want string
	}{
		{"text", formWith(textField("given_name", "Ada")), "given_name: Ada\n"},
		{"checkbox on", formWith(checkbox), "agree: Yes\n"},
		{"checkbox off", formWith(unchecked), "optin: Off\n"},
		{"push button", formWith(push), "submit: \n"},
		{"choice multi", formWith(choice), "colors: Red, Green\n"},
		{"signature", formWith(sig), "signature1: <unsigned>\n"},
		{"multiline", formWith(textField("notes", "a\nb")), "notes: a b\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dump(t, tt.form); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListNames(t *testing.T) {
	both := textField("given_name", "Ada")
	both.AltName = "Given name"
	both.ExportName = "fname"

	altOnly := textField("given_name", "Ada")
	altOnly.AltName = "Given name"

	exportOnly := textField("given_name", "Ada")
	exportOnly.ExportName = "fname"

	tests := []struct {
		name string
		form *acroform.InteractiveForm
		want string
	}{
		{"both names", formWith(both), "given_name \"Given name\" [fname]: Ada\n"},
		{"alt only", formWith(altOnly), "given_name \"Given name\": Ada\n"},
		{"export only", formWith(exportOnly), "given_name [fname]: Ada\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dump(t, tt.form); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListNestedGroup(t *testing.T) {
	group := &acroform.Group{
		Name:     "address",
		Children: []acroform.Node{textField("street", "Main St")},
	}
	if got, want := dump(t, formWith(group)), "address.street: Main St\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestListNoForm(t *testing.T) {
	if got, want := dump(t, nil), "No interactive form in document.\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestListEmptyForm(t *testing.T) {
	if got, want := dump(t, formWith()), "Interactive form contains no fields.\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
