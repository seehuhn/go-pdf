// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

// Package glyf implements reading and writing the "glyf" and "loca" tables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/glyf
// https://docs.microsoft.com/en-us/typography/opentype/spec/loca
package glyf

import "io"

// Info contains information from the "glyf" table.
type Info struct {
	Glyphs []*Glyph
}

// Decode converts the data from the "glyf" and "loca" tables into
// a slice of Glyphs.
func Decode(glyfData, locaData []byte, locaFormat int16) (*Info, error) {
	offs, err := decodeLoca(glyfData, locaData, locaFormat)
	if err != nil {
		return nil, err
	}

	numGlyphs := len(offs) - 1

	gg := make([]*Glyph, numGlyphs)
	for i := range gg {
		data := glyfData[offs[i]:offs[i+1]]
		g, err := decodeGlyph(data)
		if err != nil {
			return nil, err
		}
		gg[i] = g
	}

	info := &Info{
		Glyphs: gg,
	}
	return info, nil
}

// Encode encodes the Glyphs into a "glyf" and "loca" table.
func (info *Info) Encode(w io.Writer) ([]byte, int16, error) {
	offs := make([]int, len(info.Glyphs)+1)
	offs[0] = 0
	for i, g := range info.Glyphs {
		data := g.encode()

		n, err := w.Write(data)
		if err != nil {
			return nil, 0, err
		}
		if n%2 != 0 {
			_, err := w.Write([]byte{0})
			if err != nil {
				return nil, 0, err
			}
			n++
		}

		offs[i+1] = offs[i] + n
	}

	locaData, locaFormat := encodeLoca(offs)

	return locaData, locaFormat, nil
}
