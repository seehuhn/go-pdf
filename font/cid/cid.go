// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package cid

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"hash"

	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/postscript/type1/names"
)

type NewCID uint32

type System interface {
	// ROS returns the identifier Registry, Ordering and Supplement
	// corresponding to this CIDSystemInfo.
	ROS() *cmap.CIDSystemInfo

	// This returns the glyph name corresponding to the given CID.
	// If the glyph name is unknown, this returns the empty string.
	GlyphName(cid NewCID) string

	// GlyphText returns the text represented by the glyph corresponding to the
	// given CID.
	GlyphText(cid NewCID) string
}

type Simple struct {
	Glyphs []simplePair
}

type simplePair struct {
	Name string
	Text string
}

func NewSimple() *Simple {
	return &Simple{}
}

func (s *Simple) MakeCID(name string, text string) NewCID {
	panic("not implemented") // TODO: Implement
}

// ROS returns the identifier Registry, Ordering and Supplement
// corresponding to this CIDSystemInfo.
// This implements the [System] interface.
func (s *Simple) ROS() *cmap.CIDSystemInfo {
	h := md5.New()
	s.writeBinary(h)
	ordering := hex.EncodeToString(h.Sum(nil))

	return &cmap.CIDSystemInfo{
		Registry:   "seehuhn.de",
		Ordering:   ordering,
		Supplement: 0,
	}
}

// This returns the glyph name corresponding to the given CID.
// If the glyph name is unknown, this returns the empty string.
// This implements the [System] interface.
func (s *Simple) GlyphName(cid NewCID) string {
	code := int(cid & 0xFF)
	switch cid >> 24 {
	case simpleIsWinAnsi:
		return pdfenc.WinAnsi.Encoding[code]
	case simpleIsMacRoman:
		return pdfenc.MacRoman.Encoding[code]
	case simpleIsMacExpert:
		return pdfenc.MacExpert.Encoding[code]
	case simpleIsPair:
		if int(code) < len(s.Glyphs) {
			return s.Glyphs[code].Name
		}
	}
	return ""
}

// GlyphText returns the text represented by the glyph corresponding to the
// given CID.
// This implements the [System] interface.
func (s *Simple) GlyphText(cid NewCID) string {
	code := int(cid & 0xFF)

	var name string
	switch cid >> 24 {
	case simpleIsWinAnsi:
		name = pdfenc.WinAnsi.Encoding[code]
	case simpleIsMacRoman:
		name = pdfenc.MacRoman.Encoding[code]
	case simpleIsMacExpert:
		name = pdfenc.MacExpert.Encoding[code]
	case simpleIsPair:
		if int(code) < len(s.Glyphs) {
			return s.Glyphs[code].Text
		}
	}
	if name != "" {
		// TODO(voss): how to support Dingbats?
		rr := names.ToUnicode(name, false)
		if len(rr) > 0 {
			return string(rr)
		}
	}
	return ""
}

// writeBinary writes a binary representation of the Simple object to
// the [hash.Hash] h.
func (s *Simple) writeBinary(h hash.Hash) {
	// h.Write is guaranteed to never return an error

	const magic uint32 = 0x6c26b5be
	binary.Write(h, binary.BigEndian, magic)

	var buf [binary.MaxVarintLen64]byte
	writeInt := func(x int) {
		k := binary.PutUvarint(buf[:], uint64(x))
		h.Write(buf[:k])
	}
	writeString := func(x string) {
		b := []byte(x)
		writeInt(len(b))
		h.Write(b)
	}

	writeInt(len(s.Glyphs))
	for _, pair := range s.Glyphs {
		writeString(pair.Name)
		writeString(pair.Text)
	}
}

const (
	simpleIsBuiltin NewCID = iota
	simpleIsWinAnsi
	simpleIsMacRoman
	simpleIsMacExpert
	simpleIsPair
)
