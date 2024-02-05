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

package testfont

import (
	"bytes"

	"golang.org/x/image/font/gofont/goregular"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/internal/convert"
)

// MakeGlyfFont returns a font with glyf outlines.
func MakeGlyfFont() *sfnt.Font {
	r := bytes.NewReader(goregular.TTF)
	info, err := sfnt.Read(r)
	if err != nil {
		panic(err)
	}
	return info
}

// MakeCFFFont returns a font with CFF outlines and not CIDFont operators.
func MakeCFFFont() *sfnt.Font {
	info := MakeGlyfFont()
	info, err := convert.ToCFF(info)
	if err != nil {
		panic(err)
	}
	return info
}

// MakeCFFCIDFont returns a font with CFF outlines and CIDFont operators.
func MakeCFFCIDFont() *sfnt.Font {
	info := MakeCFFFont()
	info, err := convert.ToCFFCID(info)
	if err != nil {
		panic(err)
	}
	return info
}

// MakeCFFCIDFont2 returns a font with CFF outlines, CIDFont operators, and
// multiple private dictionaries.
func MakeCFFCIDFont2() *sfnt.Font {
	info := MakeCFFFont()
	info, err := convert.ToCFFCID2(info)
	if err != nil {
		panic(err)
	}
	return info
}
