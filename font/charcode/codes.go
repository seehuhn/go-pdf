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

package charcode

import (
	"unicode/utf8"

	"seehuhn.de/go/pdf"
)

type CharCode int

type CodeSpaceRange interface {
	Append(pdf.String, CharCode) pdf.String
	Decode(pdf.String) (CharCode, int)
	Ranges() []Range
}

type Range struct {
	Low, High []byte
}

func (r Range) Matches(s pdf.String) bool {
	if len(s) < len(r.Low) {
		return false
	}
	for i, low := range r.Low {
		if s[i] < low {
			return false
		}
		if s[i] > r.High[i] {
			return false
		}
	}
	return true
}

// Simple represents the code space range for a simple font.
// Character codes are one byte long, and are directly mapped to
// the bytes of the PDF string.
var Simple CodeSpaceRange = &simpleCS{}

type simpleCS struct{}

func (c *simpleCS) Append(s pdf.String, code CharCode) pdf.String {
	return append(s, byte(code))
}

func (c *simpleCS) Decode(s pdf.String) (CharCode, int) {
	if len(s) == 0 {
		return -1, 0
	}
	return CharCode(s[0]), 1
}

func (c *simpleCS) Ranges() []Range {
	return []Range{{[]byte{0x00}, []byte{0xFF}}}
}

// UCS2 represents a two-byte encoding.
// Character codes are two bytes long, and are stored in big-endian order.
var UCS2 CodeSpaceRange = &ucs2CS{}

type ucs2CS struct{}

func (c *ucs2CS) Append(s pdf.String, code CharCode) pdf.String {
	return append(s, byte(code>>8), byte(code))
}

func (c *ucs2CS) Decode(s pdf.String) (CharCode, int) {
	switch len(s) {
	case 0:
		return -1, 0
	case 1:
		return -1, 1
	}
	return CharCode(s[0])<<8 | CharCode(s[1]), 2
}

func (c *ucs2CS) Ranges() []Range {
	return []Range{{[]byte{0x00, 0x00}, []byte{0xFF, 0xFF}}}
}

// UTF8 represents UTF-8-encoded character codes.
var UTF8 CodeSpaceRange = &utf8CS{}

type utf8CS struct{}

func (c *utf8CS) Append(s pdf.String, code CharCode) pdf.String {
	buf := utf8.AppendRune([]byte(s), rune(code))
	return pdf.String(buf)
}

func (c *utf8CS) Decode(s pdf.String) (CharCode, int) {
	r, size := utf8.DecodeRune([]byte(s))
	if r == utf8.RuneError {
		r = -1
	}
	return CharCode(r), size
}

func (c *utf8CS) Ranges() []Range {
	return []Range{
		{[]byte{0x00}, []byte{0x7F}},
		{[]byte{0xC2, 0x80}, []byte{0xDF, 0xBF}},
		{[]byte{0xE0, 0x80, 0x80}, []byte{0xEF, 0xBF, 0xBF}},
		{[]byte{0xF0, 0x80, 0x80, 0x80}, []byte{0xF7, 0xBF, 0xBF, 0xBF}},
	}
}
