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

package cff

import (
	"fmt"

	"seehuhn.de/go/pdf/font/parser"
)

type encoding struct{}

func (cff *Font) readEncoding(p *parser.Parser) (*encoding, error) {
	format, err := p.ReadUInt8()
	if err != nil {
		return nil, err
	}

	supplement := format&128 != 0
	format &= 127

	switch format {
	case 0:
		nCodes, err := p.ReadUInt8()
		if err != nil {
			return nil, err
		}
		data, err := p.ReadBlob(int(nCodes))
		if err != nil {
			return nil, err
		}
		for i, c := range data {
			fmt.Println(c, "->", cff.Glyphs[i+1].Name)
		}
	case 1:
		fmt.Println("format 1")
	default:
		return nil, fmt.Errorf("unsupported encoding format %d", format)
	}

	if supplement {
		nSups, err := p.ReadUInt8()
		if err != nil {
			return nil, err
		}
		for i := 0; i < int(nSups); i++ {

		}
	}

	return nil, nil
}
