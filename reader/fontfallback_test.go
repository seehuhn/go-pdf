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

package reader

import (
	"bytes"
	"io"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
)

// TestMissingFontSubstituted checks that text shown under a Tf naming a font
// the resources lack is still reported, in the substitute the resources'
// fallback supplies, and that without a fallback it is dropped as before.
func TestMissingFontSubstituted(t *testing.T) {
	rawStream := "BT /Missing 12 Tf (abc) Tj ET"
	open := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte(rawStream))), nil
	}

	run := func(res *content.Resources) (n int, F font.Instance) {
		r := New(pdf.NewExtractor(noopGetter{}))
		r.State = content.NewState(content.Page, res)
		r.Character = func(font.Code) error {
			n++
			return nil
		}
		if err := r.ProcessIter(content.NewScanner(open).NewIter()); err != nil {
			t.Fatal(err)
		}
		return n, r.State.GState.TextFont
	}

	n, F := run(&content.Resources{FontFallback: extract.StandardFontFallback})
	if n != 3 {
		t.Errorf("got %d characters with a fallback, want 3", n)
	}
	if F == nil || F.PostScriptName() != "Helvetica" {
		t.Errorf("substitute font = %v, want Helvetica", F)
	}

	n, F = run(&content.Resources{})
	if n != 0 {
		t.Errorf("got %d characters without a fallback, want 0", n)
	}
	if F != nil {
		t.Errorf("font set without a fallback: %v", F)
	}
}

// TestMissingResourcesSubstituted checks that the substitute reaches a page
// whose file gives it no Resources at all: the page view repairs the entry
// to an empty dictionary, which reads with the fallback installed.
func TestMissingResourcesSubstituted(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rootRef := w.Alloc()
	pageRef := w.Alloc()
	contentRef := w.Alloc()

	stm, err := w.OpenStream(contentRef, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stm.Write([]byte("BT /Missing 12 Tf (abc) Tj ET")); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	w.Put(pageRef, pdf.Dict{
		"Type":     pdf.Name("Page"),
		"Parent":   rootRef,
		"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(100), pdf.Integer(100)},
		"Contents": contentRef,
	})
	w.Put(rootRef, pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Count": pdf.Integer(1),
		"Kids":  pdf.Array{pageRef},
	})
	w.GetMeta().Catalog.Pages = rootRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	_, pageDict, err := pagetree.GetPage(w, 0)
	if err != nil {
		t.Fatal(err)
	}
	x := pdf.NewExtractor(w)
	pg, err := pdf.Decode(pdf.CursorAt(x, nil), pageDict, page.Decode)
	if err != nil {
		t.Fatal(err)
	}

	n := 0
	r := New(x)
	r.Character = func(font.Code) error {
		n++
		return nil
	}
	if err := r.ProcessPage(pg); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("got %d characters, want 3", n)
	}
}
