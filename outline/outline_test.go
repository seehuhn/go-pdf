package outline

import (
	"bytes"
	"fmt"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
)

func TestRead(t *testing.T) {
	r, err := pdf.Open("/Users/voss/project/pdf/specs/PDF32000_2008.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	o, err := Read(r)
	if err != nil {
		t.Fatal(err)
	}

	printTree(o, "")
}

func TestReadLoop(t *testing.T) {
	buf := &bytes.Buffer{}

	for _, good := range []bool{true, false} {
		buf.Reset()
		doc, err := document.WriteSinglePage(buf, 100, 100)
		if err != nil {
			t.Fatal(err)
		}

		out := doc.Out

		// create a loop in the outline tree
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
			// Apparently this causes Acrobat reader to hang (version 2022.003.20310).
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

		_, err = out.Write(A, refA)
		if err != nil {
			t.Fatal(err)
		}
		_, err = out.Write(B, refB)
		if err != nil {
			t.Fatal(err)
		}
		_, err = out.Write(C, refC)
		if err != nil {
			t.Fatal(err)
		}
		_, err = out.Write(root, refRoot)
		if err != nil {
			t.Fatal(err)
		}

		out.Catalog.Outlines = refRoot

		err = doc.Close()
		if err != nil {
			t.Fatal(err)
		}

		r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil)
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

func printTree(node *Tree, pfx string) {
	if node == nil {
		fmt.Printf("%snull\n", pfx)
		return
	}
	if node.Title != "" {
		fmt.Printf("%s%s (%d)\n", pfx, node.Title, node.Count)
	} else {
		fmt.Printf("%s<no title> (%d)\n", pfx, node.Count)
	}
	for _, c := range node.Children {
		printTree(c, pfx+"  ")
	}
}
