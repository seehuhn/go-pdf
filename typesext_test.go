// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package pdf_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/opaque"
)

func TestArrayRoundTrip(t *testing.T) {
	var cases = []pdf.Array{
		nil,
		{},
		{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)},
		{pdf.Integer(1), nil, pdf.Integer(3)},
		{pdf.Array{}},
	}
	for i, a := range cases {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			w, m := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			ref := w.Alloc()
			w.Put(ref, a)
			b, err := pdf.NewCursor(w).Array(ref)
			if err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(a, b); d != "" {
				fmt.Println(string(m.Data))
				t.Error(d)
			}
		})
	}
}

func TestSigContentsPlaintextRoundTrip(t *testing.T) {
	contents := pdf.String("\x30\x82\x01\x02 signature \x00\xff")
	hexForm := fmt.Sprintf("/Contents<%x>", []byte(contents))
	opt := &pdf.WriterOptions{UserPassword: "user", OwnerPassword: "owner"}

	newSigDict := func() pdf.Dict {
		return pdf.Dict{
			"Type":      pdf.Name("Sig"),
			"Filter":    pdf.Name("Adobe.PPKLite"),
			"ByteRange": pdf.Array{pdf.Integer(0), pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)},
			"Contents":  contents,
			"Reason":    pdf.String("a distinctive reason"),
		}
	}

	check := func(t *testing.T, file *memfile.MemFile, ref pdf.Reference) {
		t.Helper()
		if !bytes.Contains(file.Data, []byte(hexForm)) {
			t.Error("plain hex /Contents not found in file")
		}
		if bytes.Contains(file.Data, []byte("a distinctive reason")) {
			t.Error("/Reason is not encrypted")
		}
		r, err := pdf.NewReader(bytes.NewReader(file.Data), int64(len(file.Data)), &pdf.ReaderOptions{Password: "user"})
		if err != nil {
			t.Fatal(err)
		}
		obj, err := r.Get(ref, false)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := obj.(pdf.Dict)
		if !ok {
			t.Fatalf("expected a dict, got %T", obj)
		}
		if diff := cmp.Diff(newSigDict(), got); diff != "" {
			t.Errorf("round trip failed (-want +got):\n%s", diff)
		}
	}

	for _, v := range []pdf.Version{pdf.V1_4, pdf.V1_7} {
		t.Run(v.String()+"/direct", func(t *testing.T) {
			w, file := memfile.NewPDFWriter(t, v, opt)
			ref := w.Alloc()
			if err := w.Put(ref, newSigDict()); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			check(t, file, ref)
		})

		t.Run(v.String()+"/opaque", func(t *testing.T) {
			src, _ := memfile.NewPDFWriter(t, v, nil)
			srcRef := src.Alloc()
			if err := src.Put(srcRef, newSigDict()); err != nil {
				t.Fatal(err)
			}
			if err := src.Close(); err != nil {
				t.Fatal(err)
			}
			obj := opaque.Extract(pdf.NewExtractor(src), srcRef)

			w, file := memfile.NewPDFWriter(t, v, opt)
			rm := pdf.NewResourceManager(w)
			native, err := rm.Embed(obj)
			if err != nil {
				t.Fatal(err)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			ref, ok := native.(pdf.Reference)
			if !ok {
				t.Fatalf("expected a reference, got %T", native)
			}
			check(t, file, ref)
		})
	}
}
