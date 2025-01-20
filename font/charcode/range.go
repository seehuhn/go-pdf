// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"iter"
	"slices"

	"golang.org/x/exp/maps"
)

// Range represents a range of character codes.
// To be within the range, a byte sequence must have the same length as Low
// and High, and every byte in the sequence must be between the corresponding
// bytes in Low and High (inclusive).
//
// Low and High must have the same length and must not be empty.
type Range struct {
	Low, High []byte
}

// CodeSpaceRange describes the ranges of byte sequences which are valid
// character codes for a given encoding.
type CodeSpaceRange []Range

var (
	// Simple represents the code space range for a simple font.
	// Character codes are one byte long, and correspond directly to
	// the bytes in the PDF string.
	Simple = CodeSpaceRange{{[]byte{0x00}, []byte{0xFF}}}

	// UCS2 represents a two-byte encoding.
	// Character codes are two bytes long, and are stored in big-endian order.
	UCS2 = CodeSpaceRange{{[]byte{0x00, 0x00}, []byte{0xFF, 0xFF}}}
)

// matchLen returns the number number of leading bytes in s which can be
// matched by csr.  If s does not start with a valid code, the return value
// is 0.
func (csr CodeSpaceRange) matchLen(s []byte) int {
	for _, r := range csr {
		if len(s) < len(r.Low) {
			continue
		}

		valid := true
		for i := 0; i < len(r.Low); i++ {
			if s[i] < r.Low[i] || s[i] > r.High[i] {
				valid = false
				break
			}
		}

		if valid {
			return len(r.Low)
		}
	}

	return 0
}

// isEquivalent returns true if and only if the two code space ranges contain
// the same set of character codes.
func (csr CodeSpaceRange) isEquivalent(other CodeSpaceRange) bool {
	for code := range testSequences(csr, other).All() {
		if csr.matchLen(code) != other.matchLen(code) {
			return false
		}
	}
	return true
}

type seqGen [][]byte

func testSequences(ranges ...CodeSpaceRange) seqGen {
	res := make(seqGen, 4)
	for pos := range res {
		allBreaks := make(map[int]bool)
		allBreaks[0] = true
		allBreaks[256] = true

		for _, csr := range ranges {
			for _, r := range csr {
				if pos < len(r.Low) {
					allBreaks[int(r.Low[pos])] = true
					allBreaks[int(r.High[pos])+1] = true
				}
			}
		}

		breaks := maps.Keys(allBreaks)
		slices.Sort(breaks)

		// All values between two breaks are equivalent, because in the absence
		// of breaks the code space ranges have no way to distinguish between
		// them.  We simply pick the first value in each range for the test
		// sequences.
		testValues := make([]byte, len(breaks)-1)
		for i := range testValues {
			testValues[i] = byte(breaks[i])
		}

		res[pos] = testValues
	}

	return res
}

func (t seqGen) All() iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		var idx [4]int
		var seq [4]byte

	yieldLoop:
		for {
			for i := range seq {
				seq[i] = t[i][idx[i]]
			}

			if !yield(seq[:]) {
				return
			}

			for i := 3; i >= 0; i-- {
				idx[i]++
				if idx[i] < len(t[i]) {
					continue yieldLoop
				}
				idx[i] = 0
			}
			return // All combinations done
		}
	}
}
