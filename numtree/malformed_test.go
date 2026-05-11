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

// TestMalformedLeaf verifies that the readers tolerate a leaf node whose
// Nums array is malformed: either of odd length (a trailing key with no
// value) or containing an entry with the wrong key type.  The readers must
// not panic; malformed entries are silently dropped, well-formed entries
// preserved.
func TestMalformedLeaf(t *testing.T) {
	cases := []struct {
		name      string
		nums      pdf.Array
		wantPairs int         // expected count from all read paths
		missing   pdf.Integer // key expected to be absent after extraction
		hasGood   bool        // true if a well-formed pair is expected
		goodKey   pdf.Integer
		goodValue pdf.Object
	}{
		{
			name:      "odd-length-1",
			nums:      pdf.Array{pdf.Integer(42)},
			wantPairs: 0,
			missing:   42,
		},
		{
			name: "odd-length-3",
			nums: pdf.Array{
				pdf.Integer(1), pdf.Name("first"),
				pdf.Integer(99),
			},
			wantPairs: 1,
			missing:   99,
			hasGood:   true,
			goodKey:   1,
			goodValue: pdf.Name("first"),
		},
		{
			name: "bad-key-type",
			nums: pdf.Array{
				pdf.Name("wrong"), pdf.Name("v1"),
				pdf.Integer(5), pdf.Name("v2"),
			},
			wantPairs: 1,
			missing:   999,
			hasGood:   true,
			goodKey:   5,
			goodValue: pdf.Name("v2"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
			ref := w.Alloc()
			node := pdf.Dict{"Nums": tc.nums}
			if err := w.Put(ref, node); err != nil {
				t.Fatal(err)
			}

			mem, err := ExtractInMemory(w, ref)
			if err != nil {
				t.Errorf("ExtractInMemory: %v", err)
			}
			gotMem := -1
			if mem != nil {
				gotMem = len(mem.Data)
			}
			if gotMem != tc.wantPairs {
				t.Errorf("ExtractInMemory: got %d pairs, want %d", gotMem, tc.wantPairs)
			}
			if tc.hasGood && mem != nil {
				if got, ok := mem.Data[tc.goodKey]; !ok || got != tc.goodValue {
					t.Errorf("InMemory[%d] = %v (ok=%v), want %v", tc.goodKey, got, ok, tc.goodValue)
				}
			}

			fromFile, err := ExtractFromFile(w, ref)
			if err != nil {
				t.Fatal(err)
			}

			count := 0
			for range fromFile.All() {
				count++
			}
			if count != tc.wantPairs {
				t.Errorf("FromFile.All: yielded %d pairs, want %d", count, tc.wantPairs)
			}

			size, err := Size(w, ref)
			if err != nil {
				t.Errorf("Size: %v", err)
			}
			if size != tc.wantPairs {
				t.Errorf("Size: got %d, want %d", size, tc.wantPairs)
			}

			if _, err := fromFile.Lookup(tc.missing); err != ErrKeyNotFound {
				t.Errorf("Lookup(%d) err = %v, want ErrKeyNotFound", tc.missing, err)
			}

			if tc.hasGood {
				got, err := fromFile.Lookup(tc.goodKey)
				if err != nil {
					t.Errorf("Lookup(%d): %v", tc.goodKey, err)
				}
				if got != tc.goodValue {
					t.Errorf("Lookup(%d) = %v, want %v", tc.goodKey, got, tc.goodValue)
				}
			}
		})
	}
}
