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
	"fmt"
	"unicode"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
)

// ToUnicodeOld holds the information for a PDF ToUnicodeOld cmap.
type ToUnicodeOld struct {
	CS      charcode.CodeSpaceRange
	Singles []ToUnicodeSingleOld
	Ranges  []ToUnicodeRangeOld
}

// ToUnicodeSingleOld specifies that character code Code represents the given unicode string.
type ToUnicodeSingleOld struct {
	Code  charcode.CharCodeOld
	Value []rune
}

func (s ToUnicodeSingleOld) String() string {
	return fmt.Sprintf("%d: %q", s.Code, string(s.Value))
}

// ToUnicodeRangeOld describes a range of character codes.
// First and Last are the first and last code points in the range.
// Values is a list of unicode strings.  If the list has length one, then the
// replacement character is incremented by one for each code point in the
// range.  Otherwise, the list must have the length Last-First+1, and specify
// the value for each code point in the range.
type ToUnicodeRangeOld struct {
	First  charcode.CharCodeOld
	Last   charcode.CharCodeOld
	Values [][]rune
}

func (r ToUnicodeRangeOld) String() string {
	ss := make([]string, len(r.Values))
	for i, v := range r.Values {
		ss[i] = string(v)
	}
	return fmt.Sprintf("%d-%d: %q", r.First, r.Last, ss)
}

// NewToUnicode constructs a ToUnicode cmap from the given mapping.
func NewToUnicode(cs charcode.CodeSpaceRange, m map[charcode.CharCodeOld][]rune) *ToUnicodeOld {
	info := &ToUnicodeOld{
		CS: cs,
	}
	info.SetMapping(m)
	return info
}

// NewToUnicode2 constructs a ToUnicode cmap from the given mapping.
func NewToUnicode2(cs charcode.CodeSpaceRange, m map[string][]rune) *ToUnicodeOld {
	m2 := make(map[charcode.CharCodeOld][]rune, len(m))
	for cStr, v := range m {
		code, k := cs.Decode(pdf.String(cStr))
		if code < 0 || k != len(cStr) {
			panic(fmt.Sprintf("invalid code <%x>", []byte(cStr)))
		}
		m2[code] = v
	}

	info := &ToUnicodeOld{
		CS: cs,
	}
	info.SetMapping(m2)
	return info
}

// Decode decodes the first character code from the given string.
// It returns the corresponding unicode rune and the number of bytes consumed.
// If the character code cannot be decoded, [unicode.ReplacementChar] is returned,
// and the length is either 0 (if the string is empty) or 1.
// If a valid character code is found but the code is not mapped by the
// ToUnicode cmap, then the unicode replacement character is returned.
func (info *ToUnicodeOld) Decode(s pdf.String) ([]rune, int) {
	code, k := info.CS.Decode(s)
	if code < 0 {
		return []rune{unicode.ReplacementChar}, k
	}

	for _, r := range info.Ranges {
		if code < r.First || code > r.Last {
			continue
		}
		if len(r.Values) > int(code-r.First) {
			return r.Values[code-r.First], k
		}
		if len(r.Values[0]) == 0 {
			return []rune{}, k
		}
		rr := make([]rune, len(r.Values[0]))
		copy(rr, r.Values[0])
		rr[len(rr)-1] += rune(code - r.First)
		return rr, k
	}

	for _, s := range info.Singles {
		if s.Code == code {
			return s.Value, k
		}
	}

	return []rune{unicode.ReplacementChar}, k
}
