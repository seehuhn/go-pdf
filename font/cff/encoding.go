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
	"errors"
	"fmt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

func (cff *Font) encodeEncoding(enc font.Encoding, ss cffStrings) error {
	// TODO(voss): check whether StandardEncoding or ExpertEncoding can
	// be used.

	type supplement struct {
		code     byte
		glyphSID int32
	}
	var extra []*supplement
	gid2code := make([]int16, 256)
	for i := range gid2code {
		gid2code[i] = -1
	}
	var maxGID font.GlyphID
	for code, gid := range enc {
		if int(gid) >= len(cff.Glyphs) || gid == 0 {
			return errors.New("cff: invalid GID")
		}

		if gid <= 255 && gid2code[gid] < 0 {
			if gid > maxGID {
				maxGID = gid
			}
			gid2code[gid] = int16(code)
		} else {
			extra = append(extra, &supplement{
				code:     code,
				glyphSID: ss.lookup(string(cff.Glyphs[gid].Name)),
			})
		}
	}

	length0 := 2 + int(maxGID)

	// length1 :=
	_ = length0

	return nil
}

func (cff *Font) readEncoding(p *parser.Parser) (font.Encoding, error) {
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
