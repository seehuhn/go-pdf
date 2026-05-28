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
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// testVersions lists the PDF versions exercised by the round-trip unit test and
// the fuzz seed corpus.
var testVersions = []pdf.Version{pdf.V1_7, pdf.V2_0}

// testCases holds representative interactive forms. The Fields and
// CalculationOrder entries reference field dictionaries that are not part of
// the InteractiveForm object itself; the round trip only needs to preserve the
// reference values, so fixed references are used rather than allocating real
// objects.
var testCases = []struct {
	name string
	form *InteractiveForm
}{
	{
		name: "minimal",
		form: &InteractiveForm{
			Fields: []pdf.Reference{pdf.NewReference(100, 0)},
		},
	},
	{
		name: "flags and text defaults",
		form: &InteractiveForm{
			Fields:            []pdf.Reference{pdf.NewReference(100, 0), pdf.NewReference(101, 0)},
			NeedAppearances:   true,
			SigFlags:          SignaturesExist | AppendOnly,
			DefaultAppearance: "/Helv 0 Tf 0 g",
			Align:             pdf.TextAlignCenter,
		},
	},
	{
		name: "calculation order",
		form: &InteractiveForm{
			Fields:           []pdf.Reference{pdf.NewReference(100, 0), pdf.NewReference(101, 0)},
			CalculationOrder: []pdf.Reference{pdf.NewReference(101, 0), pdf.NewReference(100, 0)},
			Align:            pdf.TextAlignRight,
		},
	},
	{
		name: "xfa",
		form: &InteractiveForm{
			Fields: []pdf.Reference{pdf.NewReference(100, 0)},
			XFA:    pdf.Array{pdf.String("template"), pdf.String("<xdp/>")},
		},
	},
	{
		name: "default resources",
		form: &InteractiveForm{
			Fields:           []pdf.Reference{pdf.NewReference(100, 0)},
			DefaultResources: &content.Resources{SingleUse: true},
		},
	},
}

func roundTripTest(t *testing.T, version pdf.Version, want *InteractiveForm) {
	t.Helper()

	w, buf := memfile.NewPDFWriter(version, nil)

	if err := memfile.AddBlankPage(w); err != nil {
		t.Fatalf("add blank page: %v", err)
	}

	rm := pdf.NewResourceManager(w)
	ref, err := want.Encode(rm)
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
	got, err := pdf.ExtractorGet(x, nil, r.GetMeta().Catalog.AcroForm, DecodeInteractiveForm)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestRoundTrip(t *testing.T) {
	for _, version := range testVersions {
		for _, tc := range testCases {
			t.Run(tc.name+"-"+version.String(), func(t *testing.T) {
				roundTripTest(t, version, tc.form)
			})
		}
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, version := range testVersions {
		for _, tc := range testCases {
			w, buf := memfile.NewPDFWriter(version, opt)

			if err := memfile.AddBlankPage(w); err != nil {
				continue
			}

			rm := pdf.NewResourceManager(w)
			ref, err := tc.form.Encode(rm)
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
		form, err := pdf.ExtractorGet(x, nil, r.GetMeta().Catalog.AcroForm, DecodeInteractiveForm)
		if err != nil {
			t.Skip("malformed interactive form")
		}
		if form == nil {
			t.Skip("no interactive form")
		}

		roundTripTest(t, pdf.GetVersion(r), form)
	})
}

func TestEncodeInvalidAlign(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	form := &InteractiveForm{
		Fields: []pdf.Reference{rm.Out.Alloc()},
		Align:  pdf.TextAlign(99),
	}

	if _, err := form.Encode(rm); err == nil {
		t.Error("expected error for out-of-range alignment, got nil")
	}
}

func TestEncodeVersionGating(t *testing.T) {
	// the XFA array form requires PDF 1.6; encoding it to a PDF 1.4 file must fail.
	w, _ := memfile.NewPDFWriter(pdf.V1_4, nil)
	rm := pdf.NewResourceManager(w)

	form := &InteractiveForm{
		Fields: []pdf.Reference{rm.Out.Alloc()},
		XFA:    pdf.Array{pdf.String("x")},
	}

	_, err := form.Encode(rm)
	if !pdf.IsWrongVersion(err) {
		t.Errorf("expected version error, got %v", err)
	}
}

func TestEncodeXFAStreamForm(t *testing.T) {
	// the XFA stream form is valid from PDF 1.5, whereas the array form
	// requires PDF 1.6, so a non-array XFA value must encode at PDF 1.5.
	w, _ := memfile.NewPDFWriter(pdf.V1_5, nil)
	rm := pdf.NewResourceManager(w)

	form := &InteractiveForm{
		Fields: []pdf.Reference{rm.Out.Alloc()},
		XFA:    rm.Out.Alloc(), // reference to a stream
	}

	if _, err := form.Encode(rm); err != nil {
		t.Errorf("unexpected error encoding XFA stream form at PDF 1.5: %v", err)
	}
}

func TestEncodeVersionGatingEntries(t *testing.T) {
	tests := []struct {
		name    string
		version pdf.Version
		build   func(rm *pdf.ResourceManager) *InteractiveForm
	}{
		{"form requires 1.2", pdf.V1_1, func(rm *pdf.ResourceManager) *InteractiveForm {
			return &InteractiveForm{Fields: []pdf.Reference{rm.Out.Alloc()}}
		}},
		{"SigFlags requires 1.3", pdf.V1_2, func(rm *pdf.ResourceManager) *InteractiveForm {
			return &InteractiveForm{
				Fields:   []pdf.Reference{rm.Out.Alloc()},
				SigFlags: SignaturesExist,
			}
		}},
		{"CO requires 1.3", pdf.V1_2, func(rm *pdf.ResourceManager) *InteractiveForm {
			ref := rm.Out.Alloc()
			return &InteractiveForm{
				Fields:           []pdf.Reference{ref},
				CalculationOrder: []pdf.Reference{ref},
			}
		}},
		{"XFA array requires 1.6", pdf.V1_5, func(rm *pdf.ResourceManager) *InteractiveForm {
			return &InteractiveForm{
				Fields: []pdf.Reference{rm.Out.Alloc()},
				XFA:    pdf.Array{pdf.String("template"), pdf.String("<xdp/>")},
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(tc.version, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := tc.build(rm).Encode(rm); !pdf.IsWrongVersion(err) {
				t.Errorf("expected version error, got %v", err)
			}
		})
	}
}

func TestDecodeValues(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	x := pdf.NewExtractor(w)

	ref := w.Alloc()
	dict := pdf.Dict{
		// the second element is not a reference and must be skipped
		"Fields":          pdf.Array{ref, pdf.Integer(7)},
		"NeedAppearances": pdf.Boolean(true),
		"SigFlags":        pdf.Integer(3),
		"DA":              pdf.String("/Helv 0 Tf"),
		"Q":               pdf.Integer(99), // out of range, must be ignored
	}

	form, err := DecodeInteractiveForm(x, nil, dict, true)
	if err != nil {
		t.Fatal(err)
	}

	if len(form.Fields) != 1 || form.Fields[0] != ref {
		t.Errorf("Fields = %v, want [%v]", form.Fields, ref)
	}
	if !form.NeedAppearances {
		t.Error("NeedAppearances = false, want true")
	}
	if form.SigFlags != SignaturesExist|AppendOnly {
		t.Errorf("SigFlags = %d, want %d", form.SigFlags, SignaturesExist|AppendOnly)
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

	form, err := DecodeInteractiveForm(x, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if form != nil {
		t.Errorf("expected nil form for nil object, got %v", form)
	}
}
