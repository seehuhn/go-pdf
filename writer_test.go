package pdf

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestWriter(t *testing.T) {
	out := &bytes.Buffer{}

	opt := &WriterOptions{
		ID:             [][]byte{},
		OwnerPassword:  "test",
		UserPermission: PermCopy,
	}
	w, err := NewWriter(out, opt)
	if err != nil {
		t.Fatal(err)
	}

	refs, err := w.ObjectStream(nil,
		Dict{
			"Title":    TextString("PDF Test Document"),
			"Author":   TextString("Jochen Voß"),
			"Subject":  TextString("Testing"),
			"Keywords": TextString("PDF, testing, Go"),
		},
		Dict{
			"Type":     Name("Font"),
			"Subtype":  Name("Type1"),
			"BaseFont": Name("Helvetica"),
			"Encoding": Name("MacRomanEncoding"),
		})
	if err != nil {
		t.Fatal(err)
	}
	info := refs[0]
	font := refs[1]

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
	catalog, err := w.Write(Dict{
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

	// ioutil.WriteFile("debug.pdf", out.Bytes(), 0o644)

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
	if err != nil {
		t.Fatal(err)
	}
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
