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

package ccittfax

import "sort"

//go:generate go run ./generate/

const maxColumns = 1 << 20

// Params holds the parameters that control CCITT Fax encoding and decoding behavior.
type Params struct {
	// Columns specifies image width in pixels.
	// The default value of 0 is interpreted as 1728 pixels.
	Columns int

	// K determines the algorithm variant:
	//   K < 0: Group 4 (pure 2D encoding)
	//   K = 0: Group 3 one-dimensional
	//   K > 0: Group 3 two-dimensional (each 1D line is followed by K-1 2D lines)
	K int

	// MaxRows specifies the maximum number of rows to encode/decode
	// (0 = use all rows).
	MaxRows int

	// EndOfLine indicates whether EOL codes are present in the stream
	EndOfLine bool

	// EncodedByteAlign indicates whether each scan line is padded to byte boundary
	EncodedByteAlign bool

	// BlackIs1 controls the interpretation of bit values.
	// If this is true, bit values are flipped before encoding and after decoding.
	BlackIs1 bool

	// IgnoreEndOfBlock indicates whether to ignore EOFB/RTC termination patterns
	// false: respect end-of-block patterns (PDF default)
	// true:  ignore termination patterns, decode entire stream
	IgnoreEndOfBlock bool

	// DamagedRowsBeforeError is the number of damaged rows of data that shall
	// be tolerated before an error occurs.
	DamagedRowsBeforeError int
}

func (p Params) whiteBit() byte {
	if p.BlackIs1 {
		return 0
	}
	return 1
}

// getPixel returns the bit value at column x.
// pixels outside the image are white.
func (p Params) getPixel(lineData []byte, x int) byte {
	byteIndex := x / 8
	if x < 0 || x >= p.Columns || byteIndex >= len(lineData) {
		if p.BlackIs1 {
			return 0
		}
		return 1
	}

	bitIndex := 7 - (x % 8)
	return (lineData[byteIndex] >> uint(bitIndex)) & 1
}

// endOfRun finds the x-coordinate of the first pixel in lineData
// at or after startX whose value is different from refValue.
func (p Params) endOfRun(lineData []byte, startX int, runBit byte) int {
	for x := startX; x < p.Columns; x++ {
		if p.getPixel(lineData, x) != runBit {
			return x
		}
	}
	return p.Columns
}

// findB1B2 locates the following two bit column indices in lineData:
// B1 is the first changing bit to the right of a0 and of color 1-currentBit.
// B2 is the next changing bit to the right of B1 (of color currentBit).
//
// The parameters currentBit must be either 0 or 1.
// The parameter a0 must be >=-1, and if a0 is -1, currentBit must correspond to white.
func (p Params) findB1B2(lineData []byte, a0 int, currentBit byte) (int, int) {
	b0 := a0
	for b0 < p.Columns && p.getPixel(lineData, b0) != currentBit {
		b0++
	}
	b1 := min(b0+1, p.Columns)
	for b1 < p.Columns && p.getPixel(lineData, b1) != 1-currentBit {
		b1++
	}
	b2 := min(b1+1, p.Columns)
	for b2 < p.Columns && p.getPixel(lineData, b2) != currentBit {
		b2++
	}
	return b1, b2
}

// changingElements appends to dst the columns where lineData's colour
// changes, treating the imaginary pixel before column 0 as white.  The
// result is strictly increasing and lies in [0, Columns).  It is built
// once per row so that [findB1B2FromChanges] can resolve the reference
// line's changing elements in O(log n) instead of scanning pixel by
// pixel, which is O(changes × Columns) for a row with many transitions.
func (p Params) changingElements(lineData []byte, dst []int) []int {
	prev := p.whiteBit()
	for x := range p.Columns {
		c := p.getPixel(lineData, x)
		if c != prev {
			dst = append(dst, x)
			prev = c
		}
	}
	return dst
}

// nextTwoChanges returns the smallest element of changes greater than a0
// and the one after it, each capped at columns.  In the 2D encoder the
// run containing a0 on the coding line always has colour currentBit, so
// these are the encoder's a1 and a2 (the ends of the next two runs).
func nextTwoChanges(changes []int, columns, a0 int) (int, int) {
	idx := sort.Search(len(changes), func(i int) bool { return changes[i] > a0 })
	a1, a2 := columns, columns
	if idx < len(changes) {
		a1 = changes[idx]
	}
	if idx+1 < len(changes) {
		a2 = changes[idx+1]
	}
	return a1, a2
}

// findB1B2FromChanges returns the same (b1, b2) as [Params.findB1B2] but
// uses the precomputed changing elements of the reference line.  changes
// must be the result of [Params.changingElements] for that line.
//
// The colour of the run starting at changes[i] is 1-whiteBit for even i
// and whiteBit for odd i (the line starts white before column 0), so b1
// is the first changing element after a0 whose colour is 1-currentBit and
// b2 is the changing element that follows it.
func findB1B2FromChanges(changes []int, columns, a0 int, currentBit, whiteBit byte) (int, int) {
	idx := sort.Search(len(changes), func(i int) bool { return changes[i] > a0 })
	colourEven := 1 - whiteBit // colour at an even-indexed changing element
	if idx < len(changes) && (idx%2 == 0) != (colourEven == 1-currentBit) {
		// changes[idx] has colour currentBit; the next one is 1-currentBit
		idx++
	}
	b1, b2 := columns, columns
	if idx < len(changes) {
		b1 = changes[idx]
	}
	if idx+1 < len(changes) {
		b2 = changes[idx+1]
	}
	return b1, b2
}
