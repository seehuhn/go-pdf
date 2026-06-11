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
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// testVersions lists the PDF versions exercised by the round-trip unit tests and
// the fuzz seed corpora.
var testVersions = []pdf.Version{pdf.V1_7, pdf.V2_0}

// formTestCases holds representative interactive forms. The Fields and
// CalculationOrder entries reference field dictionaries that are not part of
// the InteractiveForm object itself; the round trip only needs to preserve the
// reference values, so fixed references are used rather than allocating real
// objects.
// coField0 and coField1 are shared between a form's Fields and its
// CalculationOrder so the round trip exercises the same field appearing in both.
var (
	coField0 = &acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "calc0"}}
	coField1 = &acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "calc1"}}
)

var formTestCases = []struct {
	name string
	form *acroform.InteractiveForm
}{
	{
		name: "minimal",
		form: &acroform.InteractiveForm{
			Fields: []acroform.Field{&acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "f0"}}},
		},
	},
	{
		name: "flags and text defaults",
		form: &acroform.InteractiveForm{
			Fields:            []acroform.Field{&acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "f0"}}, &acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "f1"}}},
			NeedAppearances:   true,
			SigFlags:          acroform.SignaturesExist | acroform.AppendOnly,
			DefaultAppearance: "/Helv 0 Tf 0 g",
			Align:             pdf.TextAlignCenter,
		},
	},
	{
		name: "calculation order",
		form: &acroform.InteractiveForm{
			Fields:           []acroform.Field{coField0, coField1},
			CalculationOrder: []acroform.Field{coField1, coField0},
			Align:            pdf.TextAlignRight,
		},
	},
	{
		name: "xfa",
		form: &acroform.InteractiveForm{
			Fields: []acroform.Field{&acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "f0"}}},
			XFA:    pdf.Array{pdf.String("template"), pdf.String("<xdp/>")},
		},
	},
	{
		name: "default resources",
		form: &acroform.InteractiveForm{
			Fields:           []acroform.Field{&acroform.FieldTx{FieldCommon: acroform.FieldCommon{T: "f0"}}},
			DefaultResources: &content.Resources{SingleUse: true},
		},
	},
}

func formRoundTripTest(t *testing.T, version pdf.Version, want *acroform.InteractiveForm) {
	t.Helper()

	w, buf := memfile.NewPDFWriter(version, nil)

	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatalf("add blank page: %v", err)
	}

	rm := pdf.NewResourceManager(w)
	ref, err := rm.Store(want)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("encode failed: %v", err)
	}
	w.GetMeta().Catalog.AcroForm = ref

	if err := rm.Close(); err != nil {
		t.Fatalf("resource manager close failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer close failed: %v", err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Data), int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatalf("open document: %v", err)
	}
	defer r.Close()

	x := pdf.NewExtractor(r)
	got, err := pdf.ExtractorGet(x, nil, r.GetMeta().Catalog.AcroForm, Form)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if diff := cmp.Diff(want, got, fieldCmpOptions()...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestFormRoundTrip(t *testing.T) {
	for _, version := range testVersions {
		for _, tc := range formTestCases {
			t.Run(tc.name+"-"+version.String(), func(t *testing.T) {
				formRoundTripTest(t, version, tc.form)
			})
		}
	}
}

func FuzzFormRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, version := range testVersions {
		for _, tc := range formTestCases {
			w, buf := memfile.NewPDFWriter(version, opt)

			if err := memfile.AddBlankPage(w); err != nil {
				continue
			}

			rm := pdf.NewResourceManager(w)
			ref, err := rm.Store(tc.form)
			if err != nil {
				continue
			}
			w.GetMeta().Catalog.AcroForm = ref

			if err := rm.Close(); err != nil {
				continue
			}
			if err := w.Close(); err != nil {
				continue
			}

			f.Add(buf.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		defer r.Close()

		x := pdf.NewExtractor(r)
		form, err := pdf.ExtractorGet(x, nil, r.GetMeta().Catalog.AcroForm, Form)
		if err != nil {
			t.Skip("malformed interactive form")
		}
		if form == nil {
			t.Skip("no interactive form")
		}

		formRoundTripTest(t, pdf.GetVersion(r), form)
	})
}

func TestDecodeValues(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"FT": pdf.Name("Tx"), "T": pdf.TextString("f0")}); err != nil {
		t.Fatal(err)
	}
	dict := pdf.Dict{
		// the second element is not a reference and must be skipped
		"Fields":          pdf.Array{ref, pdf.Integer(7)},
		"NeedAppearances": pdf.Boolean(true),
		"SigFlags":        pdf.Integer(3),
		"DA":              pdf.String("/Helv 0 Tf"),
		"Q":               pdf.Integer(99), // out of range, must be ignored
	}

	form, err := Form(x, nil, dict, true)
	if err != nil {
		t.Fatal(err)
	}

	if len(form.Fields) != 1 || form.Fields[0].GetFieldCommon().T != "f0" {
		t.Errorf("Fields = %v, want one field named f0", form.Fields)
	}
	if !form.NeedAppearances {
		t.Error("NeedAppearances = false, want true")
	}
	if form.SigFlags != acroform.SignaturesExist|acroform.AppendOnly {
		t.Errorf("SigFlags = %d, want %d", form.SigFlags, acroform.SignaturesExist|acroform.AppendOnly)
	}
	if form.DefaultAppearance != "/Helv 0 Tf" {
		t.Errorf("DefaultAppearance = %q", form.DefaultAppearance)
	}
	if form.Align != pdf.TextAlignLeft {
		t.Errorf("Align = %d, want %d (out-of-range value ignored)", form.Align, pdf.TextAlignLeft)
	}
}

func TestDecodeNil(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	form, err := Form(x, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if form != nil {
		t.Errorf("expected nil form for nil object, got %v", form)
	}
}
