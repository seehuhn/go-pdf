// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package jbig2

import (
	"fmt"
	"math"
)

// Arithmetic integer coding procedures (ITU-T T.88, Annex A).
//
// Each procedure maintains 512 contexts and decodes/encodes integer values
// using a prefix code followed by magnitude bits.

// intRange defines one row of the integer coding table (Table A.1).
type intRange struct {
	prefix    int // prefix bit pattern
	prefLen   int // number of prefix bits
	rangeBits int // number of additional magnitude bits
	rangeBase int // base value for this range
	sign      int // 0 = positive, 1 = negative
}

// intCoding table from spec Table A.1.
// Ordered by prefix code, used for both encode and decode.
var intRanges = []intRange{
	// positive ranges
	{prefix: 0b00, prefLen: 2, rangeBits: 2, rangeBase: 0, sign: 0},         // 0..3
	{prefix: 0b010, prefLen: 3, rangeBits: 4, rangeBase: 4, sign: 0},        // 4..19
	{prefix: 0b0110, prefLen: 4, rangeBits: 6, rangeBase: 20, sign: 0},      // 20..83
	{prefix: 0b01110, prefLen: 5, rangeBits: 8, rangeBase: 84, sign: 0},     // 84..339
	{prefix: 0b011110, prefLen: 6, rangeBits: 12, rangeBase: 340, sign: 0},  // 340..4435
	{prefix: 0b011111, prefLen: 6, rangeBits: 32, rangeBase: 4436, sign: 0}, // 4436..

	// negative ranges
	{prefix: 0b1001, prefLen: 4, rangeBits: 0, rangeBase: 1, sign: 1},       // -1
	{prefix: 0b101, prefLen: 3, rangeBits: 1, rangeBase: 2, sign: 1},        // -3..-2
	{prefix: 0b110, prefLen: 3, rangeBits: 4, rangeBase: 4, sign: 1},        // -19..-4
	{prefix: 0b1110, prefLen: 4, rangeBits: 6, rangeBase: 20, sign: 1},      // -83..-20
	{prefix: 0b11110, prefLen: 5, rangeBits: 8, rangeBase: 84, sign: 1},     // -339..-84
	{prefix: 0b111110, prefLen: 6, rangeBits: 12, rangeBase: 340, sign: 1},  // -4435..-340
	{prefix: 0b111111, prefLen: 6, rangeBits: 32, rangeBase: 4436, sign: 1}, // ..-4436

	// out-of-band
	{prefix: 0b1000, prefLen: 4, rangeBits: 0, rangeBase: 0, sign: -1}, // OOB
}

const oobResult int64 = math.MinInt64 // sentinel for out-of-band

// intCtx holds the context state for an integer coding procedure.
type intCtx struct {
	ctx [512]byte
}

// encode encodes an integer value using the given MQ encoder.
// Returns false for out-of-band.
func (ic *intCtx) encode(enc *mqEncoder, v int64) {
	var s int
	var absv int64
	if v < 0 {
		s = 1
		absv = -v
	} else {
		absv = v
	}

	// find the matching range
	var bits []int
	for i := range intRanges {
		r := &intRanges[i]
		if r.sign == -1 {
			continue // skip OOB
		}
		if r.sign == s && absv >= int64(r.rangeBase) && (r.rangeBits == 32 || absv < int64(r.rangeBase)+(1<<r.rangeBits)) {
			// encode prefix bits
			for j := r.prefLen - 1; j >= 0; j-- {
				bits = append(bits, (r.prefix>>j)&1)
			}
			// encode magnitude bits
			mag := absv - int64(r.rangeBase)
			for j := r.rangeBits - 1; j >= 0; j-- {
				bits = append(bits, int(mag>>j)&1)
			}
			break
		}
	}

	ic.encodeBits(enc, bits)
}

// encodeOOB encodes an out-of-band signal.
func (ic *intCtx) encodeOOB(enc *mqEncoder) {
	bits := []int{1, 0, 0, 0}
	ic.encodeBits(enc, bits)
}

func (ic *intCtx) encodeBits(enc *mqEncoder, bits []int) {
	prev := 1
	for _, d := range bits {
		cx := &ic.ctx[prev]
		enc.encode(cx, d)
		if prev < 256 {
			prev = (prev << 1) | d
		} else {
			prev = (((prev << 1) | d) & 511) | 256
		}
	}
}

