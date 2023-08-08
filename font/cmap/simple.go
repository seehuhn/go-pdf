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

import "seehuhn.de/go/sfnt/glyph"

type SimpleEncoder interface {
	// TODO(voss): change the signature to
	// AppendEncoded(pdf.String, glyph.ID, []rune) pdf.String
	Encode(glyph.ID, []rune) byte

	Overflow() bool
	Encoding() []glyph.ID
}

// TODO(voss): try different encoders

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

func NewSimpleEncoderSequential() SimpleEncoder {
	enc := &sequential{
		codeLookup: make(map[glyph.ID]byte),
	}
	return enc
}

type sequential struct {
	codeLookup map[glyph.ID]byte
	numUsed    int
}

func (enc *sequential) Encode(gid glyph.ID, rr []rune) byte {
	if c, alreadyAllocated := enc.codeLookup[gid]; alreadyAllocated {
		return c
	}

	c := byte(enc.numUsed)
	enc.numUsed++

	enc.codeLookup[gid] = c
	return c
}

func (enc *sequential) Overflow() bool {
	return enc.numUsed > 256
}

func (enc *sequential) Encoding() []glyph.ID {
	res := make([]glyph.ID, 256)
	for gid, c := range enc.codeLookup {
		res[c] = gid
	}
	return res
}

type frozenSimpleEncoder struct {
	toCode   map[glyph.ID]byte
	fromCode []glyph.ID
}

func NewFrozenSimpleEncoder(enc SimpleEncoder) frozenSimpleEncoder {
	fromCode := enc.Encoding()
	toCode := make(map[glyph.ID]byte)
	for c, gid := range fromCode {
		if gid == 0 {
			continue
		}
		toCode[gid] = byte(c)
	}
	return frozenSimpleEncoder{toCode: toCode, fromCode: fromCode}
}

func (enc frozenSimpleEncoder) Encode(gid glyph.ID, _ []rune) byte {
	c, ok := enc.toCode[gid]
	if !ok {
		panic("glyphs cannot be added after a font has been closed")
	}
	return c
}

func (enc frozenSimpleEncoder) Overflow() bool {
	return false
}

func (enc frozenSimpleEncoder) Encoding() []glyph.ID {
	return enc.fromCode
}
