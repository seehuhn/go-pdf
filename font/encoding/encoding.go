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

package encoding

import (
	"math/bits"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/postscript/type1/names"
	"seehuhn.de/go/sfnt/glyph"
)

// SimpleEncoder constructs and stores mappings from one-byte character codes
// to GID values and from one-byte character codes to unicode strings.
type SimpleEncoder struct {
	Encoding []glyph.ID
	code     map[key]byte
	key      map[byte]key
}

type key struct {
	gid glyph.ID
	rr  string
}

// NewSimpleEncoder allocates a new SimpleEncoder.
func NewSimpleEncoder() *SimpleEncoder {
	res := &SimpleEncoder{
		Encoding: make([]glyph.ID, 256),
		code:     make(map[key]byte),
		key:      make(map[byte]key),
	}
	return res
}

// AppendEncoded appends the character code for the given glyph ID
// to the given PDF string (allocating new codes as needed).
// It also records the fact that the character code corresponds to the
// given unicode string.
func (e *SimpleEncoder) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	k := key{gid, string(rr)}

	// Rules for choosing the code:
	// 1. If the combination of `gid` and `rr` has previously been used,
	//    then use the same code as before.
	code, seen := e.code[k]
	if seen {
		return append(s, code)
	}

	// 2. Allocate a new code based on the last rune in rr.
	var r rune
	if len(rr) > 0 {
		r = rr[len(rr)-1]
	}
	code = e.allocateCode(r)
	e.Encoding[code] = gid
	e.code[k] = code
	e.key[code] = k
	return append(s, code)
}

func (e *SimpleEncoder) allocateCode(r rune) byte {
	if len(e.code) >= 256 {
		// Once all codes are used up, simply return 0 for everything.
		return 0
	}
	bestScore := -1
	bestCode := byte(0)
	for codeInt := 0; codeInt < 256; codeInt++ {
		code := byte(codeInt)
		if _, alreadyUsed := e.key[code]; alreadyUsed {
			continue
		}
		var score int
		q := rune(code)
		stdName := pdfenc.StandardEncoding[code]
		if stdName == ".notdef" {
			// fill up the unused slots first
			score += 100
		} else {
			q = names.ToUnicode(stdName, false)[0]
			if q == r {
				// If r is in the standard encoding, and the corresponding
				// code is still free, then use it.
				bestCode = code
				break
			} else if !(code == 32 && r != ' ') {
				// Try to keep code 32 for the space character,
				// in order to not break the PDF word spacing parameter.
				score += 10
			}
		}
		score += bits.TrailingZeros16(uint16(r) ^ uint16(q))
		if score > bestScore {
			bestScore = score
			bestCode = code
		}
	}
	return bestCode
}

// Overflow returns true if the encoder has run out of codes.
func (e *SimpleEncoder) Overflow() bool {
	return len(e.code) > 256
}

// Subset returns the subset of glyph IDs which are used by this encoder.
// The result is sorted and always include the glyph ID 0.
func (e *SimpleEncoder) Subset() []glyph.ID {
	gidUsed := make(map[glyph.ID]bool, len(e.code)+1)
	gidUsed[0] = true
	for key := range e.code {
		gidUsed[key.gid] = true
	}
	subset := maps.Keys(gidUsed)
	slices.Sort(subset)
	return subset
}

// ToUnicode returns the mapping from character codes to unicode strings.
// This can be used to construct a PDF ToUnicode CMap.
func (e *SimpleEncoder) ToUnicode() map[charcode.CharCode][]rune {
	toUnicode := make(map[charcode.CharCode][]rune)
	for k, v := range e.code {
		toUnicode[charcode.CharCode(v)] = []rune(k.rr)
	}
	return toUnicode
}

// WritingMode implements the [font.NewFont] interface.
func (e *SimpleEncoder) WritingMode() int {
	return 0 // simple fonts are always horizontal
}

func (e *SimpleEncoder) AsText(s pdf.String) []rune {
	var res []rune
	for _, c := range s {
		k, ok := e.key[c]
		if ok {
			res = append(res, []rune(k.rr)...)
		}
	}
	return res
}
