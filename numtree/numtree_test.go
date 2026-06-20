// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

// The shared tree machinery is tested generically in the internal/pdftree
// package.  These tests cover the thin number-tree facade over it.

package numtree

import (
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestRoundTrip(t *testing.T) {
	data := map[pdf.Integer]pdf.Object{
		-5: pdf.Name("neg"),
		0:  pdf.Name("zero"),
		7:  pdf.Name("seven"),
	}
	keys := []pdf.Integer{-5, 0, 7}

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	seq := func(yield func(pdf.Integer, pdf.Object) bool) {
		for _, k := range keys {
			if !yield(k, data[k]) {
				return
			}
		}
	}
	ref, err := Write(w, seq)
	if err != nil {
		t.Fatal(err)
	}

	mem, err := ExtractInMemory(w, ref)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(data, mem.Data); diff != "" {
		t.Errorf("ExtractInMemory (-want +got):\n%s", diff)
	}

	ff, err := ExtractFromFile(w, ref)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := ff.Lookup(0); err != nil || v != pdf.Name("zero") {
		t.Errorf("Lookup(0) = %v, %v", v, err)
	}
	if _, err := ff.Lookup(99); err != ErrKeyNotFound {
		t.Errorf("Lookup(absent) = %v, want ErrKeyNotFound", err)
	}

	got := maps.Collect(ff.All())
	if diff := cmp.Diff(data, got); diff != "" {
		t.Errorf("FromFile.All (-want +got):\n%s", diff)
	}

	if n, err := Size(w, ref); err != nil || n != len(data) {
		t.Errorf("Size = %d, %v; want %d", n, err, len(data))
	}
}

func TestEmpty(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	seq := func(yield func(pdf.Integer, pdf.Object) bool) {}
	ref, err := Write(w, seq)
	if err != nil {
		t.Fatal(err)
	}
	if ref != 0 {
		t.Errorf("Write(empty) = %v, want the null reference", ref)
	}
}
