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
	"io"
	"sort"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

// FdSelectFn maps glyphID values to private dicts in Font.Info.Private.
type FdSelectFn func(font.GlyphID) int

func readFDSelect(p *parser.Parser, nGlyphs, nPrivate int) (FdSelectFn, error) {
	format, err := p.ReadUint8()
	if err != nil {
		return nil, err
	}

	switch format {
	case 0:
		buf := make([]uint8, nGlyphs)
		_, err := io.ReadFull(p, buf)
		if err != nil {
			return nil, err
		}
		for i := 0; i < nGlyphs; i++ {
			if int(buf[i]) >= nPrivate {
				return nil, invalidSince("FDSelect out of range")
			}
		}
		return func(gid font.GlyphID) int {
			return int(buf[gid])
		}, nil
	case 3:
		nRanges, err := p.ReadUint16()
		if err != nil {
			return nil, err
		}
		if nGlyphs > 0 && nRanges == 0 {
			return nil, invalidSince("no FDSelect data found")
		}

		var end []font.GlyphID
		var fdIdx []uint8

		prev := uint16(0)
		for i := 0; i < int(nRanges); i++ {
			first, err := p.ReadUint16()
			if err != nil {
				return nil, err
			} else if i > 0 && first <= prev || i == 0 && first != 0 {
				return nil, invalidSince("FDSelect is invalid")
			}
			fd, err := p.ReadUint8()
			if err != nil {
				return nil, err
			} else if int(fd) >= nPrivate {
				return nil, invalidSince("FDSelect out of range")
			}
			if i > 0 {
				end = append(end, font.GlyphID(first))
			}
			fdIdx = append(fdIdx, fd)
			prev = first
		}
		sentinel, err := p.ReadUint16()
		if err != nil {
			return nil, err
		} else if int(sentinel) != nGlyphs {
			return nil, invalidSince("wrong FDSelect sentinel")
		}
		end = append(end, font.GlyphID(nGlyphs))

		return func(gid font.GlyphID) int {
			idx := sort.Search(int(nRanges),
				func(i int) bool { return gid < end[i] })
			return int(fdIdx[idx])
		}, nil
	default:
		return nil, unsupported(fmt.Sprintf("FDSelect format %d", format))
	}
}

func (fdSelect FdSelectFn) encode(nGlyphs int) []byte {
	format0Length := nGlyphs + 1

	buf := []byte{3, 0, 0}
	var currendFD int
	nSeg := 0
	for i := 0; i < nGlyphs; i++ {
		fd := fdSelect(font.GlyphID(i))
		if i > 0 && fd == currendFD {
			continue
		}
		// new segment
		if len(buf)+3+2 >= format0Length {
			goto useFormat0
		}
		buf = append(buf, byte(i>>8), byte(i), byte(fd))
		nSeg++
		currendFD = fd
	}
	buf = append(buf, byte(nGlyphs>>8), byte(nGlyphs))
	buf[1], buf[2] = byte(nSeg>>8), byte(nSeg)
	return buf

useFormat0:
	buf = make([]byte, nGlyphs+1)
	for i := 0; i < nGlyphs; i++ {
		buf[i+1] = byte(fdSelect(font.GlyphID(i)))
	}
	return buf
}
