// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
			page, err := document.CreateSinglePage(fname, &pdf.Rectangle{URx: 300, URy: 300}, opt)
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