// decode decodes an integer value using the given MQ decoder.
// Returns oobResult for out-of-band.
func (ic *intCtx) decode(dec *mqDecoder) int64 {
	prev := 1
	decodeBit := func() int {
		cx := &ic.ctx[prev]
		d := dec.decode(cx)
		if prev < 256 {
			prev = (prev << 1) | d
		} else {
			prev = (((prev << 1) | d) & 511) | 256
		}
		return d
	}

	// decode prefix to determine the range
	b0 := decodeBit()
	if b0 == 0 {
		// positive: 00..., 010..., 0110..., etc.
		b1 := decodeBit()
		if b1 == 0 {
			// 00 + 2 bits → 0..3
			return decodeMagnitude(decodeBit, 2, 0)
		}
		b2 := decodeBit()
		if b2 == 0 {
			// 010 + 4 bits → 4..19
			return decodeMagnitude(decodeBit, 4, 4)
		}
		b3 := decodeBit()
		if b3 == 0 {
			// 0110 + 6 bits → 20..83
			return decodeMagnitude(decodeBit, 6, 20)
		}
		b4 := decodeBit()
		if b4 == 0 {
			// 01110 + 8 bits → 84..339
			return decodeMagnitude(decodeBit, 8, 84)
		}
		b5 := decodeBit()
		if b5 == 0 {
			// 011110 + 12 bits → 340..4435
			return decodeMagnitude(decodeBit, 12, 340)
		}
		// 011111 + 32 bits → 4436..
		return decodeMagnitude(decodeBit, 32, 4436)
	}

	// negative or OOB: 1...
	b1 := decodeBit()
	if b1 == 0 {
		b2 := decodeBit()
		if b2 == 0 {
			b3 := decodeBit()
			if b3 == 0 {
				// 1000 → OOB
				return oobResult
			}
			// 1001 → -1
			return -1
		}
		// 101 + 1 bit → -3..-2
		return -decodeMagnitude(decodeBit, 1, 2)
	}
	// 11...
	b2 := decodeBit()
	if b2 == 0 {
		// 110 + 4 bits → -19..-4
		return -decodeMagnitude(decodeBit, 4, 4)
	}
	b3 := decodeBit()
	if b3 == 0 {
		// 1110 + 6 bits → -83..-20
		return -decodeMagnitude(decodeBit, 6, 20)
	}
	b4 := decodeBit()
	if b4 == 0 {
		// 11110 + 8 bits → -339..-84
		return -decodeMagnitude(decodeBit, 8, 84)
	}
	b5 := decodeBit()
	if b5 == 0 {
		// 111110 + 12 bits → -4435..-340
		return -decodeMagnitude(decodeBit, 12, 340)
	}
	// 111111 + 32 bits → ..-4436
	return -decodeMagnitude(decodeBit, 32, 4436)
}

func decodeMagnitude(decodeBit func() int, nBits, base int) int64 {
	var v int64
	for range nBits {
		v = (v << 1) | int64(decodeBit())
	}
	return v + int64(base)
}

// iaidCtx holds the context state for the IAID procedure.
type iaidCtx struct {
	ctx []byte // 2^codeLen contexts
}

func newIAIDCtx(codeLen int) (*iaidCtx, error) {
	if codeLen > maxIAIDCodeLen {
		return nil, fmt.Errorf("IAID code length %d exceeds maximum %d",
			codeLen, maxIAIDCodeLen)
	}
	return &iaidCtx{ctx: make([]byte, 1<<codeLen)}, nil
}

// encode encodes a symbol ID using fixed-length IAID coding.
func (ic *iaidCtx) encode(enc *mqEncoder, codeLen int, id int) {
	prev := 1
	for i := codeLen - 1; i >= 0; i-- {
		d := (id >> i) & 1
		cx := &ic.ctx[prev]
		enc.encode(cx, d)
		prev = (prev << 1) | d
	}
}

// decode decodes a symbol ID using fixed-length IAID coding.
func (ic *iaidCtx) decode(dec *mqDecoder, codeLen int) int {
	prev := 1
	for range codeLen {
		cx := &ic.ctx[prev]
		d := dec.decode(cx)
		prev = (prev << 1) | d
	}
	return prev - (1 << codeLen)
}
