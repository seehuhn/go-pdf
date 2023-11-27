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

package outline

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
)

func printTree(node *Tree, pfx string) {
	if node == nil {
		fmt.Printf("%snull\n", pfx)
		return
	}
	if node.Title != "" {
		fmt.Printf("%s%s (%t) %s\n", pfx, node.Title, node.Open, node.Action)
	} else {
		fmt.Printf("%s<no title> (%t) %s\n", pfx, node.Open, node.Action)
	}
	for _, c := range node.Children {
		printTree(c, pfx+"  ")
	}
}

func TestReadLoop(t *testing.T) {
	buf := &bytes.Buffer{}

	for _, good := range []bool{true, false} {
		buf.Reset()
		doc, err := document.WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, nil)
		if err != nil {
			t.Fatal(err)
		}

		out := doc.Out

		refRoot := out.Alloc()
		refA := out.Alloc()
		refB := out.Alloc()
		refC := out.Alloc()

		var A pdf.Dict
		if good {
			A = pdf.Dict{
				"Title":  pdf.TextString("A"),
				"Next":   refB,
				"Parent": refRoot,
			}
		} else {
			// Create a loop in the outline tree.
			// This causes Acrobat reader to hang (version 2022.003.20310).
			// Let's make sure we do better.
			A = pdf.Dict{
				"Title":  pdf.TextString("A"),
				"Next":   refA,
				"Prev":   refA,
				"Parent": refRoot,
			}
		}
		B := pdf.Dict{
			"Title":  pdf.TextString("B"),
			"Prev":   refA,
			"Next":   refC,
			"Parent": refRoot,
		}
		C := pdf.Dict{
			"Title":  pdf.TextString("C"),
			"Prev":   refB,
			"Parent": refRoot,
		}
		root := pdf.Dict{
			"First": refA,
			"Last":  refC,
		}

		err = out.Put(refA, A)
		if err != nil {
			t.Fatal(err)
		}
		err = out.Put(refB, B)
		if err != nil {
			t.Fatal(err)
		}
		err = out.Put(refC, C)
		if err != nil {
			t.Fatal(err)
		}
		err = out.Put(refRoot, root)
		if err != nil {
			t.Fatal(err)
		}

		out.GetMeta().Catalog.Outlines = refRoot

		err = doc.Close()
		if err != nil {
			t.Fatal(err)
		}

		r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
		if err != nil {
			t.Fatal(err)
		}

		_, err = Read(r)
		if (err == nil) != good {
			t.Errorf("good=%v, err=%v", good, err)
		}

		r.Close()
	}
}

func TestWrite(t *testing.T) {
	buf := &bytes.Buffer{}
	doc, err := document.WriteSinglePage(buf, &pdf.Rectangle{URx: 100, URy: 100}, nil)
	if err != nil {
		t.Fatal(err)
	}

	tree1 := &Tree{
		Children: []*Tree{
			{
				Title: "A",
				Children: []*Tree{
					{Title: "A1"},
					{Title: "A2"},
					{Title: "A3"},
				},
			},
			{
				Title: "B",
				Action: pdf.Dict{
					"S":   pdf.Name("URI"),
					"URI": pdf.String("https://seehuhn.de/"),
				},
			},
			{
				Title: "C",
				Children: []*Tree{
					{Title: "C1"},
					{Title: "C2"},
				},
				Open: true,
			},
		},
	}
	err = tree1.Write(doc.Out)
	if err != nil {
		t.Fatal(err)
	}

	err = doc.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile("test_Write.pdf", buf.Bytes(), 0644)
	if err != nil {
		t.Fatal(err)
	}

	r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
	if err != nil {
		t.Fatal(err)
	}

	tree2, err := Read(r)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(tree1, tree2); d != "" {
		t.Errorf("diff: %s", d)
	}
	printTree(tree2, "")
}
