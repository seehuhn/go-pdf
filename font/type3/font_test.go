package type3

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pages"
)

func TestType3(t *testing.T) {
	w, err := pdf.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}

	F1Builder, err := New(w, 1000, 1000)
	if err != nil {
		t.Fatal(err)
	}

	g, err := F1Builder.AddGlyph('A', 1000)
	if err != nil {
		t.Fatal(err)
	}
	g.Println("1000 0 0 0 750 750 d1")
	g.Println("0 0 750 750 re")
	g.Println("f")
	err = g.Close()
	if err != nil {
		t.Fatal(err)
	}

	g, err = F1Builder.AddGlyph('B', 1000)
	if err != nil {
		t.Fatal(err)
	}
	g.Println("1000 0 0 0 750 750 d1")
	g.Println("0 0 m")
	g.Println("375 750 l")
	g.Println("750 0 l")
	g.Println("f")
	err = g.Close()
	if err != nil {
		t.Fatal(err)
	}

	F1, err := F1Builder.Close()
	if err != nil {
		t.Fatal(err)
	}

	page, err := pages.SinglePage(w, &pages.Attributes{
		Resources: map[pdf.Name]pdf.Object{
			"Font": pdf.Dict{
				"F1": F1.Ref,
			},
		},
		MediaBox: pages.A5,
	})
	if err != nil {
		t.Fatal(err)
	}
	page.Println("BT")
	page.Println("/F1 12 Tf")
	page.Println("72 340 Td")
	page.Println("(ABABAB) Tj")
	page.Println("ET")
	err = page.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}
}
