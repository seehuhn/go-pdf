// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package document

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestSinglePage(t *testing.T) {
	buf := &bytes.Buffer{}
	doc, err := WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, pdf.V1_7, nil)
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
	doc, err := WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, pdf.V1_7, nil)
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

	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil)
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

		doc, err := WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, pdf.V1_7, nil)
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
		doc, err := WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, pdf.V1_7, nil)
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

		r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil)
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
