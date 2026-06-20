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

func TestMalformedLeaf(t *testing.T) {
	t.Run("name", func(t *testing.T) { testMalformedLeaf(t, nameKind) })
	t.Run("num", func(t *testing.T) { testMalformedLeaf(t, numKind) })
}

// testMalformedLeaf checks that the readers tolerate a leaf whose entry array
// is malformed -- an odd length (a key with no value) or an entry with the
// wrong key type.  Malformed entries are dropped, well-formed ones preserved,
// and no reader panics.
func testMalformedLeaf[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C]) {
	var kc C
	good := tk.keyAt(1)
	goodVal := tk.valAt(1)

	cases := []struct {
		name    string
		entries pdf.Array
		want    int // entries expected to survive
		hasGood bool
	}{
		{
			name:    "lone key",
			entries: pdf.Array{kc.encode(tk.keyAt(0))},
			want:    0,
		},
		{
			name:    "trailing key",
			entries: pdf.Array{kc.encode(good), goodVal, kc.encode(tk.keyAt(2))},
			want:    1,
			hasGood: true,
		},
		{
			name:    "wrong key type",
			entries: pdf.Array{tk.badKey, tk.valAt(0), kc.encode(good), goodVal},
			want:    1,
			hasGood: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			ref := w.Alloc()
			put(t, w, ref, pdf.Dict{kc.leafKey(): tc.entries})

			mem, err := ExtractInMemory[K, C](w, ref)
			if err != nil {
				t.Errorf("ExtractInMemory: %v", err)
			}
			gotN := -1
			if mem != nil {
				gotN = len(mem.Data)
			}
			if gotN != tc.want {
				t.Errorf("ExtractInMemory: got %d entries, want %d", gotN, tc.want)
			}

			ff, err := ExtractFromFile[K, C](w, ref)
			if err != nil {
				t.Fatal(err)
			}
			if _, order := collect(ff.All()); len(order) != tc.want {
				t.Errorf("FromFile.All: got %d entries, want %d", len(order), tc.want)
			}

			if sz, err := Size[K, C](w, ref); err != nil || sz != tc.want {
				t.Errorf("Size: got %d, %v; want %d", sz, err, tc.want)
			}

			if tc.hasGood {
				v, err := ff.Lookup(good)
				if err != nil || v != goodVal {
					t.Errorf("Lookup(good) = %v, %v; want %v", v, err, goodVal)
				}
			}
		})
	}
}

func TestMalformedIntermediate(t *testing.T) {
	t.Run("name", func(t *testing.T) { testMalformedIntermediate(t, nameKind) })
	t.Run("num", func(t *testing.T) { testMalformedIntermediate(t, numKind) })
}

