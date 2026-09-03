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
	"regexp"
	"strconv"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var sizePat = regexp.MustCompile(`/Size\s+(\d+)`)

// trailerSize returns the /Size named in the last trailer of data.
func trailerSize(t *testing.T, data []byte) uint32 {
	t.Helper()
	m := sizePat.FindAllSubmatch(data, -1)
	if m == nil {
		t.Fatal("no /Size entry")
	}
	n, err := strconv.Atoi(string(m[len(m)-1][1]))
	if err != nil {
		t.Fatal(err)
	}
	return uint32(n)
}

// newBaseFile writes a one-page document plus one extra object holding
// pdf.Name("old"), and returns the file and that object's reference.
func newBaseFile(t *testing.T, v pdf.Version, opt *pdf.WriterOptions) (*memfile.MemFile, pdf.Reference) {
	t.Helper()
	w, f := memfile.NewPDFWriter(t, v, opt)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Name("old")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return f, ref
}

func TestUpdaterReadsOriginal(t *testing.T) {
	f, oldRef := newBaseFile(t, pdf.V1_7, nil)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	pages := w.GetMeta().Catalog.Pages
	if pages == 0 {
		t.Fatal("catalog has no Pages reference")
	}
	obj, err := w.Get(pages, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := obj.(pdf.Dict); !ok {
		t.Errorf("pages object is %T, want Dict", obj)
	}
	obj, err = w.Get(oldRef, true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != pdf.Name("old") {
		t.Errorf("got %v, want /old", obj)
	}
}

func TestUpdaterAllocStartsAtSize(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_4, nil)
	size := trailerSize(t, f.Data)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if ref := w.Alloc(); ref.Number() != size {
		t.Errorf("first allocated number %d, want %d", ref.Number(), size)
	}
}

func TestUpdaterGetNewObject(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_7, nil)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Name("added")); err != nil {
		t.Fatal(err)
	}
	obj, err := w.Get(ref, true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != pdf.Name("added") {
		t.Errorf("got %v, want /added", obj)
	}
}

func TestUpdaterFree(t *testing.T) {
	f, oldRef := newBaseFile(t, pdf.V1_7, nil)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}

	// not in the original
	if err := w.Free(pdf.NewReference(trailerSize(t, f.Data)+5, 0)); err == nil {
		t.Error("freeing an unknown object succeeded")
	}
	// wrong generation
	if err := w.Free(pdf.NewReference(oldRef.Number(), 1)); err == nil {
		t.Error("freeing with a wrong generation succeeded")
	}

	if err := w.Free(oldRef); err != nil {
		t.Fatal(err)
	}
	obj, err := w.Get(oldRef, true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != nil {
		t.Errorf("freed object still reads as %v", obj)
	}
	// twice
	if err := w.Free(oldRef); err == nil {
		t.Error("freeing twice succeeded")
	}

	// Free after Put
	pages := w.GetMeta().Catalog.Pages
	if err := w.Put(pages, pdf.Dict{"Type": pdf.Name("Pages"), "Kids": pdf.Array{}, "Count": pdf.Integer(0)}); err != nil {
		t.Fatal(err)
	}
	if err := w.Free(pages); err == nil {
		t.Error("Free after Put succeeded")
	}
}

func TestUpdaterRejectsWrongPassword(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_7, &pdf.WriterOptions{UserPassword: "u", OwnerPassword: "o"})
	_, err := pdf.NewUpdater(f, int64(len(f.Data)), &pdf.UpdateOptions{Password: "wrong"})
	if err == nil {
		t.Error("update of an encrypted file opened without the password")
	}
}

func TestUpdaterRejectsLowerVersion(t *testing.T) {
	f, _ := newBaseFile(t, pdf.V1_4, nil)
	_, err := pdf.NewUpdater(f, int64(len(f.Data)), &pdf.UpdateOptions{Version: pdf.V1_3})
	if err == nil {
		t.Error("lowering the version succeeded")
	}
}

func TestUpdaterToCopiesOriginal(t *testing.T) {
	f, oldRef := newBaseFile(t, pdf.V1_4, nil)
	orig := bytes.Clone(f.Data)
	dst := memfile.New()
	w, err := pdf.NewUpdaterTo(bytes.NewReader(orig), int64(len(orig)), dst, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dst.Data, orig) {
		t.Error("destination does not start with a copy of the original")
	}
	obj, err := w.Get(oldRef, true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != pdf.Name("old") {
		t.Errorf("got %v, want /old", obj)
	}
}
