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
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestCopyReference(t *testing.T) {
	// build a chain of references: c -> b -> a -> 42
	orig, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
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
	dest, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
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
