// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/tempfile"
)

func TestRead(t *testing.T) {
	tree1 := &numTree{
		Data: []numTreeNode{
			{Key: -1, Value: pdf.Name("negative one")},
			{Key: 2, Value: pdf.Name("two")},
			{Key: 5, Value: pdf.Name("five")},
		},
	}
	for i := pdf.Integer(100); i < 200; i++ {
		tree1.Data = append(tree1.Data, numTreeNode{Key: i, Value: pdf.Integer(i)})
	}

	w, _ := tempfile.NewTempWriter(pdf.V1_7, nil)
	ref, err := Write(w, tree1)
	if err != nil {
		t.Fatal(err)
	}

	tree2, err := Read(w, ref)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(tree1, tree2); d != "" {
		t.Errorf("Read() mismatch (-want +got):\n%s", d)
	}
}

func TestNumTree(t *testing.T) {
	tree := &numTree{
		Data: []numTreeNode{
			{Key: 1, Value: pdf.Name("one")},
			{Key: 2, Value: pdf.Name("two")},
			{Key: 5, Value: pdf.Name("five")},
		},
	}

	key, err := tree.First()
	if key != 1 || err != nil {
		t.Errorf("First() == (%d, %v), want (%d, %v)", key, err, 1, nil)
	}

	// test the "Get" method
	type getCase struct {
		key  pdf.Integer
		want pdf.Object
		err  error
	}
	getCases := []getCase{
		{key: 0, want: nil, err: ErrKeyNotFound},
		{key: 1, want: pdf.Name("one"), err: nil},
		{key: 2, want: pdf.Name("two"), err: nil},
		{key: 3, want: nil, err: ErrKeyNotFound},
		{key: 4, want: nil, err: ErrKeyNotFound},
		{key: 5, want: pdf.Name("five"), err: nil},
		{key: 6, want: nil, err: ErrKeyNotFound},
	}
	for _, c := range getCases {
		got, err := tree.Get(c.key)
		if got != c.want || err != c.err {
			t.Errorf("Get(%d) == (%v, %v), want (%v, %v)", c.key, got, err, c.want, c.err)
		}
	}

	// test the "Next" method
	type testCase struct {
		key  pdf.Integer
		want pdf.Integer
		err  error
	}
	nextCases := []testCase{
		{key: 0, want: 1, err: nil},
		{key: 1, want: 2, err: nil},
		{key: 2, want: 5, err: nil},
		{key: 3, want: 5, err: nil},
		{key: 4, want: 5, err: nil},
		{key: 5, want: 0, err: ErrKeyNotFound},
		{key: 6, want: 0, err: ErrKeyNotFound},
	}
	for _, c := range nextCases {
		got, err := tree.Next(c.key)
		if got != c.want || err != c.err {
			t.Errorf("Next(%d) == (%d, %v), want (%d, %v)", c.key, got, err, c.want, c.err)
		}
	}

	// test the "Prev" method
	prevCases := []testCase{
		{key: 0, want: 0, err: io.EOF},
		{key: 1, want: 1, err: nil},
		{key: 2, want: 2, err: nil},
		{key: 3, want: 2, err: nil},
		{key: 4, want: 2, err: nil},
		{key: 5, want: 5, err: nil},
		{key: 6, want: 5, err: nil},
	}
	for _, c := range prevCases {
		got, err := tree.Prev(c.key)
		if got != c.want || err != c.err {
			t.Errorf("Prev(%d) == (%d, %v), want (%d, %v)", c.key, got, err, c.want, c.err)
		}
	}
}
