package pdflib

import (
	"os"
	"testing"
)

func TestWrite(t *testing.T) {
	page1 := &PDFDict{ // page 77
		Data: map[PDFName]PDFObject{
			"Type":     PDFName("Page"),
			"CropBox":  PDFArray{PDFInt(0), PDFInt(0), PDFInt(595), PDFInt(842)},
			"MediaBox": PDFArray{PDFInt(0), PDFInt(0), PDFReal(595.22), PDFInt(842)},
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
