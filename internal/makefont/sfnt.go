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

package makefont

import (
	"bytes"

	"golang.org/x/image/font/gofont/goregular"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/internal/convert"
)

// TrueType returns a font with glyf outlines.
func TrueType() *sfnt.Font {
	r := bytes.NewReader(goregular.TTF)
	info, err := sfnt.Read(r)
	if err != nil {
		panic(err)
	}
	return info
}

// OpenType returns a font with CFF outlines and not CIDFont operators.
func OpenType() *sfnt.Font {
	info := TrueType()
	info, err := convert.ToCFF(info)
	if err != nil {
		panic(err)
	}
	return info
}

// OpenTypeCID returns a font with CFF outlines and CIDFont operators.
func OpenTypeCID() *sfnt.Font {
	info := OpenType()
	info, err := convert.ToCFFCID(info)
	if err != nil {
		panic(err)
	}
	return info
}

// OpenTypeCID2 returns a font with CFF outlines, CIDFont operators, and
// multiple private dictionaries.
func OpenTypeCID2() *sfnt.Font {
	info := OpenType()
	info, err := convert.ToCFFCID2(info)
	if err != nil {
		panic(err)
	}
	return info
}
