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

package printprep

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/pagetree"
)

// makeSource builds an n-page document with a filled rectangle per page and
// some document metadata, and returns a reader over it.
func makeSource(t *testing.T, n int) *pdf.Reader {
	t.Helper()
	buf := memfile.New()
	doc, err := document.WriteMultiPage(buf, document.A4, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	doc.Out.GetMeta().Info = &pdf.Info{
		Title:    "secret title",
		Author:   "somebody",
		Producer: "test",
	}
	for i := range n {
		p := doc.AddPage()
		p.SetFillColor(color.DeviceGray(0.5))
		p.Rectangle(72, 72, float64(100+10*i), 100)
		p.Fill()
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := doc.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := pdf.NewReader(buf, int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestWriteBasic(t *testing.T) {
	r := makeSource(t, 3)

	var out bytes.Buffer
	if err := Write(&out, r, nil); err != nil {
		t.Fatal(err)
	}

	res := out.Bytes()
	rr, err := pdf.NewReader(bytes.NewReader(res), int64(len(res)), nil)
	if err != nil {
		t.Fatalf("output does not open: %v", err)
	}

	n, err := pagetree.NumPages(rr)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("got %d pages, want 3", n)
	}

	// metadata must be stripped
	if info := rr.GetMeta().Info; info != nil && info.Title != "" {
		t.Errorf("Info.Title survived: %q", info.Title)
	}
	// output must be unencrypted
	if rr.GetMeta().Trailer["Encrypt"] != nil {
		t.Error("output is encrypted")
	}
}

func TestWriteWithText(t *testing.T) {
	buf := memfile.New()
	doc, err := document.WriteMultiPage(buf, document.A4, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	F, err := gofont.Regular.NewSimple(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := doc.AddPage()
	p.TextBegin()
	p.TextSetFont(F, 12)
	p.TextFirstLine(72, 700)
	p.TextShow("Hello, printprep!")
	p.TextEnd()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := doc.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := pdf.NewReader(buf, int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Write(&out, r, nil); err != nil {
		t.Fatal(err)
	}
	res := out.Bytes()
	rr, err := pdf.NewReader(bytes.NewReader(res), int64(len(res)), nil)
	if err != nil {
		t.Fatalf("output does not open: %v", err)
	}
	_, page, err := pagetree.GetPage(rr, 0)
	if err != nil {
		t.Fatal(err)
	}
	resDict, err := pdf.NewCursor(rr).Dict(page["Resources"])
	if err != nil {
		t.Fatal(err)
	}
	fonts, err := pdf.NewCursor(rr).Dict(resDict["Font"])
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) == 0 {
		t.Error("no font resource in converted page")
	}
}

func TestWritePageSubset(t *testing.T) {
	r := makeSource(t, 4)

	var out bytes.Buffer
	if err := Write(&out, r, &Options{Pages: []int{2, 0}}); err != nil {
		t.Fatal(err)
	}

	res := out.Bytes()
	rr, err := pdf.NewReader(bytes.NewReader(res), int64(len(res)), nil)
	if err != nil {
		t.Fatal(err)
	}
	n, err := pagetree.NumPages(rr)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("got %d pages, want 2", n)
	}
}

func TestWriteFromEncrypted(t *testing.T) {
	buf := memfile.New()
	opt := &pdf.WriterOptions{UserPassword: "secret"}
	doc, err := document.WriteMultiPage(buf, document.A4, pdf.V1_7, opt)
	if err != nil {
		t.Fatal(err)
	}
	p := doc.AddPage()
	p.SetFillColor(color.DeviceGray(0.5))
	p.Rectangle(72, 72, 100, 100)
	p.Fill()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := doc.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := pdf.NewReader(buf, int64(len(buf.Data)), &pdf.ReaderOptions{Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Write(&out, r, nil); err != nil {
		t.Fatal(err)
	}
	res := out.Bytes()
	rr, err := pdf.NewReader(bytes.NewReader(res), int64(len(res)), nil)
	if err != nil {
		t.Fatalf("output does not open without password: %v", err)
	}
	if rr.GetMeta().Trailer["Encrypt"] != nil {
		t.Error("output is still encrypted")
	}
}
