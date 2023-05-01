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

			page.TextStart()
			page.TextSetFont(F, 12)
			page.TextFirstLine(50, 250)
			page.TextShow("PDF version " + V.String())
			page.TextSecondLine(0, -geom.ToPDF16(12, geom.BaseLineSkip))
			if enc == "enc" {
				page.TextShow("encrypted text")
			} else {
				page.TextShow("unencrypted text")
			}
			page.TextNewLine()
			page.TextShow("user can copy")
			page.TextNewLine()
			if enc == "plain" {
				page.TextShow("user can print")
			} else {
				page.TextShow("only owner can print")
			}
			page.TextEnd()

			err = page.Close()
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
