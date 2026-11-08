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
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/cmd/pdf-extract/forms"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// dump writes form to an in-memory PDF, reads it back, and returns the output
// of forms.List. A nil form leaves the catalog without an AcroForm entry.
func dump(t *testing.T, form *acroform.InteractiveForm) string {
	t.Helper()

	w, buf := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	rm := pdf.NewResourceManager(w)
	if form != nil {
		// writing the widgets first reserves the references the form fills in
		for _, field := range form.AllFields() {
			for _, wg := range field.GetCommon().Widgets {
				if _, err := rm.Store(wg); err != nil {
					t.Fatalf("store widget: %v", err)
				}
			}
		}
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
	f.DefaultAppearance = "/Helv 0 Tf 0 g"
	f.Text = &pdf.StringOrStream{Value: pdf.Text(value)}
	return f
}

func formWith(roots ...acroform.Node) *acroform.InteractiveForm {
	return &acroform.InteractiveForm{Fields: roots}
}

func TestListValues(t *testing.T) {
	checkbox := acroform.NewButtonField("agree", acroform.ButtonCheckbox)
	checkbox.State = "Yes"

	radio := acroform.NewButtonField("size", acroform.ButtonRadio)
	radio.ExportValues = []string{"small", "large"}
	radio.State = "1"
	// single-point widgets need no appearance
	for i := range radio.ExportValues {
		x := float64(20 * i)
		annotation.AddWidget(radio, pdf.Rectangle{LLx: x, URx: x})
	}

	choice := acroform.NewChoiceField("colors")
	choice.DefaultAppearance = "/Helv 0 Tf 0 g"
	choice.Flags = acroform.FieldMultiSelect
	choice.Options = []acroform.ChoiceOption{
		{Export: "r", Display: pdf.Text("Red")},
		{Export: "g", Display: pdf.Text("Green")},
	}
	choice.Value = []pdf.TextString{pdf.Text("Red"), pdf.Text("Green")}

	notes := textField("notes", "a\nb")
	notes.Flags = acroform.FieldMultiline

	mapped := textField("given_name", "Ada")
	mapped.ExportName = "fname"

	tests := []struct {
		name string
		form *acroform.InteractiveForm
		want string
	}{
		{"text", formWith(textField("given_name", "Ada")), "given_name: Ada\n"},
		{"export name", formWith(mapped), "fname: Ada\n"},
		{"checkbox", formWith(checkbox), "agree: Yes\n"},
		{"radio", formWith(radio), "size: large\n"},
		{"choice multi", formWith(choice), "colors: r\ncolors: g\n"},
		{"multiline", formWith(notes), "notes: a b\n"},
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

func TestListNoData(t *testing.T) {
	unchecked := acroform.NewButtonField("optin", acroform.ButtonCheckbox)
	push := acroform.NewPushButtonField("submit")
	sig := acroform.NewSignatureField("signature1")
	form := formWith(unchecked, push, sig)
	if got, want := dump(t, form), "Interactive form exports no data.\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
