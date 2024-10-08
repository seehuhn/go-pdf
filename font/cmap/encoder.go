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
	"iter"
	"slices"

	"golang.org/x/exp/maps"

	pscid "seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
)

// CIDEncoder constructs and stores mappings from character codes
// to CID values and from character codes to unicode strings.
type CIDEncoder interface {
	// AppendEncoded appends the character code for the given glyph ID
	// to the given PDF string (allocating new codes as needed).
	// It also records the fact that the character code corresponds to the
	// given unicode string.
	AppendEncoded(pdf.String, glyph.ID, []rune) pdf.String

	// CMap returns the mapping from character codes to CID values.
	CMap() *Info

	// ToUnicode returns a PDF ToUnicode CMap.
	ToUnicode() *ToUnicode

	// Subset is the set of all GIDs which have been used with AppendEncoded.
	// The returned slice is sorted and always starts with GID 0.
	Subset() []glyph.ID

	AllCIDs(pdf.String) iter.Seq2[[]byte, pscid.CID]
}

// NewCIDEncoderIdentity returns an encoder where two-byte codes
// are used directly as CID values.
func NewCIDEncoderIdentity(g2c GIDToCID) CIDEncoder {
	return &identityEncoder{
		g2c:       g2c,
		toUnicode: make(map[charcode.CharCode][]rune),
		used:      make(map[glyph.ID]struct{}),
	}
}

type identityEncoder struct {
	g2c GIDToCID

	toUnicode map[charcode.CharCode][]rune
	used      map[glyph.ID]struct{}
}

func (e *identityEncoder) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	cid := e.g2c.CID(gid, rr)
	code := charcode.CharCode(cid)
	e.toUnicode[code] = rr
	e.used[gid] = struct{}{}
	return charcode.UCS2.Append(s, code)
}

func (e *identityEncoder) CMap() *Info {
	m := make(map[charcode.CharCode]pscid.CID)
	for code := range e.toUnicode {
		m[code] = pscid.CID(code)
	}
	return New(e.g2c.ROS(), charcode.UCS2, m)
}

func (e *identityEncoder) ToUnicode() *ToUnicode {
	return NewToUnicode(charcode.UCS2, e.toUnicode)
}

func (e *identityEncoder) Subset() []glyph.ID {
	_, hasNotDef := e.toUnicode[0]
	subset := maps.Keys(e.used)
	if !hasNotDef {
		subset = append(subset, 0)
	}
	slices.Sort(subset)
	return subset
}

func (e *identityEncoder) AsText(s pdf.String) []rune {
	var res []rune
	cs := charcode.UCS2
	cs.AllCodes(s)(func(code pdf.String, valid bool) bool {
		c, _ := cs.Decode(code)
		if c >= 0 {
			res = append(res, e.toUnicode[c]...)
		}
		return true
	})
	return res
}

func (e *identityEncoder) AllCIDs(s pdf.String) iter.Seq2[[]byte, pscid.CID] {
	return func(yield func([]byte, pscid.CID) bool) {
		for len(s) >= 2 {
			var code []byte
			code, s = s[:2], s[2:]
			cid := pscid.CID(code[0])<<8 | pscid.CID(code[1])
			if !yield(code, cid) {
				return
			}
		}
	}
}

// NewCIDEncoderUTF8 returns an encoder where character codes equal the UTF-8
// encoding of the text content, where possible.
func NewCIDEncoderUTF8(g2c GIDToCID) CIDEncoder {
	return &utf8Encoder{
		g2c:   g2c,
		cache: make(map[key]charcode.CharCode),
		cmap:  make(map[charcode.CharCode]pscid.CID),
		rev:   make(map[pscid.CID]charcode.CharCode),
		next:  0xE000,
	}
}

type utf8Encoder struct {
	g2c GIDToCID

	cache map[key]charcode.CharCode
	cmap  map[charcode.CharCode]pscid.CID
	rev   map[pscid.CID]charcode.CharCode
	next  rune
}

type key struct {
	gid glyph.ID
	rr  string
}

func (e *utf8Encoder) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	s, _ = e.CodeAndCID(s, gid, rr)
	return s
}

func (e *utf8Encoder) CodeAndCID(s pdf.String, gid glyph.ID, rr []rune) (pdf.String, pscid.CID) {
	cid := e.g2c.CID(gid, rr)
	k := key{gid, string(rr)}

	// Rules for choosing the code:
	// 1. If the combination of `gid` and `rr` has previously been used,
	//    then use the same code as before.
	code, valid := e.cache[k]
	if valid {
		return utf8cs.Append(s, code), cid
	}

	// 2. If rr has length 1, and if rr has not previously been paired with a
	//    different gid, then use rr[0] as the code.
	if len(rr) == 1 {
		code = runeToCode(rr[0])
		if cidCandidate, ok := e.cmap[code]; !ok || cid == cidCandidate {
			valid = true
		}
	}

	// 3. Otherwise, allocate a new code from the unicode private use area.
	if !valid {
		code = runeToCode(e.next)
		e.next++
		if e.next == 0x00_F900 {
			e.next = 0x0F_0000
		}
	}

	e.cache[k] = code
	e.cmap[code] = cid
	e.rev[cid] = code
	return utf8cs.Append(s, code), cid
}

func runeToCode(r rune) charcode.CharCode {
	code := charcode.CharCode(r)
	if code >= 0x01_0000 {
		code += 0x01_0000 + 0x0800 + 0x0080
	} else if code >= 0x00_0800 {
		code += 0x0800 + 0x0080
	} else if code >= 0x00_0080 {
		code += 0x0080
	}
	return code
}

func (e *utf8Encoder) CMap() *Info {
	return New(e.g2c.ROS(), utf8cs, e.cmap)
}

func (e *utf8Encoder) ToUnicode() *ToUnicode {
	toUnicode := make(map[charcode.CharCode][]rune)
	for k, v := range e.cache {
		toUnicode[v] = []rune(k.rr)
	}
	return NewToUnicode(utf8cs, toUnicode)
}

func (e *utf8Encoder) Subset() []glyph.ID {
	used := make(map[glyph.ID]bool, len(e.cache)+1)
	used[0] = true
	for k := range e.cache {
		used[k.gid] = true
	}
	subset := maps.Keys(used)
	slices.Sort(subset)
	return subset
}

func (e *utf8Encoder) AsText(s pdf.String) []rune {
	return []rune(string(s))
}

func (e *utf8Encoder) AllCIDs(s pdf.String) iter.Seq2[[]byte, pscid.CID] {
	return func(yield func([]byte, pscid.CID) bool) {
		utf8cs.AllCodes(s)(func(code pdf.String, valid bool) bool {
			c, _ := utf8cs.Decode(code)
			return yield(code, e.cmap[c])
		})
	}
}

// utf8cs represents UTF-8-encoded character codes.
var utf8cs = charcode.CodeSpaceRange{
	{Low: []byte{0x00}, High: []byte{0x7F}},
	{Low: []byte{0xC0, 0x80}, High: []byte{0xDF, 0xBF}},
	{Low: []byte{0xE0, 0x80, 0x80}, High: []byte{0xEF, 0xBF, 0xBF}},
	{Low: []byte{0xF0, 0x80, 0x80, 0x80}, High: []byte{0xF7, 0xBF, 0xBF, 0xBF}},
}
