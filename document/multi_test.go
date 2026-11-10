// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package document

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestMultiPage(t *testing.T) {
	doc, err := CreateMultiPage("test.pdf", &pdf.Rectangle{URx: 100, URy: 100}, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}

	page := doc.AddPage()
	page.Circle(50, 50, 40)
	page.Fill()
	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	page = doc.AddPage()
	page.Rectangle(10, 10, 80, 80)
	page.Fill()
	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	page = doc.AddPage()
	page.MoveTo(10, 10)
	page.LineTo(90, 10)
	page.LineTo(50, 90)
	page.ClosePath()
	page.Fill()
	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkMultiPage(b *testing.B) {
	buf := &bytes.Buffer{}
	for i := 0; i < b.N; i++ {
		buf.Reset()
		doc, err := WriteMultiPage(buf, &pdf.Rectangle{URx: 100, URy: 100}, pdf.V1_7, nil)
		if err != nil {
			b.Fatal(err)
		}

		page := doc.AddPage()
		page.Circle(50, 50, 40)
		page.Fill()
		err = page.Close()
		if err != nil {
			b.Fatal(err)
		}

		page = doc.AddPage()
		page.Rectangle(10, 10, 80, 80)
		page.Fill()
		err = page.Close()
		if err != nil {
			b.Fatal(err)
		}

		page = doc.AddPage()
		page.MoveTo(10, 10)
		page.LineTo(90, 10)
		page.LineTo(50, 90)
		page.ClosePath()
		page.Fill()
		err = page.Close()
		if err != nil {
			b.Fatal(err)
		}

		err = doc.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}
