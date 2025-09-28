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
	"seehuhn.de/go/pdf/font/gofont"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	doc, err := document.CreateSinglePage("hello.pdf", document.A4r, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	font, err := gofont.Regular.NewSimple(nil)
	if err != nil {
		return err
	}

	doc.TextSetFont(font, 50)
	doc.TextBegin()
	doc.TextFirstLine(50, 420)
	doc.TextShow("Hello, World!")
	doc.TextSecondLine(0, -60)
	doc.TextShow(string(gofont.Gopher))
	doc.TextEnd()

	return doc.Close()
}
