package boxes

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pages"
)

func TestFrame(t *testing.T) {
	out, err := pdf.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = out.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	pageTree := pages.NewPageTree(out, &pages.Attributes{
		Resources: pdf.Dict{},
		MediaBox:  pages.A4,
		Rotate:    0,
	})
	pages, err := pageTree.Flush()
	if err != nil {
		t.Fatal(err)
	}

	out.SetCatalog(pdf.Struct(&pdf.Catalog{
		Pages: pages,
	}))
}
