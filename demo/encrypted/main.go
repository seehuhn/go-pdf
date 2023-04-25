package main

import (
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/builtin"
)

func main() {
	for _, V := range []pdf.Version{pdf.V1_1, pdf.V1_3, pdf.V1_4, pdf.V1_5, pdf.V1_7, pdf.V2_0} {
		opt := &pdf.WriterOptions{
			Version:        V,
			UserPassword:   "A",
			OwnerPassword:  "B",
			UserPermission: pdf.PermCopy,
		}

		fname := "encrypted-" + V.String() + ".pdf"
		page, err := document.CreateSinglePage(fname, 300, 300, opt)
		if err != nil {
			log.Fatal(err)
		}

		F, err := builtin.Embed(page.Out, builtin.TimesRoman, "F")
		if err != nil {
			log.Fatal(err)
		}

		page.BeginText()
		page.SetFont(F, 12)
		page.StartLine(50, 250)
		page.ShowText("PDF version " + V.String())
		geom := F.GetGeometry()
		page.StartNextLine(0, -geom.ToPDF16(12, geom.BaseLineSkip))
		page.ShowText("encrypted text")
		page.NewLine()
		page.ShowText("user can view")
		page.NewLine()
		page.ShowText("owner can copy")
		page.EndText()

		err = page.Close()
		if err != nil {
			log.Fatal(err)
		}
	}
}
