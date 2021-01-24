package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriter(t *testing.T) {
	out := &bytes.Buffer{}

	w, err := NewWriter(out, V1_7)
	if err != nil {
		t.Fatal(err)
	}
	var catalog, info *Reference

	info, err = w.Write(Dict{ // page 550
		"Title":    TextString("PDF Test Document"),
		"Author":   TextString("Jochen Voß"),
		"Subject":  TextString("Testing"),
		"Keywords": TextString("PDF, testing, Go"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	stream, contentNode, err := w.OpenStream(Dict{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = stream.Write([]byte(`BT
/F1 24 Tf
30 30 Td
(Hello World) Tj
ET
`))
	if err != nil {
		t.Fatal(err)
	}
	err = stream.Close()
	if err != nil {
		t.Fatal(err)
	}

	font, err := w.Write(Dict{
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

	page1, err := w.Write(Dict{
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
	_, err = w.Write(pages, pagesRef)
	if err != nil {
		t.Fatal(err)
	}

	// page 73
	catalog, err = w.Write(Dict{
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

	fd, _ := os.Create("xxx.pdf")
	fd.Write(out.Bytes())
	fd.Close()

	outR := bytes.NewReader(out.Bytes())
	_, err = NewReader(outR, outR.Size(), nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPlaceholder(t *testing.T) {
	const testVal = 12345

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.pdf")

	w, err := Create(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	length := &placeholder{
		size:  5,
		alloc: w.Alloc,
		store: w.Write,
	}
	testRef, err := w.Write(Dict{
		"Test":   Bool(true),
		"Length": length,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	cat, err := w.Write(Dict{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = length.Set(Integer(testVal))
	if err != nil {
		t.Fatal(err)
	}

	err = w.Close(cat, nil)
	if err != nil {
		t.Fatal(err)
	}

	// try to read back the file

	r, err := Open(tmpFile)
	obj, err := r.GetDict(testRef)
	if err != nil {
		t.Fatal(err)
	}

	lengthOut, err := r.GetInt(obj["Length"])
	if err != nil {
		t.Fatal(err)
	}

	if lengthOut != testVal {
		t.Errorf("wrong /Length: %d vs %d", lengthOut, testVal)
	}
}
