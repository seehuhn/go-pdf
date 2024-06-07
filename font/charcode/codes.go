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
//
// TODO(voss): remove
type CharCode int

// CodeSpaceRange describes the ranges of byte sequences which are valid
// character codes for a given encoding.
type CodeSpaceRange []Range

// AllCodes returns an iterator over all character codes in the given PDF string.
func (c CodeSpaceRange) AllCodes(s pdf.String) func(yield func(code pdf.String, valid bool) bool) {
	return func(yield func(pdf.String, bool) bool) {
		for len(s) > 0 {
			k, valid := c.firstCode(s)
			if !yield(s[:k], valid) {
				return
			}
			s = s[k:]
		}
		return
	}
}

// firstCode returns the length of the first character code from the given PDF string,
// together with a boolean indicating whether the code was valid.
//
// See the algorithm from section 9.7.6.3 of the PDF-2.0 spec.
func (c CodeSpaceRange) firstCode(s pdf.String) (int, bool) {
	candiates := make([]int, len(c))
	for j := range candiates {
		candiates[j] = j
	}

	if len(s) == 0 {
		return 0, false
	}

	var skipLen int
	for i := 0; i < len(s); i++ {
		skipLen = len(s)

		b := s[i]
		for j := 0; j < len(candiates); {
			r := c[candiates[j]]

			L := len(r.Low)
			if L < skipLen {
				skipLen = L
			}

			if L <= i || b < r.Low[i] || b > r.High[i] {
				candiates[j] = candiates[len(candiates)-1]
				candiates = candiates[:len(candiates)-1]
			} else if L == i+1 {
				return i + 1, true
			} else {
				j++
			}
		}
		if len(candiates) == 0 {
			break
		}
	}
	return skipLen, false
}

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
//
// TODO(voss): Remove?  Otherwise, fix the the number of bytes consumed
// for invalid codes as described in the PDF spec.
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
// Low and High must have the same length.
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
// Character codes are one byte long, and correspond directly to
// the bytes in the PDF string.
var Simple = CodeSpaceRange{{[]byte{0x00}, []byte{0xFF}}}

// UCS2 represents a two-byte encoding.
// Character codes are two bytes long, and are stored in big-endian order.
var UCS2 = CodeSpaceRange{{[]byte{0x00, 0x00}, []byte{0xFF, 0xFF}}}
