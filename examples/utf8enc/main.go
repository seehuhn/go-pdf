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

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/cmap"
)

func main() {
	err := doit()
	if err != nil {
		panic(err)
	}
}

func doit() error {
	page, err := document.CreateSinglePage("test.pdf", document.A4, nil)
	if err != nil {
		return err
	}

	info, err := sfnt.ReadFile("../../../otf/SourceSerif4-Regular.otf")
	if err != nil {
		return err
	}

	opt := &cff.FontOptions{
		Language:     language.German,
		MakeGIDToCID: cmap.NewIdentityGIDToCID,
		MakeEncoder:  cmap.NewUTF8Encoder,
	}
	FF, err := cff.NewComposite(info, opt)
	if err != nil {
		return err
	}
	F, err := FF.Embed(page.Out, "F")
	if err != nil {
		return err
	}

	page.TextSetFont(F, 36)
	page.TextStart()
	page.TextFirstLine(100, 700)
	page.TextShow("“Größenwahn”")
	page.TextEnd()

	return page.Close()
}
