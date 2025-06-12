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

package image

type pixRow struct {
	bytes   []byte
	byteIdx int
	bitPos  int
	numBits int // bits per append operation
}

func newPixRow(numElems, bitsPerElem int) *pixRow {
	rowBytes := (numElems*bitsPerElem + 7) >> 3
	return &pixRow{
		bytes:   make([]byte, rowBytes),
		numBits: bitsPerElem,
	}
}

func (r *pixRow) reset() {
	r.byteIdx = 0
	r.bitPos = 0
	clear(r.bytes)
}

func (r *pixRow) appendBits(bits uint16) {
	bitsToDo := r.numBits

	// fast path for 8 bit writes
	if bitsToDo == 8 {
		r.bytes[r.byteIdx] = byte(bits)
		r.byteIdx++
		return
	}

	// general case
	for bitsToDo > 0 {
		availableBits := 8 - r.bitPos
		k := min(bitsToDo, availableBits)

		shift := bitsToDo - k
		bitsToWrite := byte((bits >> shift) & ((1 << k) - 1))

		r.bytes[r.byteIdx] |= bitsToWrite << (availableBits - k)

		r.bitPos += k
		bitsToDo -= k
		if r.bitPos == 8 {
			r.byteIdx++
			r.bitPos = 0
		}
	}
}
