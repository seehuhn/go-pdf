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
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/builtin"
)

func main() {
	err := doit()
	if err != nil {
		panic(err)
	}
}

func doit() error {
	doc, err := document.CreateMultiPage("test.pdf", document.A4, nil)
	if err != nil {
		return err
	}

	for i := 0; i < 6; i++ {
		page := doc.AddPage()

		var F font.Font
		var title string
		switch i {
		case 0:
			F, err = builtin.Font(builtin.Helvetica)
			title = "Helvetica"
		default:
			title = "To Be Done"
		}

		err = page.Close()
		if err != nil {
			return err
		}
	}

	return doc.Close()
}
