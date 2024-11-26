// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/internal/makefont"
)

func main() {
	err := doit()
	if err != nil {
		panic(err)
	}
}

func doit() error {
	out, err := document.CreateSinglePage("test.pdf", document.A5r, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	psFont := makefont.Type1()
	f1, err := NewType1(psFont, nil)
	if err != nil {
		return err
	}

	// temporary hack to get a Typesetter
	_, q, err := pdf.ResourceManagerEmbed(out.RM, f1)
	if err != nil {
		return err
	}
	q2 := q.(*Typesetter)
	s := q2.AppendEncoded(nil, "Hello, World!")

	out.TextSetFont(f1, 12)
	out.TextBegin()
	out.TextFirstLine(100, 100)
	out.TextShowRaw(s)
	out.TextEnd()

	err = out.Close()
	if err != nil {
		return err
	}
	return nil
}