// testMalformedIntermediate checks Lookup's handling of children with missing
// or malformed Limits.  Such children are skipped during the Limits-guided
// descent, so Lookup misses keys that the Limits-ignoring enumeration still
// surfaces.  A /Kids entry that does not resolve to a dictionary is skipped by
// both paths.
func testMalformedIntermediate[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C]) {
	var kc C
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	enc := func(i int) pdf.Object { return kc.encode(tk.keyAt(i)) }

	// k0 is reachable via good Limits; k1..k3 only via enumeration
	good := w.Alloc()
	put(t, w, good, pdf.Dict{kc.leafKey(): pdf.Array{enc(0), tk.valAt(0)}, "Limits": pdf.Array{enc(0), enc(0)}})
	noLimits := w.Alloc()
	put(t, w, noLimits, pdf.Dict{kc.leafKey(): pdf.Array{enc(1), tk.valAt(1)}})
	shortLimits := w.Alloc()
	put(t, w, shortLimits, pdf.Dict{kc.leafKey(): pdf.Array{enc(2), tk.valAt(2)}, "Limits": pdf.Array{enc(2)}})
	badMin := w.Alloc()
	put(t, w, badMin, pdf.Dict{kc.leafKey(): pdf.Array{enc(3), tk.valAt(3)}, "Limits": pdf.Array{tk.badKey, tk.badKey}})
	badMax := w.Alloc()
	put(t, w, badMax, pdf.Dict{kc.leafKey(): pdf.Array{enc(4), tk.valAt(4)}, "Limits": pdf.Array{enc(4), tk.badKey}})

	root := w.Alloc()
	put(t, w, root, pdf.Dict{
		"Kids":   pdf.Array{good, noLimits, shortLimits, badMin, badMax, pdf.Integer(99)}, // last entry is not a dict
		"Limits": pdf.Array{enc(0), enc(4)},
	})

	want := tk.data(5) // every well-formed leaf, regardless of Limits

	// the Limits-ignoring readers surface every well-formed leaf
	mem, err := ExtractInMemory[K, C](w, root)
	if err != nil {
		t.Fatal(err)
	}
	if mem == nil || len(mem.Data) != len(want) {
		t.Errorf("ExtractInMemory: got %v, want %d entries", mem, len(want))
	}

	ff, err := ExtractFromFile[K, C](w, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, order := collect(ff.All()); len(order) != len(want) {
		t.Errorf("All yielded %d entries, want %d", len(order), len(want))
	}
	if n, err := Size[K, C](w, root); err != nil || n != len(want) {
		t.Errorf("Size = %d, %v; want %d", n, err, len(want))
	}

	// Lookup follows Limits: only the well-formed child is reachable
	if v, err := ff.Lookup(tk.keyAt(0)); err != nil || v != tk.valAt(0) {
		t.Errorf("Lookup(k0) = %v, %v; want %v", v, err, tk.valAt(0))
	}
	for _, i := range []int{1, 2, 3, 4} {
		if _, err := ff.Lookup(tk.keyAt(i)); err != ErrKeyNotFound {
			t.Errorf("Lookup(k%d) = %v, want ErrKeyNotFound", i, err)
		}
	}
}

func TestMalformedRoot(t *testing.T) {
	t.Run("name", func(t *testing.T) { testMalformedRoot(t, nameKind) })
	t.Run("num", func(t *testing.T) { testMalformedRoot(t, numKind) })
}

// testMalformedRoot checks that a root object which is not a dictionary yields
// an empty tree from every reader rather than a panic.
func testMalformedRoot[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C]) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Integer(7)); err != nil {
		t.Fatal(err)
	}

	mem, _ := ExtractInMemory[K, C](w, ref)
	if mem != nil && len(mem.Data) != 0 {
		t.Errorf("ExtractInMemory: got %d entries, want 0", len(mem.Data))
	}

	ff, err := ExtractFromFile[K, C](w, ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ff.Lookup(tk.keyAt(0)); err != ErrKeyNotFound {
		t.Errorf("Lookup = %v, want ErrKeyNotFound", err)
	}
	if _, order := collect(ff.All()); len(order) != 0 {
		t.Errorf("All yielded %d entries, want 0", len(order))
	}
}

func TestMalformedContainers(t *testing.T) {
	t.Run("name", func(t *testing.T) { testMalformedContainers(t, nameKind) })
	t.Run("num", func(t *testing.T) { testMalformedContainers(t, numKind) })
}

// testMalformedContainers checks that a /Kids or leaf-entry value that is not
// an array is treated as an empty node rather than causing a panic.
func testMalformedContainers[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C]) {
	var kc C
	cases := []struct {
		name string
		dict pdf.Dict
	}{
		{"kids not array", pdf.Dict{"Kids": pdf.Integer(5)}},
		{"entries not array", pdf.Dict{kc.leafKey(): pdf.Integer(5)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			ref := w.Alloc()
			put(t, w, ref, tc.dict)

			if mem, err := ExtractInMemory[K, C](w, ref); err != nil || mem == nil || len(mem.Data) != 0 {
				t.Errorf("ExtractInMemory = %v, %v", mem, err)
			}
			ff, err := ExtractFromFile[K, C](w, ref)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ff.Lookup(tk.keyAt(0)); err != ErrKeyNotFound {
				t.Errorf("Lookup = %v, want ErrKeyNotFound", err)
			}
			if _, order := collect(ff.All()); len(order) != 0 {
				t.Errorf("All yielded %d entries, want 0", len(order))
			}
		})
	}
}
