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

package pdftree

import (
	"cmp"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// leaf builds a leaf dictionary holding the given key-value pairs.
func leaf[K cmp.Ordered, C codec[K]](kc C, pairs ...any) pdf.Dict {
	entries := make(pdf.Array, 0, len(pairs))
	var lo, hi K
	for i := 0; i < len(pairs); i += 2 {
		k := pairs[i].(K)
		entries = append(entries, kc.encode(k), pairs[i+1].(pdf.Object))
		if i == 0 {
			lo = k
		}
		hi = k
	}
	return pdf.Dict{
		kc.leafKey(): entries,
		"Limits":     pdf.Array{kc.encode(lo), kc.encode(hi)},
	}
}

func TestKidsSelfCycle(t *testing.T) {
	t.Run("name", func(t *testing.T) { testKidsSelfCycle(t, nameKind) })
	t.Run("num", func(t *testing.T) { testKidsSelfCycle(t, numKind) })
}

// testKidsSelfCycle verifies that a node whose /Kids array references itself
// does not cause unbounded recursion in any reader.
func testKidsSelfCycle[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C]) {
	var kc C
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	root := w.Alloc()
	put(t, w, root, pdf.Dict{
		"Kids":   pdf.Array{root},
		"Limits": pdf.Array{kc.encode(tk.keyAt(0)), kc.encode(tk.keyAt(9))},
	})

	mem, err := ExtractInMemory[K, C](w, root)
	if err != nil {
		t.Errorf("ExtractInMemory: %v", err)
	}
	if mem != nil && len(mem.Data) != 0 {
		t.Errorf("ExtractInMemory: got %d entries, want 0", len(mem.Data))
	}

	ff, err := ExtractFromFile[K, C](w, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ff.Lookup(tk.keyAt(0)); err != ErrKeyNotFound {
		t.Errorf("FromFile.Lookup = %v, want ErrKeyNotFound", err)
	}
	if _, order := collect(ff.All()); len(order) != 0 {
		t.Errorf("FromFile.All yielded %d entries, want 0", len(order))
	}
	if n, err := Size[K, C](w, root); err != nil || n != 0 {
		t.Errorf("Size = %d, %v; want 0", n, err)
	}
}

func TestKidsMutualCycle(t *testing.T) {
	t.Run("name", func(t *testing.T) { testKidsMutualCycle(t, nameKind) })
	t.Run("num", func(t *testing.T) { testKidsMutualCycle(t, numKind) })
}

// testKidsMutualCycle verifies that two nodes whose /Kids point at each other
// terminate, while a leaf reachable before the cycle is still surfaced.
func testKidsMutualCycle[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C]) {
	var kc C
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	aRef := w.Alloc()
	bRef := w.Alloc()
	leafRef := w.Alloc()

	k0, k1, hi := tk.keyAt(0), tk.keyAt(1), tk.keyAt(9)
	v0, v1 := tk.valAt(0), tk.valAt(1)

	put(t, w, leafRef, leaf(kc, k0, v0, k1, v1))
	put(t, w, aRef, pdf.Dict{
		"Kids":   pdf.Array{leafRef, bRef},
		"Limits": pdf.Array{kc.encode(k0), kc.encode(hi)},
	})
	put(t, w, bRef, pdf.Dict{
		"Kids":   pdf.Array{aRef},
		"Limits": pdf.Array{kc.encode(k0), kc.encode(hi)},
	})

	mem, err := ExtractInMemory[K, C](w, aRef)
	if err != nil {
		t.Errorf("ExtractInMemory: %v", err)
	}
	want := map[K]pdf.Object{k0: v0, k1: v1}
	if mem == nil || len(mem.Data) != 2 || mem.Data[k0] != v0 || mem.Data[k1] != v1 {
		t.Errorf("ExtractInMemory = %v, want %v", mem, want)
	}

	ff, err := ExtractFromFile[K, C](w, aRef)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := ff.Lookup(k0); err != nil || v != v0 {
		t.Errorf("FromFile.Lookup(%v) = %v, %v; want %v", k0, v, err, v0)
	}
	if _, order := collect(ff.All()); len(order) != 2 {
		t.Errorf("FromFile.All yielded %d entries, want 2", len(order))
	}
	if n, err := Size[K, C](w, aRef); err != nil || n != 2 {
		t.Errorf("Size = %d, %v; want 2", n, err)
	}
}

func TestKidsLongChainNoCycle(t *testing.T) {
	t.Run("name", func(t *testing.T) { testKidsChain(t, nameKind, 50, false) })
	t.Run("num", func(t *testing.T) { testKidsChain(t, numKind, 50, false) })
}

func TestKidsDeepChainBounded(t *testing.T) {
	var kc NameCodec
	deep := kc.maxDepth() + 10
	t.Run("name", func(t *testing.T) { testKidsChain(t, nameKind, deep, true) })
	t.Run("num", func(t *testing.T) { testKidsChain(t, numKind, deep, true) })
}

// testKidsChain builds a single leaf wrapped in depth intermediate /Kids nodes.
// A chain within the depth cap must read back the leaf; a chain past the cap
// must be handled gracefully -- the enumerating readers silently truncate the
// over-deep subtree, and the streaming lookup reports a malformed file -- never
// a stack overflow.
func testKidsChain[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C], depth int, overCap bool) {
	var kc C
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	k, v := tk.keyAt(0), tk.valAt(0)

	cur := w.Alloc()
	put(t, w, cur, leaf(kc, k, v))
	for range depth {
		next := w.Alloc()
		put(t, w, next, pdf.Dict{
			"Kids":   pdf.Array{cur},
			"Limits": pdf.Array{kc.encode(k), kc.encode(k)},
		})
		cur = next
	}

	mem, err := ExtractInMemory[K, C](w, cur)
	if err != nil {
		t.Errorf("ExtractInMemory: %v", err)
	}
	ff, err := ExtractFromFile[K, C](w, cur)
	if err != nil {
		t.Fatal(err)
	}
	_, order := collect(ff.All())

	if overCap {
		if mem != nil && len(mem.Data) != 0 {
			t.Errorf("ExtractInMemory past cap: got %d entries, want 0", len(mem.Data))
		}
		if len(order) != 0 {
			t.Errorf("FromFile.All past cap: got %d entries, want 0", len(order))
		}
		if _, err := ff.Lookup(k); !pdf.IsMalformed(err) {
			t.Errorf("FromFile.Lookup past cap: err = %v, want malformed", err)
		}
		if _, err := Size[K, C](w, cur); err != nil {
			t.Errorf("Size past cap: %v", err)
		}
		return
	}

	if mem == nil || mem.Data[k] != v {
		t.Errorf("ExtractInMemory: %v[%v] != %v", mem, k, v)
	}
	if len(order) != 1 {
		t.Errorf("FromFile.All: got %d entries, want 1", len(order))
	}
	if got, err := ff.Lookup(k); err != nil || got != v {
		t.Errorf("FromFile.Lookup(%v) = %v, %v; want %v", k, got, err, v)
	}
}
