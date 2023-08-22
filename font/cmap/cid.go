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

type CIDEncoder interface {
	AppendEncoded(pdf.String, glyph.ID, []rune) pdf.String

	Encoding() []Record
	CIDSystemInfo() *type1.CIDSystemInfo
}

type Record struct {
	Code charcode.CharCode
	CID  type1.CID
	GID  glyph.ID
	Text []rune
}

func NewCIDEncoder() CIDEncoder {
	enc := &defaultCIDEncoder{
		used: make(map[glyph.ID]bool),
		text: make(map[type1.CID][]rune),
	}
	return enc
}

type defaultCIDEncoder struct {
	used map[glyph.ID]bool
	text map[type1.CID][]rune
}

func (enc *defaultCIDEncoder) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	enc.used[gid] = true
	enc.text[type1.CID(gid)] = rr
	return append(s, byte(gid>>8), byte(gid))
}

func (enc *defaultCIDEncoder) Encoding() []Record {
	var encs []Record
	for gid := range enc.used {
		cid := type1.CID(gid)
		encs = append(encs, Record{charcode.CharCode(gid), cid, gid, enc.text[cid]})
	}
	sort.Slice(encs, func(i, j int) bool {
		return encs[i].Code < encs[j].Code
	})
	return encs
}

func (enc *defaultCIDEncoder) CIDSystemInfo() *type1.CIDSystemInfo {
	// TODO(voss): is this right?
	return &type1.CIDSystemInfo{
		Registry:   "Adobe",
		Ordering:   "Identity",
		Supplement: 0,
	}
}
