package pagetree_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"
)

func TestFindPages(t *testing.T) {
	doc := pdf.NewData(pdf.V1_7)

	numPages := 234
	pageRefsIn := make([]pdf.Reference, numPages)
	tree := pagetree.NewWriter(doc)
	for i := 0; i < numPages; i++ {
		pageRefsIn[i] = doc.Alloc()
		pageDict := pdf.Dict{
			"Type": pdf.Name("Page"),
		}
		doc.Put(pageRefsIn[i], pageDict)
		err := tree.AppendPageRef(pageRefsIn[i], pageDict)
		if err != nil {
			t.Fatal(err)
		}
	}
	treeRef, err := tree.Close()
	if err != nil {
		t.Fatal(err)
	}
	doc.GetMeta().Catalog.Pages = treeRef

	pageRefsOut, err := pagetree.FindPages(doc)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(pageRefsIn, pageRefsOut); d != "" {
		t.Fatalf("unexpected pageRefs (-want +got):\n%s", d)
	}
}
