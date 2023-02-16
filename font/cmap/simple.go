package cmap

import "seehuhn.de/go/sfnt/glyph"

type SimpleEncoder interface {
	Encode(glyph.ID, []rune) byte
	Overflow() bool
	Encoding() []glyph.ID
}

func NewSimpleEncoder() SimpleEncoder {
	enc := &keepAscii{
		codeLookup: make(map[glyph.ID]byte),
		codeIsUsed: make(map[byte]bool),
	}
	return enc
}

type keepAscii struct {
	codeLookup map[glyph.ID]byte
	codeIsUsed map[byte]bool
}

func (enc *keepAscii) Encode(gid glyph.ID, rr []rune) byte {
	if c, alreadyAllocated := enc.codeLookup[gid]; alreadyAllocated {
		return c
	}

	c := enc.selectNewCharCode(gid, rr)
	enc.codeLookup[gid] = c
	enc.codeIsUsed[c] = true
	return c
}

func (enc *keepAscii) selectNewCharCode(gid glyph.ID, rr []rune) byte {
	// Try to keep the PDF somewhat human-readable.
	if len(rr) == 1 {
		r := rr[0]
		if r > 0 && r < 128 && !enc.codeIsUsed[byte(r)] {
			return byte(r)
		}
	}

	// If we need to allocate a new code, first try codes which don't clash
	// with the ASCII range.
	for c := 128; c < 256; c++ {
		if !enc.codeIsUsed[byte(c)] {
			return byte(c)
		}
	}

	// If this isn't enough, use all codes available.
	for c := 127; c > 0; c-- {
		if !enc.codeIsUsed[byte(c)] {
			return byte(c)
		}
	}

	// In case we run out of codes, we map everything to zero here and
	// return an error in [fontDict.Write].
	return 0
}

func (enc *keepAscii) Overflow() bool {
	return len(enc.codeLookup) > 256
}

func (enc *keepAscii) Encoding() []glyph.ID {
	res := make([]glyph.ID, 256)
	for gid, c := range enc.codeLookup {
		res[c] = gid
	}
	return res
}
