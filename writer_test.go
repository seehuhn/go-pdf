package pdf

import (
	"bytes"
	"testing"
)

func TestWriter(t *testing.T) {
	out := &bytes.Buffer{}

	w, err := NewWriter(out, V1_5)
	if err != nil {
		t.Fatal(err)
	}
	var catalog, info *Reference

	info, err = w.WriteIndirect(Dict{ // page 550
		"Title":    TextString("PDF Test Document"),
		"Author":   TextString("Jochen Voß"),
		"Subject":  TextString("Testing"),
		"Keywords": TextString("PDF, testing, Go"),
	}, nil)
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
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	font, err := w.WriteIndirect(Dict{
		"Type":     Name("Font"),
		"Subtype":  Name("Type1"),
		"BaseFont": Name("Helvetica"),
		"Encoding": Name("MacRomanEncoding"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	resources := Dict{
		"Font": Dict{"F1": font},
	}

	pagesRef := w.Alloc()
	pages := Dict{
		"Type":  Name("Pages"),
		"Kids":  Array{},
		"Count": Integer(0),
	}

	page1, err := w.WriteIndirect(Dict{
		"Type":      Name("Page"),
		"CropBox":   Array{Integer(0), Integer(0), Integer(200), Integer(100)},
		"Resources": resources,
		"Contents":  contentNode,
		"Parent":    pagesRef,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	pages["Kids"] = append(pages["Kids"].(Array), page1)
	pages["Count"] = pages["Count"].(Integer) + 1
	_, err = w.WriteIndirect(pages, pagesRef)
	if err != nil {
		t.Fatal(err)
	}

	// page 73
	catalog, err = w.WriteIndirect(Dict{
		"Type":  Name("Catalog"),
		"Pages": pagesRef,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close(catalog, info)
	if err != nil {
		t.Fatal(err)
	}

	outR := bytes.NewReader(out.Bytes())
	_, err = NewReader(outR, outR.Size(), nil)
	if err != nil {
		t.Fatal(err)
	}
}
