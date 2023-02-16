package cmap

import "seehuhn.de/go/sfnt/glyph"

type CIDEncoder interface {
	Encode(glyph.ID, []rune) []byte
	Encoding() []Pair
}

type Pair struct {
	Code []byte
	CID  uint32
}

func NewCIDEncoder() CIDEncoder {
	enc := &defaultCIDEncoder{
		used: make(map[glyph.ID]bool),
	}
	enc.used[0] = true // always include .notdef
	return enc
}

type defaultCIDEncoder struct {
	used map[glyph.ID]bool
}

func (enc *defaultCIDEncoder) Encode(gid glyph.ID, _ []rune) []byte {
	enc.used[gid] = true
	return []byte{byte(gid >> 8), byte(gid)}
}

func (enc *defaultCIDEncoder) Encoding() []Pair {
	var encs []Pair
	for gid := range enc.used {
		encs = append(encs, Pair{[]byte{byte(gid >> 8), byte(gid)}, uint32(gid)})
	}
	return encs
}

// sequence of bytes (character code) -> character identifier (CID) -> glyph identifier (GID)

// A character identifier gives the index of character in a character
// collection. The character collection is specified by the CIDSystemInfo
// dictionary.
//
// A CMap specifies a mapping from character codes to a font number (always 0
// for PDF), and a character selector (the CID).
// The Adobe-Identity-0 ROS uses (32 bit) unicode character codes as CID values.
