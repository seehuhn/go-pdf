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

package pagetree_test

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/pagetree"
)

func TestReader(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := document.WriteMultiPage(buf, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300; i++ {
		page := w.AddPage()
		page.PageDict["Test"] = pdf.Integer(99 - 2*i)
		err = page.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := pagetree.NewReader(r)
	if err != nil {
		t.Fatal(err)
	}
	n, err := pages.NumPages()
	if err != nil {
		t.Fatal(err)
	}
	if n != 300 {
		t.Fatalf("expected 300 pages, got %d", n)
	}
	for i := 0; i < 300; i++ {
		page, err := pages.Get(pdf.Integer(i))
		if err != nil {
			t.Fatal(err)
		}
		v, err := r.GetInt(page["Test"])
		if err != nil {
			t.Fatal(err)
		}
		if v != pdf.Integer(99-2*i) {
			t.Fatalf("expected %d, got %d", 99-2*i, v)
		}
	}

	_, err = pages.Get(300)
	if err == nil {
		t.Fatalf("expected an error")
	}

	_, err = pages.Get(-1)
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func BenchmarkReader(b *testing.B) {
	buf := &bytes.Buffer{}
	w, err := document.WriteMultiPage(buf, 0, 0)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 300; i++ {
		page := w.AddPage()
		page.PageDict["Test"] = pdf.Integer(99 - 2*i)
		err = page.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
	err = w.Close()
	if err != nil {
		b.Fatal(err)
	}
	body := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := pdf.NewReader(bytes.NewReader(body), nil)
		if err != nil {
			b.Fatal(err)
		}
		pages, err := pagetree.NewReader(r)
		if err != nil {
			b.Fatal(err)
		}
		n, err := pages.NumPages()
		if err != nil {
			b.Fatal(err)
		}
		if n != 300 {
			b.Fatalf("expected 300 pages, got %d", n)
		}
		for i := 0; i < 300; i++ {
			page, err := pages.Get(pdf.Integer(i))
			if err != nil {
				b.Fatal(err)
			}
			v, err := r.GetInt(page["Test"])
			if err != nil {
				b.Fatal(err)
			}
			if v != pdf.Integer(99-2*i) {
				b.Fatalf("expected %d, got %d", 99-2*i, v)
			}
		}
	}
}
