// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package pagetree

import (
	"bytes"
	"fmt"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestSubtree(t *testing.T) {
	testCases := []int{
		1, 2, 3, 10, 100, 1000,
		maxDegree - 1, maxDegree, maxDegree + 1,
		maxDegree*maxDegree - 1, maxDegree * maxDegree, maxDegree*maxDegree + 1,
	}

	for _, numPages := range testCases {
		// stage 1: write the tree to a file and check invariants in the process.

		buf := &bytes.Buffer{}
		w, err := pdf.NewWriter(buf, nil)
		if err != nil {
			t.Fatal(err)
		}

		s := &Writer{
			Out:            w,
			nextPageNumber: &futureInt{},
		}
		for i := 0; i < numPages; i++ {
			pageDict := pdf.Dict{
				"Type": pdf.Name("Page"),
				"Test": pdf.Integer(i),
			}
			err := s.AppendPage(pageDict)
			if err != nil {
				t.Fatal(err)
			}

			err = checkInvariants(s.tail)
			if err != nil {
				t.Error(err)
			}
		}

		rootRef, err := s.Close()
		if err != nil {
			t.Fatal(err)
		}

		w.Catalog.Pages = rootRef // pretend we have pages
		err = w.Close()
		if err != nil {
			t.Fatal(err)
		}

		// stage 2: Read back the file and check the page tree

		body := buf.Bytes()
		r, err := pdf.NewReader(bytes.NewReader(body), nil)
		if err != nil {
			t.Fatal(err)
		}

		test := pdf.Integer(0)
		total, err := walk(r, r.Catalog.Pages, 0, &test)
		if err != nil {
			t.Fatal(err)
		} else if total != pdf.Integer(numPages) {
			t.Errorf("total pages: %d != %d", total, numPages)
		}

		err = r.Close()
		if err != nil {
			t.Error(err)
		}
	}
}

func TestMerge(t *testing.T) {
	type testCase struct {
		a, b []int
	}
	cases := []testCase{
		{
			a: []int{2},
			b: []int{1},
		},
		{
			a: []int{1},
			b: []int{2},
		},
		{
			a: []int{1, 1},
			b: []int{2},
		},
		{
			a: []int{1, 1},
			b: []int{3},
		},
		{
			a: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, // 15
			b: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, // 15
		},
		{
			a: []int{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, // 15
				1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, // 15
			b: []int{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}, // 15
		},
	}

	buf := &bytes.Buffer{}
	w, err := pdf.NewWriter(buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	tree := &Writer{
		Out: w,
	}

	dd := &dictInfo{
		dict: pdf.Dict{},
	}
	for _, test := range cases {
		var a, b []*nodeInfo
		for _, depth := range test.a {
			a = append(a, &nodeInfo{dictInfo: dd, depth: depth})
		}
		for _, depth := range test.b {
			b = append(b, &nodeInfo{dictInfo: dd, depth: depth})
		}
		out := tree.merge(a, b)
		err = checkInvariants(out)
		if err != nil {
			t.Error(err)
		}
	}
}

func checkInvariants(nodes []*nodeInfo) error {
	lastDepth := nodes[0].depth + 1
	numAtDepth := 0
	for _, node := range nodes {
		if node.depth > lastDepth {
			return fmt.Errorf("depth %d > %d", node.depth, lastDepth)
		} else if node.depth == lastDepth {
			numAtDepth++
			if numAtDepth >= maxDegree {
				return fmt.Errorf("too many nodes at depth %d", lastDepth)
			}
		} else { // node.depth < lastDepth
			numAtDepth = 0
			lastDepth = node.depth
		}
	}
	return nil
}

func walk(r *pdf.Reader, nodeRef, parentRef pdf.Reference, test *pdf.Integer) (pdf.Integer, error) {
	node, err := pdf.Resolve(r, nodeRef)
	if err != nil {
		return 0, err
	}
	dict, ok := node.(pdf.Dict)
	if !ok {
		return 0, fmt.Errorf("not a dict: %T", node)
	}
	var total pdf.Integer
	switch dict["Type"] {
	case pdf.Name("Page"):
		total++
		x := dict["Test"]
		if x != *test {
			return 0, fmt.Errorf("wrong /Test: expected %d but got %s",
				*test, x)
		}
		*test++
	case pdf.Name("Pages"):
		kids := dict["Kids"]
		kidsArray, ok := kids.(pdf.Array)
		if !ok {
			return 0, fmt.Errorf("not an array: %T", kids)
		}
		var subTotal pdf.Integer
		for _, kid := range kidsArray {
			kidRef, ok := kid.(pdf.Reference)
			if !ok {
				return 0, fmt.Errorf("not a reference: %T", kid)
			}
			count, err := walk(r, kidRef, nodeRef, test)
			if err != nil {
				return 0, err
			}
			subTotal += count
		}
		if x, ok := dict["Count"].(pdf.Integer); !ok || x != subTotal {
			return 0, fmt.Errorf("wrong /Count: expected %d but got %s",
				subTotal, dict["Count"])
		}
		total += subTotal
	default:
		return 0, fmt.Errorf("unknown type: %v", dict["Type"])
	}
	if parentRef != 0 {
		want := dict["Parent"].(pdf.Reference)
		if want != parentRef {
			return 0, fmt.Errorf("parent mismatch: %s != %s", want, parentRef)
		}
	} else {
		if _, hasParent := dict["Parent"]; hasParent {
			return 0, fmt.Errorf("root has parent")
		}
		if dict["Type"] != pdf.Name("Pages") {
			return 0, fmt.Errorf("root is not a Pages node")
		}
	}
	return total, nil
}
