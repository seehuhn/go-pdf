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

package pagetree

import (
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestContentStream(t *testing.T) {
	pdfData, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	tree := NewWriter(pdfData)

	A := addStream(t, pdfData, "A")
	B := addStream(t, pdfData, "B", pdf.FilterCompress{})
	C := addStream(t, pdfData, "C", pdf.FilterCompress{}, pdf.FilterASCIIHex{})

	type testCase struct {
		name     string
		contents pdf.Object
		expect   string
		ref      pdf.Reference
	}
	cases := []*testCase{
		{
			name:     "missing",
			contents: nil,
			expect:   "",
		},
		{
			name:     "empty",
			contents: pdf.Array{},
			expect:   "",
		},
		{
			name:     "A",
			contents: A,
			expect:   "A",
		},
		{
			name:     "AB",
			contents: pdf.Array{A, B},
			expect:   "A\nB",
		},
		{
			name:     "ABC",
			contents: pdf.Array{A, B, C},
			expect:   "A\nB\nC",
		},
		{
			name:     "gap",
			contents: pdf.Array{A, nil, C},
			expect:   "A\nC",
		},
	}

	for _, test := range cases {
		test.ref = pdfData.Alloc()
		dict := pdf.Dict{
			"Contents": test.contents,
		}
		err := tree.AppendPageRef(test.ref, dict)
		if err != nil {
			t.Fatal(err)
		}
	}

	treeRef, err := tree.Close()
	if err != nil {
		t.Fatal(err)
	}
	pdfData.GetMeta().Catalog.Pages = treeRef
	err = pdfData.Close()
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			r, err := ContentStream(pdfData, test.ref)
			if err != nil {
				t.Fatal(err)
			}

			body, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}

			if string(body) != test.expect {
				t.Errorf("expected %q, got %q", test.expect, body)
			}
		})
	}
}

func addStream(t *testing.T, w *pdf.Writer, body string, ff ...pdf.Filter) pdf.Reference {
	t.Helper()

	ref := w.Alloc()
	stm, err := w.OpenStream(ref, nil, ff...)
	if err != nil {
		t.Fatal(err)
	}

	_, err = stm.Write([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	err = stm.Close()
	if err != nil {
		t.Fatal(err)
	}

	return ref
}
