// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package image

// PixelRow is a helper for efficiently packing pixel data into bytes
// for PDF image streams. It handles arbitrary bits per pixel and
// packs them into a byte array.
type PixelRow struct {
	bytes   []byte
	byteIdx int
	bitPos  int
	numBits int // bits per append operation
}

// NewPixelRow creates a new PixelRow for packing image data.
// numElems is the number of elements (pixels * channels) in the row.
// bitsPerElem is the number of bits per element (1, 2, 4, 8, or 16).
func NewPixelRow(numElems, bitsPerElem int) *PixelRow {
	rowBytes := (numElems*bitsPerElem + 7) >> 3
	return &PixelRow{
		bytes:   make([]byte, rowBytes),
		numBits: bitsPerElem,
	}
}

// Reset clears the row buffer and resets position counters.
func (r *PixelRow) Reset() {
	r.byteIdx = 0
	r.bitPos = 0
	clear(r.bytes)
}

// Bytes returns the underlying byte slice containing the packed pixel data.
func (r *PixelRow) Bytes() []byte {
	return r.bytes
}

// AppendBits appends the specified number of bits to the row.
// Only the low-order bitsPerElem bits of the value are used.
func (r *PixelRow) AppendBits(bits uint16) {
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
