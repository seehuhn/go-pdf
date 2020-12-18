package pdflib

import (
	"bytes"
	"os"
	"testing"
)

func makeDict(args ...interface{}) *Dict {
	res := &Dict{
		Data: make(map[Name]Object),
	}
	var name Name
	for i, arg := range args {
		if i%2 == 0 {
			name = Name(arg.(string))
		} else {
			res.Data[name] = Object(arg.(Object))
		}
	}
	return res
}

func TestWrite(t *testing.T) {
	contents := `BT
/F1 24 Tf
30 30 Td
(Hello World) Tj
ET
`
	buf := bytes.NewReader([]byte(contents))
	contentNode := &Stream{
		Dict: *makeDict(
			"Length", Integer(buf.Size()),
		),
		R: buf,
	}

	font := makeDict(
		"Type", Name("Font"),
		"Subtype", Name("Type1"),
		"BaseFont", Name("Helvetica"),
		"Encoding", Name("MacRomanEncoding"))

	resources := makeDict(
		"Font", makeDict("F1", font))

	page1 := &Dict{ // page 77
		Data: map[Name]Object{
			"Type":      Name("Page"),
			"CropBox":   Array{Integer(0), Integer(0), Integer(200), Integer(100)},
			"MediaBox":  Array{Integer(0), Integer(0), Integer(200), Integer(100)},
			"Resources": resources,
			"Contents":  contentNode,
		},
	}

	pages := &Dict{ // page 76
		Data: map[Name]Object{
			"Type":  Name("Pages"),
			"Kids":  Array{page1},
			"Count": Integer(1),
		},
	}
	page1.Data["Parent"] = pages

	catalog := &Dict{ // page 73
		Data: map[Name]Object{
			"Type":  Name("Catalog"),
			"Pages": pages,
		},
	}
	info := &Dict{ // page 550
		Data: map[Name]Object{
			"Title":    String("PDF Test Document"),
			"Author":   String("Jochen Voss"),
			"Subject":  String("Testing"),
			"Keywords": String("PDF, testing, Go"),
		},
	}

	fd, err := os.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()
	Write(fd, catalog, info, V1_5)
}
