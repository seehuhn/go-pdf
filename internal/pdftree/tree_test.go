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
	"fmt"
	"iter"
	"slices"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// treeKind bundles a codec with the helpers the generic tree tests need to
// fabricate keys, values and an object that is not a valid key for that codec.
// The two instances drive every test against both name trees and number trees.
type treeKind[K cmp.Ordered, C codec[K]] struct {
	name   string
	keyAt  func(i int) K          // strictly increasing in i
	valAt  func(i int) pdf.Object // value for the i-th entry
	badKey pdf.Object             // an object that codec C cannot decode as a key
}

var nameKind = treeKind[pdf.Name, NameCodec]{
	name:   "name",
	keyAt:  func(i int) pdf.Name { return pdf.Name(fmt.Sprintf("key%06d", i)) },
	valAt:  func(i int) pdf.Object { return pdf.Integer(i) },
	badKey: pdf.Integer(0),
}

var numKind = treeKind[pdf.Integer, NumCodec]{
	name:   "num",
	keyAt:  func(i int) pdf.Integer { return pdf.Integer(i - 3) }, // spans negatives and zero
	valAt:  func(i int) pdf.Object { return pdf.Integer(i) },
	badKey: pdf.String("not an integer"),
}

// data returns a map with the first n entries of this kind.
func (tk treeKind[K, C]) data(n int) map[K]pdf.Object {
	m := make(map[K]pdf.Object, n)
	for i := range n {
		m[tk.keyAt(i)] = tk.valAt(i)
	}
	return m
}

// collect drains a key-value iterator into a map and the key visitation order.
func collect[K cmp.Ordered](seq iter.Seq2[K, pdf.Object]) (map[K]pdf.Object, []K) {
	m := map[K]pdf.Object{}
	var order []K
	for k, v := range seq {
		m[k] = v
		order = append(order, k)
	}
	return m, order
}

func put(t *testing.T, w *pdf.Writer, ref pdf.Reference, d pdf.Dict) {
	t.Helper()
	if err := w.Put(ref, d); err != nil {
		t.Fatal(err)
	}
}

func TestRoundTrip(t *testing.T) {
	t.Run("name", func(t *testing.T) { testRoundTrip(t, nameKind) })
	t.Run("num", func(t *testing.T) { testRoundTrip(t, numKind) })
}

// testRoundTrip writes trees of every structural shape and checks that both
// readers reproduce the data.  The sizes exercise each writer path: a single
// root leaf, a full leaf promoted to a root, a two-level tree, and a tree large
// enough to force intermediate-node merging.
func testRoundTrip[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C]) {
	sizes := []int{1, maxChildren - 1, maxChildren, maxChildren + 1, maxChildren*maxChildren + 1}
	for _, n := range sizes {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			if n >= maxChildren*maxChildren && testing.Short() {
				t.Skip("large tree skipped in short mode")
			}
			want := tk.data(n)

			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			ref, err := Write[K, C](w, (&InMemory[K, C]{Data: want}).All())
			if err != nil {
				t.Fatal(err)
			}
			if ref == 0 {
				t.Fatal("Write returned the null reference")
			}

			// ExtractInMemory must reproduce the data exactly.
			mem, err := ExtractInMemory[K, C](w, ref)
			if err != nil {
				t.Fatal(err)
			}
			if diff := gocmp.Diff(want, mem.Data); diff != "" {
				t.Errorf("ExtractInMemory round trip (-want +got):\n%s", diff)
			}

			// ExtractFromFile must yield every entry, in sorted order.
			ff, err := ExtractFromFile[K, C](w, ref)
			if err != nil {
				t.Fatal(err)
			}
			got, order := collect(ff.All())
			if diff := gocmp.Diff(want, got); diff != "" {
				t.Errorf("FromFile.All round trip (-want +got):\n%s", diff)
			}
			if !slices.IsSorted(order) {
				t.Error("FromFile.All did not yield keys in sorted order")
			}

			// Lookup must find sampled keys and report an absent key.
			for _, i := range []int{0, n / 2, n - 1} {
				v, err := ff.Lookup(tk.keyAt(i))
				if err != nil {
					t.Errorf("Lookup(%v): %v", tk.keyAt(i), err)
				} else if v != tk.valAt(i) {
					t.Errorf("Lookup(%v) = %v, want %v", tk.keyAt(i), v, tk.valAt(i))
				}
			}
			if _, err := ff.Lookup(tk.keyAt(n + 100)); err != ErrKeyNotFound {
				t.Errorf("Lookup(absent) = %v, want ErrKeyNotFound", err)
			}

			// Size must agree with the entry count.
			sz, err := Size[K, C](w, ref)
			if err != nil {
				t.Fatal(err)
			}
			if sz != n {
				t.Errorf("Size = %d, want %d", sz, n)
			}
		})
	}
}

