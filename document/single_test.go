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

package document

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestSinglePage(t *testing.T) {
	buf := &bytes.Buffer{}
	doc, err := WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, nil)
	if err != nil {
		t.Fatal(err)
	}

	doc.Circle(50, 50, 40)
	doc.Fill()

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRoundTrip(t *testing.T) {
	testVal := pdf.Integer(42)

	buf := &bytes.Buffer{}
	doc, err := WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := doc.Out.Alloc()
	err = doc.Out.Put(ref, testVal)
	if err != nil {
		t.Fatal(err)
	}
	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := pdf.Resolve(r, ref)
	if err != nil {
		t.Fatal(err)
	}
	if obj != testVal {
		t.Fatalf("expected %v, got %v", testVal, obj)
	}
}

func BenchmarkSinglePage(b *testing.B) {
	buf := &bytes.Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Reset()

		doc, err := WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, nil)
		if err != nil {
			b.Fatal(err)
		}

		doc.Circle(50, 50, 40)
		doc.Fill()

		err = doc.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRoundTrip(b *testing.B) {
	testVal := pdf.Integer(42)

	buf := &bytes.Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Reset()
		doc, err := WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, nil)
		if err != nil {
			b.Fatal(err)
		}
		ref := doc.Out.Alloc()
		err = doc.Out.Put(ref, testVal)
		if err != nil {
			b.Fatal(err)
		}
		err = doc.Close()
		if err != nil {
			b.Fatal(err)
		}

		r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
		if err != nil {
			b.Fatal(err)
		}
		obj, err := pdf.Resolve(r, ref)
		if err != nil {
			b.Fatal(err)
		}
		if obj != testVal {
			b.Fatalf("expected %v, got %v", testVal, obj)
		}
	}
}
