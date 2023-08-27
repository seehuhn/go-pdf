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

package tounicode

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"unicode"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/type1"
)

// Info holds the information from a ToUnicode cmap.
type Info struct {
	Name    pdf.Name
	ROS     *type1.CIDSystemInfo
	CS      charcode.CodeSpaceRange
	Singles []Single
	Ranges  []Range
}

// Decode decodes the first character code from the given string.
// It returns the corresponding unicode rune and the number of bytes consumed.
// If the character code cannot be decoded, [unicode.ReplacementChar] is returned,
// and the length is either 0 (if the string is empty) or 1.
// If a valid character code is found but the code is not mapped by the
// ToUnicode cmap, then the unicode replacement character is returned.
func (info *Info) Decode(s pdf.String) ([]rune, int) {
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

// Single specifies that character code Code represents the given unicode string.
type Single struct {
	Code  charcode.CharCode
	Value []rune
}

func (s Single) String() string {
	return fmt.Sprintf("%d: %q", s.Code, string(s.Value))
}

// Range describes a range of character codes.
// First and Last are the first and last code points in the range.
// Values is a list of unicode strings.  If the list has length one, then the
// replacement character is incremented by one for each code point in the
// range.  Otherwise, the list must have the length Last-First+1, and specify
// the value for each code point in the range.
type Range struct {
	First  charcode.CharCode
	Last   charcode.CharCode
	Values [][]rune
}

func (r Range) String() string {
	ss := make([]string, len(r.Values))
	for i, v := range r.Values {
		ss[i] = string(v)
	}
	return fmt.Sprintf("%d-%d: %q", r.First, r.Last, ss)
}

// MakeName sets a unique name for the ToUnicode cmap.
func (info *Info) makeName() {
	var buf [binary.MaxVarintLen64]byte

	h := sha256.New()
	k := binary.PutVarint(buf[:], int64(len(info.ROS.Registry)))
	h.Write(buf[:k])
	h.Write([]byte(info.ROS.Registry))
	k = binary.PutVarint(buf[:], int64(len(info.ROS.Ordering)))
	h.Write(buf[:k])
	h.Write([]byte(info.ROS.Ordering))
	k = binary.PutVarint(buf[:], int64(info.ROS.Supplement))
	h.Write(buf[:k])

	rr := info.CS.Ranges()
	k = binary.PutVarint(buf[:], int64(len(rr)))
	h.Write(buf[:k])
	for _, r := range rr {
		k = binary.PutVarint(buf[:], int64(len(r.Low)))
		h.Write(buf[:k])
		h.Write(r.Low)
		k = binary.PutVarint(buf[:], int64(len(r.High)))
		h.Write(buf[:k])
		h.Write(r.High)
	}

	k = binary.PutVarint(buf[:], int64(len(info.Singles)))
	h.Write(buf[:k])
	for _, s := range info.Singles {
		binary.Write(h, binary.LittleEndian, s.Code)
		k = binary.PutVarint(buf[:], int64(len(s.Value)))
		h.Write(buf[:k])
		binary.Write(h, binary.LittleEndian, s.Value)
	}

	k = binary.PutVarint(buf[:], int64(len(info.Ranges)))
	h.Write(buf[:k])
	for _, r := range info.Ranges {
		binary.Write(h, binary.LittleEndian, r.First)
		binary.Write(h, binary.LittleEndian, r.Last)
		k = binary.PutVarint(buf[:], int64(len(r.Values)))
		h.Write(buf[:k])
		for _, v := range r.Values {
			k = binary.PutVarint(buf[:], int64(len(v)))
			h.Write(buf[:k])
			binary.Write(h, binary.LittleEndian, v)
		}
	}

	sum := h.Sum(nil)
	info.Name = pdf.Name(fmt.Sprintf("Seehuhn-%x", sum[:8]))
}
