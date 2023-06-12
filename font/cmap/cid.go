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
	"bytes"
	"sort"

	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/type1"
)

type CIDEncoder interface {
	Encode(glyph.ID, []rune) []byte
	Encoding() []Record
	CIDSystemInfo() *type1.CIDSystemInfo
}

type Record struct {
	Code []byte
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

func (enc *defaultCIDEncoder) Encode(gid glyph.ID, rr []rune) []byte {
	enc.used[gid] = true
	enc.text[type1.CID(gid)] = rr
	return []byte{byte(gid >> 8), byte(gid)}
}

func (enc *defaultCIDEncoder) Encoding() []Record {
	var encs []Record
	for gid := range enc.used {
		code := []byte{byte(gid >> 8), byte(gid)}
		cid := type1.CID(gid)
		encs = append(encs, Record{code, cid, gid, enc.text[cid]})
	}
	sort.Slice(encs, func(i, j int) bool {
		return bytes.Compare(encs[i].Code, encs[j].Code) < 0
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
