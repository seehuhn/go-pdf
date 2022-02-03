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
	if nGlyphs < 0 || nGlyphs >= 0x10000 {
		return nil, fmt.Errorf("invalid number of glyphs: %d", nGlyphs)
	}

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
			parser.CmdStash16,
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
			for i := int32(0); i < int32(nLeft)+1; i++ {
				code := int32(first) + i
				if code > 0xFFFF {
					return nil, fmt.Errorf("invalid charset entry: %d", code)
				}
				charset = append(charset, code)
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
			for i := int32(0); i < int32(nLeft)+1; i++ {
				code := int32(first) + i
				if code > 0xFFFF {
					return nil, fmt.Errorf("invalid charset entry: %d", code)
				}
				charset = append(charset, code)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported charset format %d", format)
	}

	if len(charset) != nGlyphs {
		return nil, fmt.Errorf("unexpected charset length: %d", len(charset))
	}

	return charset, nil
}

func encodeCharset(names []int32) ([]byte, error) {
	if names[0] != 0 {
		return nil, errors.New("invalid charset")
	}
	names = names[1:]

	// find runs of consecutive glyph names
	runs := []int{0}
	for i := 1; i < len(names); i++ {
		if names[i] != names[i-1]+1 {
			runs = append(runs, i)
		}
	}
	runs = append(runs, len(names))

	length0 := 1 + 2*len(names) // length with format 0 encoding

	length1 := 1 + 3*(len(runs)-1) // length with format 1 encoding
	for i := 1; i < len(runs); i++ {
		d := runs[i] - runs[i-1]
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
		buf = make([]byte, 0, length1)
		buf = append(buf, 1)
		for i := 0; i < len(runs)-1; i++ {
			name := names[runs[i]]
			length := int32(runs[i+1] - runs[i])
			for length > 0 {
				chunk := length
				if chunk > 256 {
					chunk = 256
				}
				buf = append(buf, byte(name>>8), byte(name), byte(chunk-1))
				name += chunk
				length -= chunk
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
