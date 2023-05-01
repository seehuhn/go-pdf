package pdf_test

import (
	"bytes"
	"os"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
)

func FuzzEncrypted(f *testing.F) {
	buf := &bytes.Buffer{}

	passwd := "secret"

	for _, v := range []pdf.Version{pdf.V1_1, pdf.V1_2, pdf.V1_3, pdf.V1_4, pdf.V1_5, pdf.V1_6, pdf.V1_7, pdf.V2_0} {
		opt := &pdf.WriterOptions{
			Version:         v,
			UserPassword:    passwd,
			UserPermissions: pdf.PermPrintDegraded,
		}

		// minimal PDF file
		w, err := pdf.NewWriter(buf, opt)
		if err != nil {
			f.Fatal(err)
		}
		w.Catalog.Pages = w.Alloc() // pretend we have a page tree
		err = w.Close()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(buf.Bytes())

		// minimal working PDF file
		buf = &bytes.Buffer{}
		page, err := document.WriteSinglePage(buf, 100, 100, opt)
		if err != nil {
			f.Fatal(err)
		}
		err = page.Close()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(buf.Bytes())
	}

	ropt := &pdf.ReaderOptions{
		ReadPassword: func(ID []byte, try int) string {
			if try < 3 {
				return passwd
			}
			return ""
		},
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		r := bytes.NewReader(raw)
		pdf1, err := pdf.Read(r, ropt)
		if err != nil {
			return
		}
		buf := &bytes.Buffer{}
		err = pdf1.Write(buf)
		if err != nil {
			t.Fatal(err)
		}
		pdfContents1 := buf.Bytes()

		// TODO(voss): also use encryption in the write and read cycle here

		r = bytes.NewReader(pdfContents1)
		pdf2, err := pdf.Read(r, nil)
		if err != nil {
			t.Fatal(err)
		}
		buf = &bytes.Buffer{}
		err = pdf2.Write(buf)
		if err != nil {
			t.Fatal(err)
		}
		pdfContents2 := buf.Bytes()

		if !bytes.Equal(pdfContents1, pdfContents2) {
			os.WriteFile("a.pdf", pdfContents1, 0644)
			os.WriteFile("b.pdf", pdfContents2, 0644)
			t.Fatalf("pdf contents differ")
		}
	})
}