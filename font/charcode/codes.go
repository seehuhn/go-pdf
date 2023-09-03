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
	"seehuhn.de/go/pdf"
)

// CharCode represents a character code within a [CodeSpaceRange] as a non-negative integer.
type CharCode int

// CodeSpaceRange describes the ranges of byte sequences which are valid
// character codes for a given encoding.
type CodeSpaceRange []Range

// Append appends the given character code to the given PDF string.
func (c CodeSpaceRange) Append(s pdf.String, code CharCode) pdf.String {
	for _, r := range c {
		if numCodes := r.numCodes(); code >= numCodes {
			code -= numCodes
			continue
		}

		n := len(s)
		for range r.Low {
			s = append(s, 0)
		}
		for i := len(r.Low) - 1; i >= 0; i-- {
			k := CharCode(r.High[i]) - CharCode(r.Low[i]) + 1
			s[n+i] = r.Low[i] + byte(code%k)
			code /= k
		}
		break
	}
	return s
}

// Decode decodes the first character code from the given PDF string.
// It returns the character code and the number of bytes consumed.
// If the character code cannot be decoded, a code of -1 is returned,
// and the length is either 0 (if the string is empty) or 1.
func (c CodeSpaceRange) Decode(s pdf.String) (CharCode, int) {
	var base CharCode
tryNextRange:
	for _, r := range c {
		numCodes := r.numCodes()
		if len(s) < len(r.Low) {
			base += numCodes
			continue tryNextRange
		}

		var code CharCode
		for i := 0; i < len(r.Low); i++ {
			b := s[i]
			if b < r.Low[i] || b > r.High[i] {
				base += numCodes
				continue tryNextRange
			}

			k := CharCode(r.High[i]) - CharCode(r.Low[i]) + 1
			code = code*k + CharCode(b-r.Low[i])
		}
		return code + base, len(r.Low)
	}

	if len(s) == 0 {
		return -1, 0
	}
	return -1, 1
}

// Range represents a range of character codes.
// The range is inclusive, i.e. the character codes Low and High are
// part of the range.
// Low and High must have the same length and only differ in the last byte.
type Range struct {
	Low, High []byte
}

func (r Range) numCodes() CharCode {
	var numCodes CharCode = 1
	for i, low := range r.Low {
		numCodes *= CharCode(r.High[i]-low) + 1
	}
	return numCodes
}

// Matches returns true, if the PDF string starts with a character code
// in the given range.
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
var Simple = CodeSpaceRange{{[]byte{0x00}, []byte{0xFF}}}

// UCS2 represents a two-byte encoding.
// Character codes are two bytes long, and are stored in big-endian order.
var UCS2 = CodeSpaceRange{{[]byte{0x00, 0x00}, []byte{0xFF, 0xFF}}}
