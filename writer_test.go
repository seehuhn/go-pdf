package pdflib

import (
	"bytes"
	"os"
	"testing"
)

func TestWriter(t *testing.T) {
	fd, err := os.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()

	w, err := NewWriter(fd, V1_5)
	if err != nil {
		t.Fatal(err)
	}
	var catalog, info *Reference
	defer func() {
		err = w.Close(catalog, info)
		if err != nil {
			t.Error(err)
		}
	}()

	info, err = w.WriteIndirect(Dict{ // page 550
		"Title":    String("PDF Test Document"),
		"Author":   String("Jochen Voss"),
		"Subject":  String("Testing"),
		"Keywords": String("PDF, testing, Go"),
	})
	if err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewReader([]byte(`BT
/F1 24 Tf
30 30 Td
(Hello World) Tj
ET
`))
	contentNode, err := w.WriteIndirect(&Stream{
		Dict: Dict{
			"Length": Integer(buf.Size()),
		},
		R: buf,
	})
	if err != nil {
		t.Fatal(err)
	}
	font, err := w.WriteIndirect(Dict{
		"Type":     Name("Font"),
		"Subtype":  Name("Type1"),
		"BaseFont": Name("Helvetica"),
		"Encoding": Name("MacRomanEncoding"),
	})
	if err != nil {
		t.Fatal(err)
	}

	resources := Dict{
		"Font": Dict{"F1": font},
	}

	pagesObj, pagesRef := w.ReserveNumber(Dict{ // page 76
		"Type":  Name("Pages"),
		"Kids":  Array{},
		"Count": Integer(0),
	})

	page1, err := w.WriteIndirect(Dict{ // page 77
		"Type":      Name("Page"),
		"CropBox":   Array{Integer(0), Integer(0), Integer(200), Integer(100)},
		"MediaBox":  Array{Integer(0), Integer(0), Integer(200), Integer(100)},
		"Resources": resources,
		"Contents":  contentNode,
		"Parent":    pagesRef,
	})
	if err != nil {
		t.Fatal(err)
	}

	pages := pagesObj.Obj.(Dict)
	pages["Kids"] = append(pages["Kids"].(Array), page1)
	pages["Count"] = pages["Count"].(Integer) + 1
	_, err = w.WriteIndirect(pagesObj)
	if err != nil {
		t.Fatal(err)
	}

	// page 73
	catalog, err = w.WriteIndirect(Dict{
		"Type":  Name("Catalog"),
		"Pages": pagesRef,
	})
	if err != nil {
		t.Fatal(err)
	}
}
