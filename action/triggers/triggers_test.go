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

package triggers

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// test actions
var (
	uriAction = &action.URI{URI: "https://example.com/"}
	jsAction  = &action.JavaScript{JS: pdf.String("app.alert('Hello');")}
)

// --- Annotation tests ---

var annotationTestCases = []struct {
	name string
	aa   *Annotation
}{
	{"empty", &Annotation{}},
	{"enter only", &Annotation{Enter: uriAction}},
	{"exit only", &Annotation{Exit: uriAction}},
	{"down only", &Annotation{Down: uriAction}},
	{"up only", &Annotation{Up: uriAction}},
	{"focus only", &Annotation{Focus: uriAction}},
	{"blur only", &Annotation{Blur: uriAction}},
	{"page open only", &Annotation{PageOpen: uriAction}},
	{"page close only", &Annotation{PageClose: uriAction}},
	{"page visible only", &Annotation{PageVisible: uriAction}},
	{"page invisible only", &Annotation{PageInvisible: uriAction}},
	{"multiple mouse events", &Annotation{
		Enter: uriAction,
		Exit:  uriAction,
		Down:  uriAction,
		Up:    uriAction,
	}},
	{"focus and blur", &Annotation{
		Focus: uriAction,
		Blur:  uriAction,
	}},
	{"all page events", &Annotation{
		PageOpen:      uriAction,
		PageClose:     uriAction,
		PageVisible:   uriAction,
		PageInvisible: uriAction,
	}},
	{"all events", &Annotation{
		Enter:         uriAction,
		Exit:          uriAction,
		Down:          uriAction,
		Up:            uriAction,
		Focus:         uriAction,
		Blur:          uriAction,
		PageOpen:      uriAction,
		PageClose:     uriAction,
		PageVisible:   uriAction,
		PageInvisible: uriAction,
	}},
}

func testAnnotationRoundTrip(t *testing.T, v pdf.Version, aa *Annotation) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(w)

	obj, err := aa.Encode(rm)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip()
		}
		t.Fatalf("encode failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("close rm failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := DecodeAnnotation(x, obj)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if diff := cmp.Diff(aa, decoded); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestAnnotationRoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_5, pdf.V1_7, pdf.V2_0} {
		for _, tc := range annotationTestCases {
			t.Run(tc.name+"-"+v.String(), func(t *testing.T) {
				testAnnotationRoundTrip(t, v, tc.aa)
			})
		}
	}
}

func FuzzAnnotationRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range annotationTestCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)
		rm := pdf.NewResourceManager(w)

		if err := memfile.AddBlankPage(w); err != nil {
			continue
		}

		obj, err := tc.aa.Encode(rm)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = obj
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		aa, err := DecodeAnnotation(x, obj)
		if err != nil {
			t.Skip("malformed annotation AA")
		}
		if aa == nil {
			t.Skip("nil annotation AA")
		}

		testAnnotationRoundTrip(t, pdf.GetVersion(r), aa)
	})
}

// --- Page tests ---

var pageTestCases = []struct {
	name string
	aa   *Page
}{
	{"empty", &Page{}},
	{"page open only", &Page{PageOpen: uriAction}},
	{"page close only", &Page{PageClose: uriAction}},
	{"both events", &Page{
		PageOpen:  uriAction,
		PageClose: uriAction,
	}},
}

func testPageRoundTrip(t *testing.T, v pdf.Version, aa *Page) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(w)

	obj, err := aa.Encode(rm)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip()
		}
		t.Fatalf("encode failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("close rm failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := DecodePage(x, obj)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if diff := cmp.Diff(aa, decoded); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestPageRoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_2, pdf.V1_7, pdf.V2_0} {
		for _, tc := range pageTestCases {
			t.Run(tc.name+"-"+v.String(), func(t *testing.T) {
				testPageRoundTrip(t, v, tc.aa)
			})
		}
	}
}

func FuzzPageRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range pageTestCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)
		rm := pdf.NewResourceManager(w)

		if err := memfile.AddBlankPage(w); err != nil {
			continue
		}

		obj, err := tc.aa.Encode(rm)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = obj
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		aa, err := DecodePage(x, obj)
		if err != nil {
			t.Skip("malformed page AA")
		}
		if aa == nil {
			t.Skip("nil page AA")
		}

		testPageRoundTrip(t, pdf.GetVersion(r), aa)
	})
}

// --- Form tests ---

var formTestCases = []struct {
	name string
	aa   *Form
}{
	{"empty", &Form{}},
	{"keystroke only", &Form{Keystroke: jsAction}},
	{"format only", &Form{Format: jsAction}},
	{"validate only", &Form{Validate: jsAction}},
	{"calculate only", &Form{Calculate: jsAction}},
	{"all events", &Form{
		Keystroke: jsAction,
		Format:    jsAction,
		Validate:  jsAction,
		Calculate: jsAction,
	}},
}

func testFormRoundTrip(t *testing.T, v pdf.Version, aa *Form) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(w)

	obj, err := aa.Encode(rm)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip()
		}
		t.Fatalf("encode failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("close rm failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := DecodeForm(x, obj)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if diff := cmp.Diff(aa, decoded); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestFormRoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_3, pdf.V1_7, pdf.V2_0} {
		for _, tc := range formTestCases {
			t.Run(tc.name+"-"+v.String(), func(t *testing.T) {
				testFormRoundTrip(t, v, tc.aa)
			})
		}
	}
}

func FuzzFormRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range formTestCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)
		rm := pdf.NewResourceManager(w)

		if err := memfile.AddBlankPage(w); err != nil {
			continue
		}

		obj, err := tc.aa.Encode(rm)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = obj
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		aa, err := DecodeForm(x, obj)
		if err != nil {
			t.Skip("malformed form AA")
		}
		if aa == nil {
			t.Skip("nil form AA")
		}

		testFormRoundTrip(t, pdf.GetVersion(r), aa)
	})
}

// --- Catalog tests ---

var catalogTestCases = []struct {
	name string
	aa   *Catalog
}{
	{"empty", &Catalog{}},
	{"will close only", &Catalog{WillClose: jsAction}},
	{"will save only", &Catalog{WillSave: jsAction}},
	{"did save only", &Catalog{DidSave: jsAction}},
	{"will print only", &Catalog{WillPrint: jsAction}},
	{"did print only", &Catalog{DidPrint: jsAction}},
	{"save events", &Catalog{
		WillSave: jsAction,
		DidSave:  jsAction,
	}},
	{"print events", &Catalog{
		WillPrint: jsAction,
		DidPrint:  jsAction,
	}},
	{"all events", &Catalog{
		WillClose: jsAction,
		WillSave:  jsAction,
		DidSave:   jsAction,
		WillPrint: jsAction,
		DidPrint:  jsAction,
	}},
}

func testCatalogRoundTrip(t *testing.T, v pdf.Version, aa *Catalog) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(v, nil)
	rm := pdf.NewResourceManager(w)

	obj, err := aa.Encode(rm)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip()
		}
		t.Fatalf("encode failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("close rm failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := DecodeCatalog(x, obj)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if diff := cmp.Diff(aa, decoded); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestCatalogRoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_4, pdf.V1_7, pdf.V2_0} {
		for _, tc := range catalogTestCases {
			t.Run(tc.name+"-"+v.String(), func(t *testing.T) {
				testCatalogRoundTrip(t, v, tc.aa)
			})
		}
	}
}

func FuzzCatalogRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range catalogTestCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)
		rm := pdf.NewResourceManager(w)

		if err := memfile.AddBlankPage(w); err != nil {
			continue
		}

		obj, err := tc.aa.Encode(rm)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = obj
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		aa, err := DecodeCatalog(x, obj)
		if err != nil {
			t.Skip("malformed catalog AA")
		}
		if aa == nil {
			t.Skip("nil catalog AA")
		}

		testCatalogRoundTrip(t, pdf.GetVersion(r), aa)
	})
}
