// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"math"
	"os"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
)

func FuzzReadWrite(f *testing.F) {
	buf := &bytes.Buffer{}

	// minimal PDF file
	w, err := pdf.NewWriter(buf, pdf.V1_7, nil)
	if err != nil {
		f.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = w.Alloc() // pretend we have a page tree
	err = w.Close()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())

	// minimal working PDF file
	buf = &bytes.Buffer{}
	page, err := document.WriteSinglePage(buf, document.A4, pdf.V1_7, nil)
	if err != nil {
		f.Fatal(err)
	}
	err = page.Close()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())

	// PDF file which contains all object types
	buf = &bytes.Buffer{}
	w, err = pdf.NewWriter(buf, pdf.V1_7, nil)
	if err != nil {
		f.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = w.Alloc() // pretend we have a page tree
	objs := []pdf.Object{
		pdf.Array{pdf.Integer(1), pdf.Integer(2), pdf.Integer(3)},
		pdf.Boolean(true),
		pdf.Dict{
			"foo": pdf.Integer(1),
			"bar": pdf.Integer(2),
			"baz": pdf.Name("3"),
		},
		pdf.Integer(-1),
		pdf.Name("test name"),
		pdf.Real(math.Pi),
		pdf.NewReference(999, 1),
		&pdf.Stream{
			Dict: pdf.Dict{"Test": pdf.Boolean(true), "Length": pdf.Integer(11)},
			R:    strings.NewReader("test stream"),
		},
		pdf.String("test string"),
		nil,
	}
	for _, obj := range objs {
		err = w.Put(w.Alloc(), obj)
		if err != nil {
			f.Fatal(err)
		}
	}
	err = w.Close()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())

	f.Fuzz(func(t *testing.T, raw []byte) {
		r := bytes.NewReader(raw)
		pdf1, err := pdf.Read(r, nil)
		if err != nil {
			return
		}
		buf := &bytes.Buffer{}
		err = pdf1.Write(buf)
		if err != nil {
			t.Fatal(err)
		}
		pdfContents1 := buf.Bytes()

		r = bytes.NewReader(pdfContents1)
		pdf2, err := pdf.Read(r, nil)
		if err != nil {
			t.Fatal(err)
		}
		buf = &bytes.Buffer{}
		err = pdf2.Write(buf)
		if err != nil {
			t.Fatal(err)
		}
		pdfContents2 := buf.Bytes()

		if !bytes.Equal(pdfContents1, pdfContents2) {
			os.WriteFile("a.pdf", pdfContents1, 0644)
			os.WriteFile("b.pdf", pdfContents2, 0644)
			t.Fatalf("pdf contents differ")
		}
	})
}
