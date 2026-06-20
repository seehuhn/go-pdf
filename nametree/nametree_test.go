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
// package.  These tests cover the thin name-tree facade over it.

package nametree

import (
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestRoundTrip(t *testing.T) {
	data := map[pdf.Name]pdf.Object{
		"alpha": pdf.Integer(1),
		"beta":  pdf.Integer(2),
		"gamma": pdf.Integer(3),
	}

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref, err := WriteMap(w, data)
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
	if v, err := ff.Lookup("beta"); err != nil || v != pdf.Integer(2) {
		t.Errorf("Lookup(beta) = %v, %v", v, err)
	}
	if _, err := ff.Lookup("absent"); err != ErrKeyNotFound {
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

func TestWrite(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	seq := func(yield func(pdf.Name, pdf.Object) bool) {
		yield("only", pdf.Integer(42))
	}
	ref, err := Write(w, seq)
	if err != nil {
		t.Fatal(err)
	}
	mem, err := ExtractInMemory(w, ref)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := mem.Data["only"]; !ok || got != pdf.Integer(42) {
		t.Errorf("only = %v (ok=%v), want 42", got, ok)
	}
}

func TestEmpty(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref, err := WriteMap(w, map[pdf.Name]pdf.Object{})
	if err != nil {
		t.Fatal(err)
	}
	if ref != 0 {
		t.Errorf("WriteMap(empty) = %v, want the null reference", ref)
	}
}
