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
	"golang.org/x/text/language"

	"seehuhn.de/go/pdf"

	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func main() {
	err := doit()
	if err != nil {
		panic(err)
	}
}

func doit() error {
	pdfOpt := &pdf.WriterOptions{
		HumanReadable: false,
	}
	page, err := document.CreateSinglePage("test.pdf", document.A4, pdf.V1_7, pdfOpt)
	if err != nil {
		return err
	}

	cffFont := makefont.OpenType()
	fontOpt := &cff.Options{
		Language:    language.German,
		Composite:   true,
		MakeEncoder: cidenc.NewCompositeUtf8,
	}
	F1, err := cff.New(cffFont, fontOpt)
	if err != nil {
		return err
	}
	F2, err := cff.New(cffFont, nil)
	if err != nil {
		return err
	}

	page.TextBegin()
	page.TextFirstLine(100, 700)
	page.TextSetFont(F1, 36)
	page.TextShow("„Größenwahn“")
	page.TextSecondLine(0, -40)
	page.TextSetFont(F2, 36)
	page.TextShow("„Größenwahn“")
	page.TextEnd()

	return page.Close()
}
