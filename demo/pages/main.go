// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
	"fmt"
	"log"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pages"
)

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
		(OXO) Tj
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

	pageTree := pages.NewPageTree(out, &pages.DefaultAttributes{
		Resources: pdf.Dict{
			"Font": pdf.Dict{"F1": font},
		},
		MediaBox: &pages.Rectangle{LLx: 0, LLy: 0, URx: 200, URy: 200},
	})
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

	pagesRef, err := pageTree.Flush()
	if err != nil {
		log.Fatal(err)
	}

	err = out.SetInfo(pdf.Struct(&pdf.Info{
		Title:  "PDF Test Document",
		Author: "Jochen Voß",
	}))
	if err != nil {
		log.Fatal(err)
	}

	err = out.SetCatalog(pdf.Dict{
		"Type":  pdf.Name("Catalog"),
		"Pages": pagesRef,
	})
	if err != nil {
		log.Fatal(err)
	}

	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}
