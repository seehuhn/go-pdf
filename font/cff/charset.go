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

	"seehuhn.de/go/pdf/font/parser"
)

func readCharset(p *parser.Parser, nGlyphs int) ([]int32, error) {
	format, err := p.ReadUInt8()
	if err != nil {
		return nil, err
	}

	charset := make([]int32, 0, nGlyphs)
	charset = append(charset, 0)
	switch format {
	case 0:
		s := &parser.State{
			A: int64(nGlyphs - 1),
		}
		err = p.Exec(s,
			parser.CmdLoop,
			parser.CmdStash,
			parser.CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}

		data := s.GetStash()
		for _, xi := range data {
			charset = append(charset, int32(xi))
		}
	case 1:
		for len(charset) < nGlyphs {
			first, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			}
			nLeft, err := p.ReadUInt8()
			if err != nil {
				return nil, err
			}
			for i := 0; i < int(nLeft)+1; i++ {
				charset = append(charset, int32(int(first)+i))
			}
		}
	case 2:
		for len(charset) < nGlyphs {
			first, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			}
			nLeft, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			}
			for i := 0; i < int(nLeft)+1; i++ {
				charset = append(charset, int32(int(first)+i))
			}
		}
	default:
		return nil, fmt.Errorf("unsupported charset format %d", format)
	}

	return charset, nil
}

func encodeCharset(names []int32) ([]byte, error) {
	if names[0] != 0 {
		return nil, errors.New("invalid charset")
	}
	names = names[1:]

	// find runs of consecutive glyph names
	var runs []int
	for i := 0; i < len(names); i++ {
		if i == 0 || names[i] != names[i-1]+1 {
			runs = append(runs, i)
		}
	}
	runs = append(runs, len(names))

	length0 := 1 + 2*len(names) // length with format 0 encoding

	length1 := 1 + 3*(len(runs)-1) // length with format 1 encoding
	for i := 0; i < len(runs)-1; i++ {
		d := runs[i+1] - runs[i]
		for d > 256 {
			length1 += 3
			d -= 256
		}
	}

	length2 := 1 + 4*(len(runs)-1) // length with format 2 encoding

	var buf []byte
	if length0 <= length1 && length0 <= length2 {
		buf = make([]byte, length0)
		buf[0] = 0
		for i, name := range names {
			buf[2*i+1] = byte(name >> 8)
			buf[2*i+2] = byte(name)
		}
	} else if length1 < length2 {
		buf = make([]byte, length1)
		buf[0] = 1
		for i := 0; i < len(runs)-1; i++ {
			name := names[runs[i]]
			dd := runs[i+1] - runs[i]
			for dd > 0 {
				d := dd - 1
				if d > 255 {
					d = 255
				}
				buf[3*i+1] = byte(name >> 8)
				buf[3*i+2] = byte(name)
				buf[3*i+3] = byte(d)
				name += int32(d + 1)
				dd -= d + 1
			}
		}
	} else {
		buf = make([]byte, length2)
		buf[0] = 2
		for i := 0; i < len(runs)-1; i++ {
			name := names[runs[i]]
			d := runs[i+1] - runs[i] - 1
			buf[4*i+1] = byte(name >> 8)
			buf[4*i+2] = byte(name)
			buf[4*i+3] = byte(d >> 8)
			buf[4*i+4] = byte(d)
		}
	}
	return buf, nil
}
