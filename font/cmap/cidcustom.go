// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package cmap

import (
	"sort"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt/glyph"
)

func NewCustomCIDEncoder(CS charcode.CodeSpaceRange, ROS *type1.CIDSystemInfo) CIDEncoderOld {
	enc := &customCIDEncoder{
		CS:       CS,
		ROS:      ROS,
		enc:      make(map[glyph.ID]charcode.CharCode),
		cid:      make(map[charcode.CharCode]type1.CID),
		text:     make(map[charcode.CharCode][]rune),
		NextCode: 0xE000, // unicode private use area
		NextCID:  1,
	}
	return enc
}

type customCIDEncoder struct {
	CS  charcode.CodeSpaceRange
	ROS *type1.CIDSystemInfo

	enc      map[glyph.ID]charcode.CharCode
	cid      map[charcode.CharCode]type1.CID
	text     map[charcode.CharCode][]rune
	NextCode charcode.CharCode
	NextCID  type1.CID
}

func (c *customCIDEncoder) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	code, ok := c.enc[gid]
	if !ok {
		if len(rr) == 1 {
			code = charcode.CharCode(rr[0])
		}
		for code == 0 || c.cid[code] != 0 {
			code = c.NextCode
			c.NextCode++
		}
		c.enc[gid] = code
		c.cid[code] = c.NextCID
		c.NextCID++

		if len(rr) > 0 {
			c.text[code] = rr
		}
	}
	return c.CS.Append(s, code)
}

func (c *customCIDEncoder) Encoding() []Record {
	encs := make([]Record, 0, len(c.enc))
	for gid, code := range c.enc {
		encs = append(encs, Record{
			Code: code,
			CID:  c.cid[code],
			GID:  gid,
			Text: c.text[code],
		})
	}
	sort.Slice(encs, func(i, j int) bool {
		return encs[i].Code < encs[j].Code
	})
	return encs
}

func (c *customCIDEncoder) CIDSystemInfo() *type1.CIDSystemInfo {
	return c.ROS
}
