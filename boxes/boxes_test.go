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
	pageTree := pages.NewPageTree(out, &pages.DefaultAttributes{
		Resources: pdf.Dict{},
		MediaBox:  pages.A5,
		Rotate:    0,
	})

	box := &vBox{
		stuffExtent: stuffExtent{
			Width:  pages.A5.URx - pages.A5.LLx,
			Height: pages.A5.URy - pages.A5.LLy,
			Depth:  0,
		},
		Contents: []stuff{
			kern(30),
			&hBox{
				stuffExtent: stuffExtent{
					Width:  pages.A5.URx - pages.A5.LLx,
					Height: 10,
					Depth:  2,
				},
				Contents: []stuff{
					kern(36),
					&rule{
						stuffExtent: stuffExtent{
							Width:  20,
							Height: 8,
							Depth:  0,
						},
					},
					kern(5),
					&rule{
						stuffExtent: stuffExtent{
							Width:  30,
							Height: 8,
							Depth:  1.8,
						},
					},
					&glue{
						Length: 0,
						Plus:   stretchAmount{1, 1},
					},
				},
			},
			&glue{
				Length: 0,
				Plus:   stretchAmount{1, 1},
			},
			&hBox{
				stuffExtent: stuffExtent{
					Width:  pages.A5.URx - pages.A5.LLx,
					Height: 10,
					Depth:  2,
				},
				Contents: []stuff{
					&glue{
						Length: 0,
						Plus:   stretchAmount{1, 1},
					},
					&rule{
						stuffExtent: stuffExtent{
							Width:  20,
							Height: 8,
							Depth:  0,
						},
					},
					&glue{
						Length: 0,
						Plus:   stretchAmount{1, 1},
					},
				},
			},
			kern(30),
		},
	}

	page, err := pageTree.AddPage(&pages.Attributes{
		MediaBox: &pages.Rectangle{
			URx: pages.A5.URx - pages.A5.LLx,
			URy: pages.A5.URy - pages.A5.LLy,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	box.Draw(page, 0, box.Depth)

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

// compile time test: we implement the correct interfaces
var _ stuff = &rule{}
var _ stuff = &vBox{}
var _ stuff = kern(0)
var _ stuff = &glue{}
