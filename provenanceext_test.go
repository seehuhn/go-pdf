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

package pdf_test

import (
	"bytes"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestTwoManagersShareObjects(t *testing.T) {
	w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm1 := pdf.NewResourceManager(w)
	rm2 := pdf.NewResourceManager(w)

	info := &pdf.Info{Title: "shared"}
	ref1, err := rm1.Embed(info)
	if err != nil {
		t.Fatal(err)
	}
	ref2, err := rm2.Embed(info)
	if err != nil {
		t.Fatal(err)
	}
	if ref1 != ref2 {
		t.Errorf("same value embedded as %v and %v", ref1, ref2)
	}
	if err := rm1.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rm2.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(f.Data, []byte("(shared)")); n != 1 {
		t.Errorf("title written %d times, want 1", n)
	}
}

// newUpdateBase writes a one-page file with an Info dictionary and one
// extra object, and returns it.
func newUpdateBase(t *testing.T) *memfile.MemFile {
	t.Helper()
	w, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	w.GetMeta().Info.Title = "base"
	if err := w.Put(w.Alloc(), pdf.Name("extra")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestOriginOfDecodedValue(t *testing.T) {
	f := newUpdateBase(t)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	pagesRef := w.GetMeta().Catalog.Pages
	pg, err := pdf.Decode(c, pagesRef, func(c pdf.Cursor, obj pdf.Object, _ bool) (*pdf.Dict, error) {
		d, err := c.Dict(obj)
		return &d, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Origin(pg); got != pagesRef {
		t.Errorf("Origin = %v, want %v", got, pagesRef)
	}
	if got := w.Origin(&pdf.Dict{}); got != 0 {
		t.Errorf("Origin of unrelated value = %v, want 0", got)
	}
}

func TestDecodedValueEmbedsAsOrigin(t *testing.T) {
	f := newUpdateBase(t)
	orig := bytes.Clone(f.Data)
	w, err := pdf.NewUpdater(f, int64(len(orig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	infoRef := w.GetMeta().Trailer["Info"].(pdf.Reference)
	info, err := pdf.Decode(c, infoRef, pdf.ExtractInfo)
	if err != nil {
		t.Fatal(err)
	}
	rm := pdf.NewResourceManager(w)
	got, err := rm.Embed(info)
	if err != nil {
		t.Fatal(err)
	}
	if got != infoRef {
		t.Errorf("embedded as %v, want %v", got, infoRef)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(f.Data[len(orig):], []byte("(base)")) {
		t.Error("unchanged Info was written again")
	}
}

func TestValueFromOtherGetterHasNoOrigin(t *testing.T) {
	f := newUpdateBase(t)
	r, err := pdf.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	infoRef := r.GetMeta().Trailer["Info"].(pdf.Reference)
	info, err := pdf.Decode(pdf.NewCursor(r), infoRef, pdf.ExtractInfo)
	if err != nil {
		t.Fatal(err)
	}

	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Origin(info); got != 0 {
		t.Errorf("Origin = %v for a value from another Getter", got)
	}
	rm := pdf.NewResourceManager(w)
	got, err := rm.Embed(info)
	if err != nil {
		t.Fatal(err)
	}
	if got == infoRef {
		t.Error("value from another Getter reused the original reference")
	}
}

func TestInlineValueHasNoOrigin(t *testing.T) {
	f := newUpdateBase(t)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	v, err := pdf.Decode(c, pdf.Dict{"A": pdf.Integer(1)}, func(c pdf.Cursor, obj pdf.Object, _ bool) (*pdf.Dict, error) {
		d, err := c.Dict(obj)
		return &d, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Origin(v); got != 0 {
		t.Errorf("Origin = %v for an inline value", got)
	}
}

func TestOriginFollowsReferenceChain(t *testing.T) {
	w0, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	target := w0.Alloc()
	middle := w0.Alloc()
	if err := w0.Put(target, pdf.Dict{"Kind": pdf.Name("target")}); err != nil {
		t.Fatal(err)
	}
	if err := w0.Put(middle, target); err != nil {
		t.Fatal(err)
	}
	if err := w0.Close(); err != nil {
		t.Fatal(err)
	}

	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err := pdf.Decode(pdf.NewCursor(w), middle, func(c pdf.Cursor, obj pdf.Object, _ bool) (*pdf.Dict, error) {
		d, err := c.Dict(obj)
		return &d, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Origin(v); got != target {
		t.Errorf("Origin = %v, want the last reference %v", got, target)
	}
}

func TestReplaceWritesInPlace(t *testing.T) {
	f := newUpdateBase(t)
	orig := bytes.Clone(f.Data)
	w, err := pdf.NewUpdater(f, int64(len(orig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	infoRef := w.GetMeta().Trailer["Info"].(pdf.Reference)
	old, err := pdf.Decode(c, infoRef, pdf.ExtractInfo)
	if err != nil {
		t.Fatal(err)
	}

	repl := *old
	repl.Title = "replaced"
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Replace(old, &repl)
	if err != nil {
		t.Fatal(err)
	}
	if ref != infoRef {
		t.Errorf("Replace wrote at %v, want %v", ref, infoRef)
	}
	if got, _ := rm.Embed(old); got != infoRef {
		t.Errorf("original embeds as %v after Replace", got)
	}
	if got, _ := rm.Embed(&repl); got != infoRef {
		t.Errorf("replacement embeds as %v after Replace", got)
	}
	if _, err := rm.Replace(old, &repl); err == nil {
		t.Error("second Replace of the same object succeeded")
	}
	if _, err := rm.Replace(old, "not an encoder"); err == nil {
		t.Error("Replace with a non-Encoder succeeded")
	}
	if _, err := rm.Replace(&pdf.Info{}, &repl); err == nil {
		t.Error("Replace of a value without provenance succeeded")
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.GetMeta().Info == nil || r.GetMeta().Info.Title != "replaced" {
		t.Errorf("Info after update: %+v", r.GetMeta().Info)
	}
}

func TestReplaceSkipsEqualEncoding(t *testing.T) {
	w0, f := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	ref0 := w0.Alloc()
	if err := w0.Put(ref0, pdf.Dict{"Type": pdf.Name("Catalog"), "Pages": w0.GetMeta().Catalog.Pages}); err != nil {
		t.Fatal(err)
	}
	if err := w0.Close(); err != nil {
		t.Fatal(err)
	}
	orig := bytes.Clone(f.Data)

	w, err := pdf.NewUpdater(f, int64(len(orig)), nil)
	if err != nil {
		t.Fatal(err)
	}
	old, err := pdf.Decode(pdf.NewCursor(w), ref0, pdf.DecodeCatalog)
	if err != nil {
		t.Fatal(err)
	}
	repl := *old
	rm := pdf.NewResourceManager(w)
	if _, err := rm.Replace(old, &repl); err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(f.Data[len(orig):], []byte("/Catalog")) {
		t.Error("an equal replacement was written")
	}
}

func TestOriginVoidAfterFree(t *testing.T) {
	f := newUpdateBase(t)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)
	infoRef := w.GetMeta().Trailer["Info"].(pdf.Reference)
	info, err := pdf.Decode(c, infoRef, pdf.ExtractInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Free(infoRef); err != nil {
		t.Fatal(err)
	}
	if got := w.Origin(info); got != 0 {
		t.Errorf("Origin = %v after Free, want 0", got)
	}
	rm := pdf.NewResourceManager(w)
	got, err := rm.Embed(info)
	if err != nil {
		t.Fatal(err)
	}
	if got == infoRef {
		t.Error("embedding after Free reused the freed reference")
	}
}
