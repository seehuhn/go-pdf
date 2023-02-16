package cmap

import (
	"bytes"
	"sort"

	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/type1"
)

// character code (sequence of bytes) -> CID -> glyph identifier (GID)

// A character identifier (CID) gives the index of character in a character
// collection. The character collection is specified by the CIDSystemInfo
// dictionary.
//
// A CMap specifies a mapping from character codes to a font number (always 0
// for PDF), and a character selector (the CID).

type CIDEncoder interface {
	Encode(glyph.ID, []rune) []byte
	Encoding() []Record
	CIDSystemInfo() *type1.CIDSystemInfo
}

type Record struct {
	Code []byte
	CID  CID
	GID  glyph.ID
	Text []rune
}

func NewCIDEncoder() CIDEncoder {
	enc := &defaultCIDEncoder{
		used: make(map[glyph.ID]bool),
		text: make(map[CID][]rune),
	}
	return enc
}

type defaultCIDEncoder struct {
	used map[glyph.ID]bool
	text map[CID][]rune
}

func (enc *defaultCIDEncoder) Encode(gid glyph.ID, rr []rune) []byte {
	enc.used[gid] = true
	enc.text[CID(gid)] = rr
	return []byte{byte(gid >> 8), byte(gid)}
}

func (enc *defaultCIDEncoder) Encoding() []Record {
	var encs []Record
	for gid := range enc.used {
		code := []byte{byte(gid >> 8), byte(gid)}
		cid := CID(gid)
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
