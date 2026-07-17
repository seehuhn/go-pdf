// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package makefont

import (
	"bytes"
	_ "embed"

	"seehuhn.de/go/postscript/type1"
)

//go:embed testdata/mm.pfa
var mmType1Data []byte

// MMType1 returns a multiple master Type 1 font for use in tests.
// The font has two design axes, "Weight" and "Width".
func MMType1() *type1.Font {
	psFont, err := type1.Read(bytes.NewReader(mmType1Data))
	if err != nil {
		panic(err)
	}
	return psFont
}
