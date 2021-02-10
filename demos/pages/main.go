package main

import (
	"fmt"
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pages"
)

// Rect takes the coordinates of two diagonally opposite points
// and returns a PDF rectangle.
func Rect(llx, lly, urx, ury int) pdf.Array {
	return pdf.Array{pdf.Integer(llx), pdf.Integer(lly),
		pdf.Integer(urx), pdf.Integer(ury)}
}

// WritePage emits a single page to the PDF file and returns the page dict.
func WritePage(out *pdf.Writer, i int) (pdf.Dict, error) {
	stream, contentNode, err := out.OpenStream(nil, nil, nil)
	if err != nil {
		return nil, err
	}
	if i != 3 {
		_, err = stream.Write([]byte(fmt.Sprintf(`BT
		/F1 12 Tf
		30 30 Td
		(page %d) Tj
		ET`, i)))
	} else {
		_, err = stream.Write([]byte(`BT
		/F1 36 Tf
		10 50 Td
		(AVAVXXXAVAV) Tj
		ET`))
	}
	if err != nil {
		return nil, err
	}
	err = stream.Close()
	if err != nil {
		return nil, err
	}

	return pdf.Dict{
		"Type":     pdf.Name("Page"),
		"Contents": contentNode,
	}, nil
}

func main() {
	out, err := pdf.Create("test.pdf")
	if err != nil {
		log.Fatal(err)
	}

	font, err := out.Write(pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name("Helvetica"),
		"Encoding": pdf.Name("MacRomanEncoding"),
	}, nil)
	if err != nil {
		log.Fatal(err)
	}

	pageTree := pages.NewPageTree(out)
	for i := 1; i <= 100; i++ {
		page, err := WritePage(out, i)
		if err != nil {
			log.Fatal(err)
		}
		err = pageTree.Ship(page, nil)
		if err != nil {
			log.Fatal(err)
		}
	}

	pages, pagesRef, err := pageTree.Flush()
	if err != nil {
		log.Fatal(err)
	}
	pages["CropBox"] = Rect(0, 0, 200, 200)
	pages["Resources"] = pdf.Dict{
		"Font": pdf.Dict{"F1": font},
	}
	_, err = out.Write(pages, pagesRef)
	if err != nil {
		log.Fatal(err)
	}

	info, err := out.Write(pdf.Dict{
		"Title":  pdf.TextString("PDF Test Document"),
		"Author": pdf.TextString("Jochen Voß"),
	}, nil)
	if err != nil {
		log.Fatal(err)
	}

	catalog, err := out.Write(pdf.Dict{
		"Type":  pdf.Name("Catalog"),
		"Pages": pagesRef,
	}, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = out.Close(catalog, info)
	if err != nil {
		log.Fatal(err)
	}
}
