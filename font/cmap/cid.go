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
	"slices"
	"sort"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/sfnt/glyph"
)

// Encoder constructs and stores the mappings from character codes
// to CID values and from character codes to unicode strings.
type Encoder interface {
	// AppendEncoded appends the character code for the given glyph ID
	// to the given PDF string (allocating new codes as needed).
	// It also records that the character code
	// used corresponds to the given unicode string.
	AppendEncoded(pdf.String, glyph.ID, []rune) pdf.String

	// CMap returns the information needed to construct a PDF CMap.
	CMap() map[charcode.CharCode]type1.CID

	// ToUnicode returns the mapping from character codes to unicode strings.
	ToUnicode() map[charcode.CharCode][]rune

	// CodeSpaceRange returns the range of character codes which are
	// used by this encoder.
	CodeSpaceRange() charcode.CodeSpaceRange

	// Return the set of all GIDs which have been used with AppendEncoded.
	// The returned slice is sorted and always includes GID 0.
	UsedGIDs() []glyph.ID
}

// NewIdentityEncoder returns an encoder where CID values equal the
// corresponding glyph ID.
func NewIdentityEncoder(g2c GIDToCID) Encoder {
	return &identityEncoder{
		toUnicode: make(map[charcode.CharCode][]rune),
		used:      make(map[glyph.ID]struct{}),
	}
}

type identityEncoder struct {
	toUnicode map[charcode.CharCode][]rune
	used      map[glyph.ID]struct{}
}

func (e *identityEncoder) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	code := charcode.CharCode(gid)
	e.toUnicode[code] = rr
	e.used[gid] = struct{}{}
	return charcode.UCS2.Append(s, code)
}

func (e *identityEncoder) CMap() map[charcode.CharCode]type1.CID {
	cmap := make(map[charcode.CharCode]type1.CID)
	for code := range e.toUnicode {
		cmap[code] = type1.CID(code)
	}
	return cmap
}

func (e *identityEncoder) ToUnicode() map[charcode.CharCode][]rune {
	return e.toUnicode
}

func (e *identityEncoder) CodeSpaceRange() charcode.CodeSpaceRange {
	return charcode.UCS2
}

func (e *identityEncoder) UsedGIDs() []glyph.ID {
	_, hasNotDef := e.toUnicode[0]
	subset := maps.Keys(e.used)
	if !hasNotDef {
		subset = append(subset, 0)
	}
	slices.Sort(subset)
	return subset
}

func NewUTF8Encoder(g2c GIDToCID) Encoder {
	return &utf8Encoder{
		g2c:   g2c,
		cache: make(map[key]charcode.CharCode),
		cmap:  make(map[charcode.CharCode]type1.CID),
		next:  0xE000,
	}
}

type key struct {
	gid glyph.ID
	rr  string
}

type utf8Encoder struct {
	g2c GIDToCID

	cache map[key]charcode.CharCode
	cmap  map[charcode.CharCode]type1.CID
	next  rune
}