func TestInMemory(t *testing.T) {
	t.Run("name", func(t *testing.T) { testInMemory(t, nameKind) })
	t.Run("num", func(t *testing.T) { testInMemory(t, numKind) })
}

func testInMemory[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C]) {
	want := tk.data(5)
	tree := &InMemory[K, C]{Data: want}

	for i := range 5 {
		v, err := tree.Lookup(tk.keyAt(i))
		if err != nil {
			t.Errorf("Lookup(%v): %v", tk.keyAt(i), err)
		} else if v != tk.valAt(i) {
			t.Errorf("Lookup(%v) = %v, want %v", tk.keyAt(i), v, tk.valAt(i))
		}
	}
	if _, err := tree.Lookup(tk.keyAt(100)); err != ErrKeyNotFound {
		t.Errorf("Lookup(absent) = %v, want ErrKeyNotFound", err)
	}

	got, order := collect(tree.All())
	if diff := gocmp.Diff(want, got); diff != "" {
		t.Errorf("All (-want +got):\n%s", diff)
	}
	if !slices.IsSorted(order) {
		t.Error("All did not yield keys in sorted order")
	}
}

func TestNilTree(t *testing.T) {
	t.Run("name", func(t *testing.T) { testNilTree(t, nameKind) })
	t.Run("num", func(t *testing.T) { testNilTree(t, numKind) })
}

func testNilTree[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C]) {
	// a nil root yields a nil tree from both extractors
	if got, err := ExtractInMemory[K, C](nil, nil); err != nil || got != nil {
		t.Errorf("ExtractInMemory(nil) = %v, %v", got, err)
	}
	if got, err := ExtractFromFile[K, C](nil, nil); err != nil || got != nil {
		t.Errorf("ExtractFromFile(nil) = %v, %v", got, err)
	}

	// nil tree values must behave like empty trees, not panic
	var mem *InMemory[K, C]
	if _, err := mem.Lookup(tk.keyAt(0)); err != ErrKeyNotFound {
		t.Errorf("nil InMemory.Lookup = %v, want ErrKeyNotFound", err)
	}
	for range mem.All() {
		t.Error("nil InMemory.All yielded an entry")
	}

	var ff *FromFile[K, C]
	if _, err := ff.Lookup(tk.keyAt(0)); err != ErrKeyNotFound {
		t.Errorf("nil FromFile.Lookup = %v, want ErrKeyNotFound", err)
	}
	for range ff.All() {
		t.Error("nil FromFile.All yielded an entry")
	}
}

func TestEmptyTree(t *testing.T) {
	t.Run("name", func(t *testing.T) { testEmptyTree(t, nameKind) })
	t.Run("num", func(t *testing.T) { testEmptyTree(t, numKind) })
}

// testEmptyTree pins the contract for writing an empty tree: it produces the
// null reference rather than a root object.
func testEmptyTree[K cmp.Ordered, C codec[K]](t *testing.T, _ treeKind[K, C]) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref, err := Write[K, C](w, (&InMemory[K, C]{Data: map[K]pdf.Object{}}).All())
	if err != nil {
		t.Fatal(err)
	}
	if ref != 0 {
		t.Errorf("Write(empty) = %v, want the null reference", ref)
	}
}

