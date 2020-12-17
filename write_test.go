package pdflib

import (
	"bytes"
	"os"
	"testing"
)

func makeDict(args ...interface{}) *PDFDict {
	res := &PDFDict{
		Data: make(map[PDFName]PDFObject),
	}
	var name PDFName
	for i, arg := range args {
		if i%2 == 0 {
			name = PDFName(arg.(string))
		} else {
			res.Data[name] = PDFObject(arg)
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
	contentNode := &PDFStream{
		PDFDict: *makeDict(
			"Length", PDFInt(buf.Size()),
		),
		R: buf,
	}

	font := makeDict(
		"Type", PDFName("Font"),
		"Subtype", PDFName("Type1"),
		"BaseFont", PDFName("Helvetica"),
		"Encoding", PDFName("MacRomanEncoding"))

	resources := makeDict(
		"Font", makeDict("F1", font))

	page1 := &PDFDict{ // page 77
		Data: map[PDFName]PDFObject{
			"Type":      PDFName("Page"),
			"CropBox":   PDFArray{PDFInt(0), PDFInt(0), PDFInt(200), PDFInt(100)},
			"MediaBox":  PDFArray{PDFInt(0), PDFInt(0), PDFInt(200), PDFInt(100)},
			"Resources": resources,
			"Contents":  contentNode,
		},
	}

	pages := &PDFDict{ // page 76
		Data: map[PDFName]PDFObject{
			"Type":  PDFName("Pages"),
			"Kids":  PDFArray{page1},
			"Count": PDFInt(1),
		},
	}
	page1.Data["Parent"] = pages

	catalog := &PDFDict{ // page 73
		Data: map[PDFName]PDFObject{
			"Type":  PDFName("Catalog"),
			"Pages": pages,
		},
	}
	info := &PDFDict{ // page 550
		Data: map[PDFName]PDFObject{
			"Title":    PDFString("PDF Test Document"),
			"Author":   PDFString("Jochen Voss"),
			"Subject":  PDFString("Testing"),
			"Keywords": PDFString("PDF, testing, Go"),
		},
	}

	fd, err := os.Create("test.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()
	Write(fd, catalog, info, V1_5)
}