func (e *utf8Encoder) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	k := key{gid, string(rr)}

	// Rules for choosing the code:
	// 1. If the combination of `gid` and `rr` has previously been used,
	//    then use the same code as before.
	code, valid := e.cache[k]
	if valid {
		return utf8cs.Append(s, code)
	}

	// 2. If rr has length 1, and if rr has not previously been paired with a
	//    different gid, then use rr[0] as the code.
	if len(rr) == 1 {
		code = runeToCode(rr[0])
		if cid, ok := e.cmap[code]; !ok || e.g2c.CID(gid) == cid {
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
	e.cmap[code] = e.g2c.CID(gid)
	return utf8cs.Append(s, code)
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

func (e *utf8Encoder) CMap() map[charcode.CharCode]type1.CID {
	return e.cmap
}

func (e *utf8Encoder) ToUnicode() map[charcode.CharCode][]rune {
	toUnicode := make(map[charcode.CharCode][]rune)
	for k, v := range e.cache {
		toUnicode[v] = []rune(k.rr)
	}
	return toUnicode
}

func (e *utf8Encoder) CodeSpaceRange() charcode.CodeSpaceRange {
	return utf8cs
}

func (e *utf8Encoder) UsedGIDs() []glyph.ID {
	used := make(map[glyph.ID]bool, len(e.cache)+1)
	used[0] = true
	for k := range e.cache {
		used[k.gid] = true
	}
	subset := maps.Keys(used)
	slices.Sort(subset)
	return subset
}

// utf8cs represents UTF-8-encoded character codes.
var utf8cs = charcode.CodeSpaceRange{
	{Low: []byte{0x00}, High: []byte{0x7F}},
	{Low: []byte{0xC0, 0x80}, High: []byte{0xDF, 0xBF}},
	{Low: []byte{0xE0, 0x80, 0x80}, High: []byte{0xEF, 0xBF, 0xBF}},
	{Low: []byte{0xF0, 0x80, 0x80, 0x80}, High: []byte{0xF7, 0xBF, 0xBF, 0xBF}},
}

type GIDToCID interface {
	CID(glyph.ID) type1.CID

	ROS() *type1.CIDSystemInfo

	GIDToCID(numGlyph int) []type1.CID
}

func NewGIDToCIDSequential() GIDToCID {
	return &gidToCIDSequential{
		data: make(map[glyph.ID]type1.CID),
	}
}

type gidToCIDSequential struct {
	data map[glyph.ID]type1.CID
}

func (g *gidToCIDSequential) CID(gid glyph.ID) type1.CID {
	cid, ok := g.data[gid]
	if !ok {
		cid = type1.CID(len(g.data) + 1)
		g.data[gid] = cid
	}
	return cid
}

func (g *gidToCIDSequential) ROS() *type1.CIDSystemInfo {
	return &type1.CIDSystemInfo{
		Registry:   "Seehuhn",
		Ordering:   "Sonderbar", // TODO(voss)
		Supplement: 0,
	}
}

func (g *gidToCIDSequential) GIDToCID(numGlyph int) []type1.CID {
	res := make([]type1.CID, numGlyph)
	for gid, cid := range g.data {
		res[gid] = cid
	}
	return res
}

func NewGIDToCIDIdentity() GIDToCID {
	return &gidToCIDIdentity{}
}

type gidToCIDIdentity struct{}

func (g *gidToCIDIdentity) CID(gid glyph.ID) type1.CID {
	return type1.CID(gid)
}

func (g *gidToCIDIdentity) ROS() *type1.CIDSystemInfo {
	return &type1.CIDSystemInfo{
		Registry:   "Adobe",
		Ordering:   "Identity",
		Supplement: 0,
	}
}

func (g *gidToCIDIdentity) GIDToCID(numGlyph int) []type1.CID {
	res := make([]type1.CID, numGlyph)
	for i := range res {
		res[i] = type1.CID(i)
	}
	return res
}

// Old versions ==============================================================

// TODO(voss): remove
type CIDEncoderOld interface {
	AppendEncoded(pdf.String, glyph.ID, []rune) pdf.String

	Encoding() []Record
	CIDSystemInfo() *type1.CIDSystemInfo
}

// TODO(voss): remove
type Record struct {
	Code charcode.CharCode
	CID  type1.CID
	GID  glyph.ID
	Text []rune
}

// TODO(voss): remove
func NewCIDEncoderOld() CIDEncoderOld {
	enc := &defaultCIDEncoderOld{
		used: make(map[glyph.ID]bool),
		text: make(map[type1.CID][]rune),
	}
	return enc
}

// TODO(voss): remove
type defaultCIDEncoderOld struct {
	used map[glyph.ID]bool
	text map[type1.CID][]rune
}

func (enc *defaultCIDEncoderOld) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	enc.used[gid] = true
	enc.text[type1.CID(gid)] = rr
	return append(s, byte(gid>>8), byte(gid))
}

func (enc *defaultCIDEncoderOld) Encoding() []Record {
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

func (enc *defaultCIDEncoderOld) CIDSystemInfo() *type1.CIDSystemInfo {
	// TODO(voss): is this right?
	return &type1.CIDSystemInfo{
		Registry:   "Adobe",
		Ordering:   "Identity",
		Supplement: 0,
	}
}
