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

func TestMultiPage(t *testing.T) {
	doc, err := CreateMultiPage("test.pdf", &pdf.Rectangle{URx: 100, URy: 100}, nil)
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
		doc, err := WriteMultiPage(buf, &pdf.Rectangle{URx: 100, URy: 100}, nil)
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
