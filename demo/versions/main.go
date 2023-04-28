package main

import (
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/builtin"
)

func main() {
	vv := []pdf.Version{pdf.V1_0, pdf.V1_1, pdf.V1_2, pdf.V1_3, pdf.V1_4,
		pdf.V1_5, pdf.V1_6, pdf.V1_7, pdf.V2_0}

	for _, V := range vv {
		for _, enc := range []string{"plain", "prot", "enc"} {
			if V == pdf.V1_0 && enc != "plain" {
				continue
			}

			fname := "out-" + V.String() + "-" + enc + ".pdf"

			opt := &pdf.WriterOptions{
				Version: V,
			}
			if enc != "plain" {
				opt.OwnerPassword = "B"
				opt.UserPermissions = pdf.PermCopy
			}
			if enc == "enc" {
				opt.UserPassword = "A"
			}
			page, err := document.CreateSinglePage(fname, 300, 300, opt)
			if err != nil {
				log.Fatal(err)
			}

			F, err := builtin.Embed(page.Out, builtin.TimesRoman, "F")
			if err != nil {
				log.Fatal(err)
			}
			geom := F.GetGeometry()

			page.BeginText()
			page.SetFont(F, 12)
			page.StartLine(50, 250)
			page.ShowText("PDF version " + V.String())
			page.StartNextLine(0, -geom.ToPDF16(12, geom.BaseLineSkip))
			if enc == "enc" {
				page.ShowText("encrypted text")
			} else {
				page.ShowText("unencrypted text")
			}
			page.NewLine()
			page.ShowText("user can copy")
			page.NewLine()
			if enc == "plain" {
				page.ShowText("user can print")
			} else {
				page.ShowText("only owner can print")
			}
			page.EndText()

			err = page.Close()
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
