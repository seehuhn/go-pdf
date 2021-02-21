package boxes

import (
	"fmt"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pages"
)

func draw(page *pages.Page, box *vBox) {
	originX := 0.0
	originY := box.Depth
	fmt.Fprintf(page, "%f %f %f %f re s",
		originX, originY-box.Depth, box.Width, box.Depth+box.Height)
}

func TestFrame(t *testing.T) {
	out, err := pdf.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}
	pageTree := pages.NewPageTree(out, &pages.DefaultAttributes{
		Resources: pdf.Dict{},
		MediaBox:  pages.A5,
		Rotate:    0,
	})

	box := &vBox{
		Width:    pages.A5.URx - pages.A5.LLx,
		Height:   pages.A5.URy - pages.A5.LLy,
		Depth:    0,
		Contents: []stuff{},
	}

	page, err := pageTree.AddPage(&pages.Attributes{
		MediaBox: &pages.Rectangle{
			LLx: 0,
			LLy: 0,
			URx: pages.A5.URx - pages.A5.LLx,
			URy: pages.A5.URy - pages.A5.LLy,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	draw(page, box)

	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	pages, err := pageTree.Flush()
	if err != nil {
		t.Fatal(err)
	}

	err = out.SetCatalog(pdf.Struct(&pdf.Catalog{
		Pages: pages,
	}))
	if err != nil {
		t.Fatal(err)
	}

	err = out.Close()
	if err != nil {
		t.Error(err)
	}
}
