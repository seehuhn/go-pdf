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
	"io"
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

// rotatedSource builds a one-page document whose page carries the given
// /Rotate value verbatim, and whose page-tree node carries parentRotate.
// A nil value omits the entry.
func rotatedSource(t *testing.T, pageRotate, parentRotate pdf.Object) *pdf.Reader {
	t.Helper()
	w, buf := memfile.NewPDFWriter(pdf.V1_7, nil)

	contentRef := w.Alloc()
	stm, err := w.OpenStream(contentRef, pdf.Dict{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(stm, "0 0 1 rg 10 10 40 20 re f\n"); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}

	pageRef, pagesRef := w.Alloc(), w.Alloc()
	pageDict := pdf.Dict{
		"Type": pdf.Name("Page"), "Parent": pagesRef,
		"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(300)},
		"Contents": contentRef,
	}
	if pageRotate != nil {
		pageDict["Rotate"] = pageRotate
	}
	w.Put(pageRef, pageDict)

	pagesDict := pdf.Dict{
		"Type": pdf.Name("Pages"), "Kids": pdf.Array{pageRef}, "Count": pdf.Integer(1),
	}
	if parentRotate != nil {
		pagesDict["Rotate"] = parentRotate
	}
	w.Put(pagesRef, pagesDict)
	w.GetMeta().Catalog.Pages = pagesRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(buf, int64(len(buf.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestWriteRotate checks the /Rotate entry of the output page.  The value is
// the one printprep reads, not the one the file gives: the flattened
// annotations are placed under that reading, so the page must agree with it.
// A value which is not a multiple of 90 reads as no rotation and is left out.
func TestWriteRotate(t *testing.T) {
	cases := []struct {
		name         string
		pageRotate   pdf.Object
		parentRotate pdf.Object
		want         pdf.Object
	}{
		{name: "absent"},
		{name: "upright", pageRotate: pdf.Integer(0)},
		{name: "quarterTurn", pageRotate: pdf.Integer(90), want: pdf.Integer(90)},
		{name: "negative", pageRotate: pdf.Integer(-90), want: pdf.Integer(270)},
		{name: "fullTurnPlus", pageRotate: pdf.Integer(450), want: pdf.Integer(90)},
		{name: "notAMultipleOf90", pageRotate: pdf.Integer(45)},
		{name: "real", pageRotate: pdf.Real(180), want: pdf.Integer(180)},
		{name: "malformed", pageRotate: pdf.Name("sideways")},
		{name: "inherited", parentRotate: pdf.Integer(270), want: pdf.Integer(270)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := rotatedSource(t, tc.pageRotate, tc.parentRotate)

			var out bytes.Buffer
			if err := Write(&out, r, nil); err != nil {
				t.Fatal(err)
			}
			res := out.Bytes()
			rr, err := pdf.NewReader(bytes.NewReader(res), int64(len(res)), nil)
			if err != nil {
				t.Fatal(err)
			}
			_, pageDict, err := pagetree.GetPage(rr, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := pageDict["Rotate"]; got != tc.want {
				t.Errorf("output /Rotate = %v, want %v", got, tc.want)
			}
		})
	}
}
