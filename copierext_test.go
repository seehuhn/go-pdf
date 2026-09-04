// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestCopyReference(t *testing.T) {
	// build a chain of references: c -> b -> a -> 42
	orig, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	a := orig.Alloc()
	err := orig.Put(a, pdf.Integer(42))
	if err != nil {
		t.Fatal(err)
	}
	b := orig.Alloc()
	err = orig.Put(b, a)
	if err != nil {
		t.Fatal(err)
	}
	c := orig.Alloc()
	err = orig.Put(c, b)
	if err != nil {
		t.Fatal(err)
	}

	// copy the chain
	dest, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	copier := pdf.NewCopier(dest, orig)
	copiedC, err := copier.CopyReference(c)
	if err != nil {
		t.Fatal(err)
	}

	// check that copied reference points to the correct object
	obj, err := dest.Get(copiedC, true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != pdf.Integer(42) {
		t.Fatalf("expected 42, got %v", obj)
	}
}

// malformedGetter is a [pdf.Getter] whose every object is unreadable, as if
// the source file were corrupt.  It models a reference to an object whose
// body cannot be parsed.
type malformedGetter struct {
	meta *pdf.MetaInfo
}

func (g malformedGetter) GetMeta() *pdf.MetaInfo { return g.meta }

func (g malformedGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	return nil, &pdf.MalformedFileError{Err: errors.New("unparsable object")}
}

// A reference to a malformed source object must copy as a reference to null
// (PDF 2.0, 7.3.10), not abort the whole copy.  This mirrors a corrupt PDF
// whose stream dict carries a stray indirect reference to an unparsable
// object.
func TestCopyReferenceMalformedBecomesNull(t *testing.T) {
	dest, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	src := malformedGetter{meta: dest.GetMeta()}
	copier := pdf.NewCopier(dest, src)

	badRef := pdf.NewReference(1, 0)
	in := pdf.Dict{"Junk": badRef, "Width": pdf.Integer(10)}
	out, err := copier.CopyDict(in)
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}

	if out["Width"] != pdf.Integer(10) {
		t.Errorf("Width = %v, want 10", out["Width"])
	}

	ref, ok := out["Junk"].(pdf.Reference)
	if !ok {
		t.Fatalf("Junk = %T, want Reference", out["Junk"])
	}
	val, err := dest.Get(ref, true)
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Errorf("resolved value = %v, want null", val)
	}
}

// A Copier whose source is the file being updated must pass references
// through unchanged: the objects already exist in the output file, and
// allocating fresh copies would duplicate them in the update.
func TestCopyReferenceSameFile(t *testing.T) {
	f, oldRef := newBaseFile(t, pdf.V2_0, nil)
	w, err := pdf.NewUpdater(f, int64(len(f.Data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	before := len(f.Data)

	copier := pdf.NewCopier(w, w)
	got, err := copier.CopyReference(oldRef)
	if err != nil {
		t.Fatal(err)
	}
	if got != oldRef {
		t.Errorf("reference not passed through: got %v, want %v", got, oldRef)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(f.Data[before:], []byte("/old")); n != 0 {
		t.Errorf("source object copied into the update %d times", n)
	}
}
