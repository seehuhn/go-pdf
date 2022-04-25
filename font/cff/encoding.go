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

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/type1"
)

func readEncoding(p *parser.Parser, charset []int32) ([]font.GlyphID, error) {
	format, err := p.ReadUint8()
	if err != nil {
		return nil, err
	}

	res := make([]font.GlyphID, 256)
	currentGid := font.GlyphID(1)
	switch format & 127 {
	case 0:
		nCodes, err := p.ReadUint8()
		if err != nil {
			return nil, err
		}
		if int(nCodes) >= len(charset) {
			return nil, invalidSince("format 0 encoding too long")
		}
		codes := make([]byte, nCodes)
		_, err = io.ReadFull(p, codes)
		if err != nil {
			return nil, err
		}
		for _, c := range codes {
			if res[c] != 0 {
				return nil, invalidSince("invalid format 0 encoding")
			}
			res[c] = currentGid
			currentGid++
		}
	case 1:
		nRanges, err := p.ReadUint8()
		if err != nil {
			return nil, err
		}
		for i := 0; i < int(nRanges); i++ {
			first, err := p.ReadUint8()
			if err != nil {
				return nil, err
			}
			nLeft, err := p.ReadUint8()
			if err != nil {
				return nil, err
			}
			if int(first)+int(nLeft) > 255 {
				return nil, invalidSince("invalid format 1 encoding")
			}
			for j := int(first); j <= int(first+nLeft); j++ {
				if int(currentGid) >= len(charset) {
					return nil, invalidSince("format 1 encoding too long")
				} else if res[j] != 0 {
					return nil, invalidSince("invalid format 1 encoding")
				}
				res[j] = currentGid
				currentGid++
			}
		}
	default:
		return nil, unsupported(fmt.Sprintf("encoding format %d", format&127))
	}

	if (format & 128) != 0 {
		lookup := make(map[uint16]font.GlyphID)
		for gid, sid := range charset {
			lookup[uint16(sid)] = font.GlyphID(gid)
		}
		nSups, err := p.ReadUint8()
		if err != nil {
			return nil, err
		}
		for i := 0; i < int(nSups); i++ {
			code, err := p.ReadUint8()
			if err != nil {
				return nil, err
			} else if res[code] != 0 {
				return nil, invalidSince("invalid encoding supplement")
			}
			sid, err := p.ReadUint16()
			if err != nil {
				return nil, err
			}
			gid := lookup[sid]
			if gid >= currentGid {
				return nil, invalidSince("invalid encoding supplement")
			}
			if gid != 0 {
				res[code] = gid
			}
		}
	}

	return res, nil
}

func encodeEncoding(encoding []font.GlyphID, cc []int32) ([]byte, error) {
	var maxGid font.GlyphID
	codes := map[font.GlyphID]uint8{}
	type suppl struct {
		code uint8
		gid  font.GlyphID
	}
	var extra []suppl
	for code, gid := range encoding {
		if gid == 0 {
			continue
		}
		c8 := uint8(code)
		if _, ok := codes[gid]; ok {
			extra = append(extra, suppl{c8, gid})
			continue
		}
		codes[gid] = c8
		if gid > maxGid {
			maxGid = gid
		}
	}

	type seg struct {
		firstCode uint8
		nLeft     uint8
	}
	var ss []seg

	startGid := font.GlyphID(1)
	startCode := codes[startGid]
	for gid := font.GlyphID(1); gid <= maxGid; gid++ {
		code, ok := codes[gid]
		if !ok {
			msg := fmt.Sprintf("glyph %d not in encoding", gid)
			return nil, invalidSince(msg)
		}
		if int(gid-startGid) != int(code)-int(startCode) {
			ss = append(ss, seg{startCode, uint8(gid - startGid - 1)})
			startGid = gid
			startCode = code
		}
	}
	ss = append(ss, seg{startCode, uint8(maxGid - startGid)})
	if len(ss) > 255 {
		return nil, invalidSince("too many segments")
	}

	format0Len := 2 + int(maxGid)
	format1Len := 2 + len(ss)*2
	extraLen := 0
	if len(extra) > 0 {
		extraLen = 1 + 3*len(extra)
	}
	var buf []byte
	var extraBase int
	if format0Len <= format1Len && maxGid <= 255 {
		extraBase = format0Len
		buf = make([]byte, format0Len+extraLen)
		// buf[0] = 0
		buf[1] = byte(maxGid)
		for i := font.GlyphID(1); i <= maxGid; i++ {
			buf[i+1] = codes[i]
		}
	} else {
		extraBase = format1Len
		buf = make([]byte, format1Len+extraLen)
		buf[0] = 1
		buf[1] = byte(len(ss))
		for i, s := range ss {
			buf[2+i*2] = s.firstCode
			buf[2+i*2+1] = s.nLeft
		}
	}

	if len(extra) > 0 {
		buf[0] |= 128
		buf[extraBase] = byte(len(extra))
		for i, s := range extra {
			buf[extraBase+i*3+1] = s.code
			sid := uint16(cc[s.gid])
			buf[extraBase+i*3+2] = byte(sid >> 8)
			buf[extraBase+i*3+3] = byte(sid)
		}
	}

	return buf, nil
}

func standardEncoding(glyphs []*Glyph) []font.GlyphID {
	encoding := make([]font.GlyphID, 256)
	for gid, g := range glyphs {
		code, ok := type1.StandardEncoding[g.Name]
		if ok {
			encoding[code] = font.GlyphID(gid)
		}
	}
	return encoding
}

func isStandardEncoding(encoding []font.GlyphID, glyphs []*Glyph) bool {
	tmp := standardEncoding(glyphs)
	for i, gid := range tmp {
		if encoding[i] != gid {
			return false
		}
	}
	return true
}

func expertEncoding(glyphs []*Glyph) []font.GlyphID {
	res := make([]font.GlyphID, 256)
	for gid, g := range glyphs {
		code, ok := type1.ExpertEncoding[g.Name]
		if ok {
			res[code] = font.GlyphID(gid)
		}
	}
	return res
}

func isExpertEncoding(encoding []font.GlyphID, glyphs []*Glyph) bool {
	tmp := expertEncoding(glyphs)
	for i, gid := range tmp {
		if encoding[i] != gid {
			return false
		}
	}
	return true
}