func TestWriteRejectsBadInput(t *testing.T) {
	t.Run("name", func(t *testing.T) { testWriteRejectsBadInput(t, nameKind) })
	t.Run("num", func(t *testing.T) { testWriteRejectsBadInput(t, numKind) })
}

func testWriteRejectsBadInput[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C]) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	unsorted := func(yield func(K, pdf.Object) bool) {
		if !yield(tk.keyAt(2), tk.valAt(2)) {
			return
		}
		yield(tk.keyAt(1), tk.valAt(1)) // out of order
	}
	if _, err := Write[K, C](w, unsorted); err == nil {
		t.Error("Write accepted unsorted keys")
	}

	duplicate := func(yield func(K, pdf.Object) bool) {
		if !yield(tk.keyAt(1), tk.valAt(1)) {
			return
		}
		yield(tk.keyAt(1), tk.valAt(2)) // duplicate
	}
	if _, err := Write[K, C](w, duplicate); err == nil {
		t.Error("Write accepted duplicate keys")
	}
}

func TestEmbed(t *testing.T) {
	t.Run("name", func(t *testing.T) { testEmbed(t, nameKind) })
	t.Run("num", func(t *testing.T) { testEmbed(t, numKind) })
}

// testEmbed checks that both tree representations embed through the resource
// manager and read back unchanged.
func testEmbed[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C]) {
	want := tk.data(5)

	t.Run("InMemory", func(t *testing.T) {
		w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		ref := embed(t, w, &InMemory[K, C]{Data: want})
		checkContents[K, C](t, w, ref, want)
	})

	t.Run("FromFile", func(t *testing.T) {
		w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
		srcRef, err := Write[K, C](w, (&InMemory[K, C]{Data: want}).All())
		if err != nil {
			t.Fatal(err)
		}
		ff, err := ExtractFromFile[K, C](w, srcRef)
		if err != nil {
			t.Fatal(err)
		}
		ref := embed(t, w, ff)
		checkContents[K, C](t, w, ref, want)
	})
}

func embed(t *testing.T, w *pdf.Writer, e pdf.Embedder) pdf.Object {
	t.Helper()
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(e)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	return ref
}

func checkContents[K cmp.Ordered, C codec[K]](t *testing.T, r pdf.Getter, root pdf.Object, want map[K]pdf.Object) {
	t.Helper()
	mem, err := ExtractInMemory[K, C](r, root)
	if err != nil {
		t.Fatal(err)
	}
	if diff := gocmp.Diff(want, mem.Data); diff != "" {
		t.Errorf("embedded tree (-want +got):\n%s", diff)
	}
}

func TestEarlyTermination(t *testing.T) {
	t.Run("name", func(t *testing.T) { testEarlyTermination(t, nameKind) })
	t.Run("num", func(t *testing.T) { testEarlyTermination(t, numKind) })
}

// testEarlyTermination checks that breaking out of an All iterator stops the
// walk, including across the intermediate nodes of a multi-leaf tree.
func testEarlyTermination[K cmp.Ordered, C codec[K]](t *testing.T, tk treeKind[K, C]) {
	want := tk.data(maxChildren + 5) // forces a multi-leaf tree with /Kids
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref, err := Write[K, C](w, (&InMemory[K, C]{Data: want}).All())
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		all  func() iter.Seq2[K, pdf.Object]
	}{
		{"InMemory", func() iter.Seq2[K, pdf.Object] {
			m, _ := ExtractInMemory[K, C](w, ref)
			return m.All()
		}},
		{"FromFile", func() iter.Seq2[K, pdf.Object] {
			f, _ := ExtractFromFile[K, C](w, ref)
			return f.All()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seen := 0
			for range tc.all() {
				seen++
				break
			}
			if seen != 1 {
				t.Errorf("iterated %d entries after break, want 1", seen)
			}
		})
	}
}
