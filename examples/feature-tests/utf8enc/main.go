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
	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
)

func main() {
	err := doit()
	if err != nil {
		panic(err)
	}
}

func doit() error {
	page, err := document.CreateSinglePage("test.pdf", document.A4, pdf.V1_7, nil)
	if err != nil {
		return err
	}

	info, err := sfnt.ReadFile("../../../../../otf/SourceSerif4-Regular.otf")
	if err != nil {
		return err
	}

	opt := &font.Options{
		Language:     language.German,
		Composite:    true,
		MakeGIDToCID: cmap.NewGIDToCIDIdentity,
		MakeEncoder:  cidenc.NewCompositeUtf8,
	}
	F, err := cff.New(info, opt)
	if err != nil {
		return err
	}

	page.TextSetFont(F, 36)
	page.TextBegin()
	page.TextFirstLine(100, 700)
	page.TextShow("„Größenwahn“")
	page.TextEnd()

	return page.Close()
}
