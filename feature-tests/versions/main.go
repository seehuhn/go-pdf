// Copyright 2023 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"crypto/md5"
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
)

func main() {
	vv := []pdf.Version{pdf.V1_0, pdf.V1_1, pdf.V1_2, pdf.V1_3, pdf.V1_4,
		pdf.V1_5, pdf.V1_6, pdf.V1_7, pdf.V2_0}

	for _, v := range vv {
		for _, enc := range []string{"plain", "prot", "enc"} {
			if v == pdf.V1_0 && enc != "plain" {
				continue
			}

			fname := "out-" + v.String() + "-" + enc + ".pdf"

			opt := &pdf.WriterOptions{}
			if enc != "plain" {
				// a fixed file identifier, so that a password file can name
				// the encrypted outputs
				id := md5.Sum([]byte("seehuhn.de/go/pdf/feature-tests/versions/" + fname))
				opt.ID = [][]byte{id[:]}
				opt.OwnerPassword = "B"
				opt.UserPermissions = pdf.PermCopy
			}
			if enc == "enc" {
				opt.UserPassword = "A"
			}
			page, err := document.CreateSinglePage(fname, &pdf.Rectangle{URx: 300, URy: 300}, v, opt)
			if err != nil {
				log.Fatal(err)
			}

			F := font.Must(standard.TimesRoman.New())
			if v == pdf.V1_0 {
				F.Name = "F1"
			}
			geom := F.GetGeometry()

			page.TextBegin()
			page.TextSetFont(F, 12)
			page.TextFirstLine(50, 250)
			page.TextShow("PDF version " + v.String())
			page.TextSecondLine(0, -12*geom.Leading)
			if enc == "enc" {
				page.TextShow("encrypted text")
			} else {
				page.TextShow("unencrypted text")
			}
			page.TextNextLine()
			page.TextShow("user can copy")
			page.TextNextLine()
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
