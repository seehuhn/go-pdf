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
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/page"
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
		w, err := pdf.NewWriter(buf, pdf.V1_7, nil)
		if err != nil {
			t.Fatal(err)
		}

		rm := pdf.NewResourceManager(w)
		s := NewWriter(w, rm)

		for i := 0; i < numPages; i++ {
			p := &page.Page{
				MediaBox: &pdf.Rectangle{URx: 100, URy: 100},
			}
			err := s.AppendPage(p)
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

		w.GetMeta().Catalog.Pages = rootRef // pretend we have pages
		err = rm.Close()
		if err != nil {
			t.Fatal(err)
		}
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

		total, err := walkPages(r, r.GetMeta().Catalog.Pages, 0)
		if err != nil {
			t.Fatal(err)
		} else if total != numPages {
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
	w, err := pdf.NewWriter(buf, pdf.V1_7, nil)
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

// walkPages walks the page tree and counts pages, verifying structure.
func walkPages(r pdf.Getter, nodeRef, parentRef pdf.Reference) (int, error) {
	node, err := pdf.Resolve(r, nodeRef)
	if err != nil {
		return 0, err
	}
	dict, ok := node.(pdf.Dict)
	if !ok {
		return 0, fmt.Errorf("not a dict: %T", node)
	}
	var total int
	switch dict["Type"] {
	case pdf.Name("Page"):
		total++
	case pdf.Name("Pages"):
		kids := dict["Kids"]
		kidsArray, ok := kids.(pdf.Array)
		if !ok {
			return 0, fmt.Errorf("not an array: %T", kids)
		}
		var subTotal int
		for _, kid := range kidsArray {
			kidRef, ok := kid.(pdf.Reference)
			if !ok {
				return 0, fmt.Errorf("not a reference: %T", kid)
			}
			count, err := walkPages(r, kidRef, nodeRef)
			if err != nil {
				return 0, err
			}
			subTotal += count
		}
		if x, ok := dict["Count"].(pdf.Integer); !ok || int(x) != subTotal {
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

func TestInheritRotate(t *testing.T) {
	n := maxDegree
	cc := make([]*nodeInfo, n)
	for i := 0; i < n; i++ {
		dict := pdf.Dict{}
		switch i {
		case 0:
			dict["Rotate"] = pdf.Integer(0)
		case 1:
			dict["Rotate"] = pdf.Integer(90)
		default:
			dict["Rotate"] = pdf.Integer(180)
		}
		cc[i] = &nodeInfo{dictInfo: &dictInfo{dict: dict}}
	}
	parentDict := pdf.Dict{}
	inheritRotate(parentDict, cc)
	fmt.Println("parent:", pdf.AsString(parentDict))
	for i := 0; i < n; i++ {
		fmt.Printf("child %d: %s\n", i, pdf.AsString(cc[i].dict))
	}
}

func FuzzInherit(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3})

	// Pre-define some MediaBox and CropBox values for testing inheritance
	mediaBoxes := []*pdf.Rectangle{
		{URx: 100, URy: 100},
		{URx: 200, URy: 200},
		{URx: 300, URy: 300},
	}
	cropBoxes := []*pdf.Rectangle{
		nil, // no CropBox
		{LLx: 10, LLy: 10, URx: 90, URy: 90},
		{LLx: 20, LLy: 20, URx: 180, URy: 180},
	}
	rotations := []page.Rotation{
		page.RotateInherit,
		page.Rotate0,
		page.Rotate90,
		page.Rotate180,
		page.Rotate270,
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		// Build expected values from fuzz data
		expectedMediaBox := make([]*pdf.Rectangle, len(data))
		expectedCropBox := make([]*pdf.Rectangle, len(data))
		expectedRotate := make([]page.Rotation, len(data))

		for i, c := range data {
			expectedMediaBox[i] = mediaBoxes[int(c&3)%len(mediaBoxes)]
			expectedCropBox[i] = cropBoxes[int((c>>2)&3)%len(cropBoxes)]
			expectedRotate[i] = rotations[int((c>>4)&7)%len(rotations)]
		}

		doc, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
		rm := pdf.NewResourceManager(doc)
		pp := NewWriter(doc, rm)

		for i := range data {
			p := &page.Page{
				MediaBox: expectedMediaBox[i],
				CropBox:  expectedCropBox[i],
				Rotate:   expectedRotate[i],
			}
			err := pp.AppendPage(p)
			if err != nil {
				t.Fatal(err)
			}
		}
		rootRef, err := pp.Close()
		if err != nil {
			t.Fatal(err)
		}
		doc.GetMeta().Catalog.Pages = rootRef
		err = rm.Close()
		if err != nil {
			t.Fatal(err)
		}

		// Verify the values are preserved after inheritance optimization
		for i := range data {
			_, dict, err := GetPage(doc, i)
			if err != nil {
				t.Fatal(err)
			}

			// Check MediaBox
			gotMediaBox, err := pdf.GetRectangle(doc, dict["MediaBox"])
			if err != nil {
				t.Fatalf("page %d: failed to get MediaBox: %v", i, err)
			}
			if *gotMediaBox != *expectedMediaBox[i] {
				t.Errorf("page %d: wrong MediaBox: got %v, expected %v", i, gotMediaBox, expectedMediaBox[i])
			}

			// Check CropBox
			if expectedCropBox[i] != nil {
				gotCropBox, err := pdf.GetRectangle(doc, dict["CropBox"])
				if err != nil {
					t.Fatalf("page %d: failed to get CropBox: %v", i, err)
				}
				if *gotCropBox != *expectedCropBox[i] {
					t.Errorf("page %d: wrong CropBox: got %v, expected %v", i, gotCropBox, expectedCropBox[i])
				}
			}

			// Check Rotate - account for inheritance (RotateInherit means 0 degrees)
			rotate, _ := pdf.GetInteger(doc, dict["Rotate"])
			expectedDegrees := expectedRotate[i].Degrees()
			if int(rotate) != expectedDegrees {
				t.Errorf("page %d: wrong Rotate: got %d, expected %d", i, rotate, expectedDegrees)
			}
		}
	})
}
