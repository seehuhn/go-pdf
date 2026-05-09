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

package numtree

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestKidsSelfCycle verifies that a number-tree node whose /Kids array
// references itself does not cause unbounded recursion.
func TestKidsSelfCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	rootRef := w.Alloc()
	root := pdf.Dict{
		"Kids":   pdf.Array{rootRef},
		"Limits": pdf.Array{pdf.Integer(0), pdf.Integer(1000)},
	}
	if err := w.Put(rootRef, root); err != nil {
		t.Fatal(err)
	}

	// ExtractInMemory must not stack-overflow.
	mem, err := ExtractInMemory(w, rootRef)
	if err != nil {
		t.Errorf("ExtractInMemory: unexpected error %v", err)
	}
	if mem != nil && len(mem.Data) != 0 {
		t.Errorf("ExtractInMemory: got %d entries, want 0", len(mem.Data))
	}

	// FromFile.Lookup must not stack-overflow.
	ff, err := ExtractFromFile(w, rootRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ff.Lookup(pdf.Integer(42)); err != ErrKeyNotFound {
		t.Errorf("FromFile.Lookup: err = %v, want ErrKeyNotFound", err)
	}

	// FromFile.All must not stack-overflow.
	count := 0
	for range ff.All() {
		count++
	}
	if count != 0 {
		t.Errorf("FromFile.All: yielded %d entries, want 0", count)
	}

	// Size must not stack-overflow.
	n, err := Size(w, rootRef)
	if err != nil {
		t.Errorf("Size: unexpected error %v", err)
	}
	if n != 0 {
		t.Errorf("Size: got %d, want 0", n)
	}
}

// TestKidsMutualCycle verifies that two nodes whose /Kids point at each
// other do not cause unbounded recursion. A leaf hanging off node A
// before the cycle is still surfaced.
func TestKidsMutualCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	aRef := w.Alloc()
	bRef := w.Alloc()
	leafRef := w.Alloc()

	leaf := pdf.Dict{
		"Nums": pdf.Array{
			pdf.Integer(1), pdf.Name("one"),
			pdf.Integer(2), pdf.Name("two"),
		},
		"Limits": pdf.Array{pdf.Integer(1), pdf.Integer(2)},
	}
	if err := w.Put(leafRef, leaf); err != nil {
		t.Fatal(err)
	}

	a := pdf.Dict{
		"Kids":   pdf.Array{leafRef, bRef},
		"Limits": pdf.Array{pdf.Integer(0), pdf.Integer(1000)},
	}
	if err := w.Put(aRef, a); err != nil {
		t.Fatal(err)
	}
	b := pdf.Dict{
		"Kids":   pdf.Array{aRef},
		"Limits": pdf.Array{pdf.Integer(0), pdf.Integer(1000)},
	}
	if err := w.Put(bRef, b); err != nil {
		t.Fatal(err)
	}

	// ExtractInMemory: must terminate, must surface the two leaf entries.
	mem, err := ExtractInMemory(w, aRef)
	if err != nil {
		t.Errorf("ExtractInMemory: unexpected error %v", err)
	}
	if mem == nil || len(mem.Data) != 2 {
		gotLen := -1
		if mem != nil {
			gotLen = len(mem.Data)
		}
		t.Errorf("ExtractInMemory: got %d entries, want 2", gotLen)
	}
	if mem != nil {
		if got, ok := mem.Data[1]; !ok || got != pdf.Name("one") {
			t.Errorf("Data[1] = %v, want one (ok=%v)", got, ok)
		}
		if got, ok := mem.Data[2]; !ok || got != pdf.Name("two") {
			t.Errorf("Data[2] = %v, want two (ok=%v)", got, ok)
		}
	}

	// FromFile.Lookup: must find a value from the leaf before the cycle.
	ff, err := ExtractFromFile(w, aRef)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ff.Lookup(pdf.Integer(1))
	if err != nil {
		t.Errorf("FromFile.Lookup(1): %v", err)
	}
	if got != pdf.Name("one") {
		t.Errorf("FromFile.Lookup(1) = %v, want one", got)
	}

	// FromFile.All: must terminate, must yield two entries.
	count := 0
	for range ff.All() {
		count++
	}
	if count != 2 {
		t.Errorf("FromFile.All: yielded %d entries, want 2", count)
	}

	// Size: must terminate; partial count of 2 is the documented behaviour.
	n, err := Size(w, aRef)
	if err != nil {
		t.Errorf("Size: unexpected error %v", err)
	}
	if n != 2 {
		t.Errorf("Size: got %d, want 2", n)
	}
}

// TestKidsLongChainNoCycle is a regression test that the new cycle
// plumbing does not break legitimate, deep but acyclic number trees.
func TestKidsLongChainNoCycle(t *testing.T) {
	const depth = 50

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	leafRef := w.Alloc()
	leaf := pdf.Dict{
		"Nums": pdf.Array{
			pdf.Integer(7), pdf.Name("seven"),
		},
		"Limits": pdf.Array{pdf.Integer(7), pdf.Integer(7)},
	}
	if err := w.Put(leafRef, leaf); err != nil {
		t.Fatal(err)
	}

	cur := leafRef
	for range depth {
		next := w.Alloc()
		node := pdf.Dict{
			"Kids":   pdf.Array{cur},
			"Limits": pdf.Array{pdf.Integer(7), pdf.Integer(7)},
		}
		if err := w.Put(next, node); err != nil {
			t.Fatal(err)
		}
		cur = next
	}

	mem, err := ExtractInMemory(w, cur)
	if err != nil {
		t.Fatalf("ExtractInMemory: %v", err)
	}
	if got, ok := mem.Data[7]; !ok || got != pdf.Name("seven") {
		t.Errorf("Data[7] = %v, want seven (ok=%v)", got, ok)
	}

	ff, err := ExtractFromFile(w, cur)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ff.Lookup(pdf.Integer(7))
	if err != nil {
		t.Errorf("FromFile.Lookup: %v", err)
	}
	if got != pdf.Name("seven") {
		t.Errorf("FromFile.Lookup = %v, want seven", got)
	}
}
