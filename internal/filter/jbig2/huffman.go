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
	"io"
)

// Huffman coding for JBIG2 (ITU-T T.88, Annex B).

// huffLine represents one line of a Huffman code table.
type huffLine struct {
	RangeLow int32 // lowest value in range
	PrefLen  int   // prefix code length
	RangeLen int   // number of additional bits
	IsLower  bool  // lower range line (value = RangeLow - offset)
	IsOOB    bool  // out-of-band line
}

// huffTable is a Huffman code table.
type huffTable struct {
	Lines []huffLine
	codes []uint32 // assigned prefix codes
}

// newHuffTable creates a Huffman table and assigns canonical prefix codes.
func newHuffTable(lines []huffLine) *huffTable {
	t := &huffTable{Lines: lines}
	t.assignCodes()
	return t
}

func (t *huffTable) assignCodes() {
	n := len(t.Lines)
	t.codes = make([]uint32, n)

	// find max prefix length
	maxLen := 0
	for _, l := range t.Lines {
		if l.PrefLen > maxLen {
			maxLen = l.PrefLen
		}
	}
	if maxLen == 0 {
		return
	}

	// count occurrences of each prefix length
	lenCount := make([]int, maxLen+1)
	for _, l := range t.Lines {
		lenCount[l.PrefLen]++
	}

	// compute first code for each length
	firstCode := make([]uint32, maxLen+1)
	lenCount[0] = 0
	for i := 1; i <= maxLen; i++ {
		firstCode[i] = (firstCode[i-1] + uint32(lenCount[i-1])) << 1
	}

	// assign codes
	curCode := make([]uint32, maxLen+1)
	copy(curCode, firstCode)
	for i, l := range t.Lines {
		if l.PrefLen > 0 {
			t.codes[i] = curCode[l.PrefLen]
			curCode[l.PrefLen]++
		}
	}
}

// huffReader reads bits from a byte slice for Huffman decoding.
type huffReader struct {
	data    []byte
	bytePos int
	bitPos  int // 0-7, counts from MSB (7 = MSB, 0 = LSB)
	eof     bool
	err     error // set on corrupt data; short-circuits subsequent reads
}

func newHuffReader(data []byte) *huffReader {
	return &huffReader{data: data, bitPos: 7}
}

func (r *huffReader) readBit() int {
	if r.err != nil {
		return 0
	}
	if r.bytePos >= len(r.data) {
		r.eof = true
		return 0
	}
	bit := (r.data[r.bytePos] >> r.bitPos) & 1
	r.bitPos--
	if r.bitPos < 0 {
		r.bitPos = 7
		r.bytePos++
	}
	return int(bit)
}

func (r *huffReader) readBits(n int) uint32 {
	var v uint32
	for range n {
		v = (v << 1) | uint32(r.readBit())
	}
	return v
}

// align skips to the next byte boundary.
func (r *huffReader) align() {
	if r.bitPos != 7 {
		r.bitPos = 7
		r.bytePos++
	}
}

// offset returns the current byte offset.
func (r *huffReader) offset() int {
	if r.bitPos == 7 {
		return r.bytePos
	}
	return r.bytePos + 1
}

// bitWriter writes bits MSB-first into a dynamically growing byte slice.
type bitWriter struct {
	buf    []byte
	bitPos int // number of bits written into current byte (0..7)
}

func newBitWriter() *bitWriter {
	return &bitWriter{}
}

func (w *bitWriter) writeBit(bit int) {
	if w.bitPos == 0 {
		w.buf = append(w.buf, 0)
	}
	w.buf[len(w.buf)-1] |= byte(bit&1) << (7 - w.bitPos)
	w.bitPos++
	if w.bitPos >= 8 {
		w.bitPos = 0
	}
}

func (w *bitWriter) writeBits(val uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		w.writeBit(int((val >> i) & 1))
	}
}

// writeBytes appends raw bytes. The writer must be byte-aligned.
func (w *bitWriter) writeBytes(data []byte) {
	w.buf = append(w.buf, data...)
}

// align advances to the next byte boundary. If already aligned, does nothing.
func (w *bitWriter) align() {
	if w.bitPos != 0 {
		w.bitPos = 0
	}
}

// bytes returns all written bytes.
func (w *bitWriter) bytes() []byte {
	return w.buf
}

// decode reads one value from the Huffman table.
func (t *huffTable) decode(r *huffReader) int64 {
	var code uint32
	codeLen := 0
	for {
		code = (code << 1) | uint32(r.readBit())
		codeLen++
		if r.eof {
			r.err = io.ErrUnexpectedEOF
			return 0
		}
		for i, l := range t.Lines {
			if l.PrefLen == codeLen && t.codes[i] == code {
				if l.IsOOB {
					return oobResult
				}
				if l.RangeLen == 0 {
					return int64(l.RangeLow)
				}
				offset := int64(r.readBits(l.RangeLen))
				if r.eof {
					r.err = io.ErrUnexpectedEOF
					return 0
				}
				if l.IsLower {
					return int64(l.RangeLow) - offset
				}
				return int64(l.RangeLow) + offset
			}
		}
		if codeLen > 32 {
			r.err = fmt.Errorf("invalid Huffman code")
			return 0
		}
	}
}

// encode writes the Huffman code for value v into the bitWriter.
func (t *huffTable) encode(w *bitWriter, v int64) error {
	for i, l := range t.Lines {
		if l.IsOOB || l.PrefLen == 0 {
			continue
		}
		if l.IsLower {
			maxOffset := int64(1)<<l.RangeLen - 1
			if l.RangeLen == 32 {
				maxOffset = 1<<32 - 1
			}
			low := int64(l.RangeLow) - maxOffset
			if v >= low && v <= int64(l.RangeLow) {
				w.writeBits(t.codes[i], l.PrefLen)
				w.writeBits(uint32(int64(l.RangeLow)-v), l.RangeLen)
				return nil
			}
		} else {
			maxVal := int64(l.RangeLow) + int64(1)<<l.RangeLen - 1
			if l.RangeLen == 32 {
				maxVal = int64(l.RangeLow) + 1<<32 - 1
			}
			if v >= int64(l.RangeLow) && v <= maxVal {
				w.writeBits(t.codes[i], l.PrefLen)
				w.writeBits(uint32(v-int64(l.RangeLow)), l.RangeLen)
				return nil
			}
		}
	}
	return fmt.Errorf("huffman encode: value %d does not match any table line", v)
}

// encodeOOB writes the out-of-band code.
func (t *huffTable) encodeOOB(w *bitWriter) error {
	for i, l := range t.Lines {
		if l.IsOOB {
			w.writeBits(t.codes[i], l.PrefLen)
			return nil
		}
	}
	return fmt.Errorf("huffman encodeOOB: table has no OOB line")
}

// standard Huffman tables (Tables B.1 through B.15)

var huffTableB1 = newHuffTable([]huffLine{
	{RangeLow: 0, PrefLen: 1, RangeLen: 4},
	{RangeLow: 16, PrefLen: 2, RangeLen: 8},
	{RangeLow: 272, PrefLen: 3, RangeLen: 16},
	{RangeLow: 65808, PrefLen: 3, RangeLen: 32},
})

var huffTableB2 = newHuffTable([]huffLine{
	{RangeLow: 0, PrefLen: 1, RangeLen: 0},
	{RangeLow: 1, PrefLen: 2, RangeLen: 0},
	{RangeLow: 2, PrefLen: 3, RangeLen: 0},
	{RangeLow: 3, PrefLen: 4, RangeLen: 3},
	{RangeLow: 11, PrefLen: 5, RangeLen: 6},
	{RangeLow: 75, PrefLen: 6, RangeLen: 32},
	{IsOOB: true, PrefLen: 6},
})

var huffTableB3 = newHuffTable([]huffLine{
	{RangeLow: -256, PrefLen: 8, RangeLen: 8, IsLower: false},
	{RangeLow: 0, PrefLen: 1, RangeLen: 0},
	{RangeLow: 1, PrefLen: 2, RangeLen: 0},
	{RangeLow: 2, PrefLen: 3, RangeLen: 0},
	{RangeLow: 3, PrefLen: 4, RangeLen: 3},
	{RangeLow: 11, PrefLen: 5, RangeLen: 6},
	{RangeLow: -257, PrefLen: 8, RangeLen: 32, IsLower: true},
	{RangeLow: 75, PrefLen: 7, RangeLen: 32},
	{IsOOB: true, PrefLen: 6},
})

var huffTableB4 = newHuffTable([]huffLine{
	{RangeLow: 1, PrefLen: 1, RangeLen: 0},
	{RangeLow: 2, PrefLen: 2, RangeLen: 0},
	{RangeLow: 3, PrefLen: 3, RangeLen: 0},
	{RangeLow: 4, PrefLen: 4, RangeLen: 3},
	{RangeLow: 12, PrefLen: 5, RangeLen: 6},
	{RangeLow: 76, PrefLen: 5, RangeLen: 32},
})

var huffTableB5 = newHuffTable([]huffLine{
	{RangeLow: -255, PrefLen: 7, RangeLen: 8},
	{RangeLow: 1, PrefLen: 1, RangeLen: 0},
	{RangeLow: 2, PrefLen: 2, RangeLen: 0},
	{RangeLow: 3, PrefLen: 3, RangeLen: 0},
	{RangeLow: 4, PrefLen: 4, RangeLen: 3},
	{RangeLow: 12, PrefLen: 5, RangeLen: 6},
	{RangeLow: -256, PrefLen: 7, RangeLen: 32, IsLower: true},
	{RangeLow: 76, PrefLen: 6, RangeLen: 32},
})

var huffTableB6 = newHuffTable([]huffLine{
	{RangeLow: -2048, PrefLen: 5, RangeLen: 10},
	{RangeLow: -1024, PrefLen: 4, RangeLen: 9},
	{RangeLow: -512, PrefLen: 4, RangeLen: 8},
	{RangeLow: -256, PrefLen: 4, RangeLen: 7},
	{RangeLow: -128, PrefLen: 5, RangeLen: 6},
	{RangeLow: -64, PrefLen: 5, RangeLen: 5},
	{RangeLow: -32, PrefLen: 4, RangeLen: 5},
	{RangeLow: 0, PrefLen: 2, RangeLen: 7},
	{RangeLow: 128, PrefLen: 3, RangeLen: 7},
	{RangeLow: 256, PrefLen: 3, RangeLen: 8},
	{RangeLow: 512, PrefLen: 4, RangeLen: 9},
	{RangeLow: 1024, PrefLen: 4, RangeLen: 10},
	{RangeLow: -2049, PrefLen: 6, RangeLen: 32, IsLower: true},
	{RangeLow: 2048, PrefLen: 6, RangeLen: 32},
})

var huffTableB7 = newHuffTable([]huffLine{
	{RangeLow: -1024, PrefLen: 4, RangeLen: 9},
	{RangeLow: -512, PrefLen: 3, RangeLen: 8},
	{RangeLow: -256, PrefLen: 4, RangeLen: 7},
	{RangeLow: -128, PrefLen: 5, RangeLen: 6},
	{RangeLow: -64, PrefLen: 5, RangeLen: 5},
	{RangeLow: -32, PrefLen: 4, RangeLen: 5},
	{RangeLow: 0, PrefLen: 4, RangeLen: 5},
	{RangeLow: 32, PrefLen: 5, RangeLen: 5},
	{RangeLow: 64, PrefLen: 5, RangeLen: 6},
	{RangeLow: 128, PrefLen: 4, RangeLen: 7},
	{RangeLow: 256, PrefLen: 3, RangeLen: 8},
	{RangeLow: 512, PrefLen: 3, RangeLen: 9},
	{RangeLow: 1024, PrefLen: 3, RangeLen: 10},
	{RangeLow: -1025, PrefLen: 5, RangeLen: 32, IsLower: true},
	{RangeLow: 2048, PrefLen: 5, RangeLen: 32},
})

// table B.8 (Table H): DS with OOB
var huffTableB8 = newHuffTable([]huffLine{
	{RangeLow: -15, PrefLen: 8, RangeLen: 3},
	{RangeLow: -7, PrefLen: 9, RangeLen: 1},
	{RangeLow: -5, PrefLen: 8, RangeLen: 1},
	{RangeLow: -3, PrefLen: 9, RangeLen: 0},
	{RangeLow: -2, PrefLen: 7, RangeLen: 0},
	{RangeLow: -1, PrefLen: 4, RangeLen: 0},
	{RangeLow: 0, PrefLen: 2, RangeLen: 1},
	{RangeLow: 2, PrefLen: 5, RangeLen: 0},
	{RangeLow: 3, PrefLen: 6, RangeLen: 0},
	{RangeLow: 4, PrefLen: 3, RangeLen: 4},
	{RangeLow: 20, PrefLen: 6, RangeLen: 1},
	{RangeLow: 22, PrefLen: 4, RangeLen: 4},
	{RangeLow: 38, PrefLen: 4, RangeLen: 5},
	{RangeLow: 70, PrefLen: 5, RangeLen: 6},
	{RangeLow: 134, PrefLen: 5, RangeLen: 7},
	{RangeLow: 262, PrefLen: 6, RangeLen: 7},
	{RangeLow: 390, PrefLen: 7, RangeLen: 8},
	{RangeLow: 646, PrefLen: 6, RangeLen: 10},
	{RangeLow: -16, PrefLen: 9, RangeLen: 32, IsLower: true},
	{RangeLow: 1670, PrefLen: 9, RangeLen: 32},
	{IsOOB: true, PrefLen: 2},
})

// table B.9 (Table I): DS with OOB
var huffTableB9 = newHuffTable([]huffLine{
	{RangeLow: -31, PrefLen: 8, RangeLen: 4},
	{RangeLow: -15, PrefLen: 9, RangeLen: 2},
	{RangeLow: -11, PrefLen: 8, RangeLen: 2},
	{RangeLow: -7, PrefLen: 9, RangeLen: 1},
	{RangeLow: -5, PrefLen: 7, RangeLen: 1},
	{RangeLow: -3, PrefLen: 4, RangeLen: 1},
	{RangeLow: -1, PrefLen: 3, RangeLen: 1},
	{RangeLow: 1, PrefLen: 3, RangeLen: 1},
	{RangeLow: 3, PrefLen: 5, RangeLen: 1},
	{RangeLow: 5, PrefLen: 6, RangeLen: 1},
	{RangeLow: 7, PrefLen: 3, RangeLen: 5},
	{RangeLow: 39, PrefLen: 6, RangeLen: 2},
	{RangeLow: 43, PrefLen: 4, RangeLen: 5},
	{RangeLow: 75, PrefLen: 4, RangeLen: 6},
	{RangeLow: 139, PrefLen: 5, RangeLen: 7},
	{RangeLow: 267, PrefLen: 5, RangeLen: 8},
	{RangeLow: 523, PrefLen: 6, RangeLen: 8},
	{RangeLow: 779, PrefLen: 7, RangeLen: 9},
	{RangeLow: 1291, PrefLen: 6, RangeLen: 11},
	{RangeLow: -32, PrefLen: 9, RangeLen: 32, IsLower: true},
	{RangeLow: 3339, PrefLen: 9, RangeLen: 32},
	{IsOOB: true, PrefLen: 2},
})

// table B.10 (Table J): DS with OOB
var huffTableB10 = newHuffTable([]huffLine{
	{RangeLow: -21, PrefLen: 7, RangeLen: 4},
	{RangeLow: -5, PrefLen: 8, RangeLen: 0},
	{RangeLow: -4, PrefLen: 7, RangeLen: 0},
	{RangeLow: -3, PrefLen: 5, RangeLen: 0},
	{RangeLow: -2, PrefLen: 2, RangeLen: 2},
	{RangeLow: 2, PrefLen: 5, RangeLen: 0},
	{RangeLow: 3, PrefLen: 6, RangeLen: 0},
	{RangeLow: 4, PrefLen: 7, RangeLen: 0},
	{RangeLow: 5, PrefLen: 8, RangeLen: 0},
	{RangeLow: 6, PrefLen: 2, RangeLen: 6},
	{RangeLow: 70, PrefLen: 5, RangeLen: 5},
	{RangeLow: 102, PrefLen: 6, RangeLen: 5},
	{RangeLow: 134, PrefLen: 6, RangeLen: 6},
	{RangeLow: 198, PrefLen: 6, RangeLen: 7},
	{RangeLow: 326, PrefLen: 6, RangeLen: 8},
	{RangeLow: 582, PrefLen: 6, RangeLen: 9},
	{RangeLow: 1094, PrefLen: 6, RangeLen: 10},
	{RangeLow: 2118, PrefLen: 7, RangeLen: 11},
	{RangeLow: -22, PrefLen: 8, RangeLen: 32, IsLower: true},
	{RangeLow: 4166, PrefLen: 8, RangeLen: 32},
	{IsOOB: true, PrefLen: 2},
})

var huffTableB11 = newHuffTable([]huffLine{
	{RangeLow: 1, PrefLen: 1, RangeLen: 0},
	{RangeLow: 2, PrefLen: 2, RangeLen: 1},
	{RangeLow: 4, PrefLen: 4, RangeLen: 0},
	{RangeLow: 5, PrefLen: 4, RangeLen: 1},
	{RangeLow: 7, PrefLen: 5, RangeLen: 1},
	{RangeLow: 9, PrefLen: 5, RangeLen: 2},
	{RangeLow: 13, PrefLen: 6, RangeLen: 2},
	{RangeLow: 17, PrefLen: 7, RangeLen: 2},
	{RangeLow: 21, PrefLen: 7, RangeLen: 3},
	{RangeLow: 29, PrefLen: 7, RangeLen: 4},
	{RangeLow: 45, PrefLen: 7, RangeLen: 5},
	{RangeLow: 77, PrefLen: 7, RangeLen: 6},
	{RangeLow: 141, PrefLen: 7, RangeLen: 32},
})

// table B.12 (Table L): DT
var huffTableB12 = newHuffTable([]huffLine{
	{RangeLow: 1, PrefLen: 1, RangeLen: 0},
	{RangeLow: 2, PrefLen: 2, RangeLen: 0},
	{RangeLow: 3, PrefLen: 3, RangeLen: 1},
	{RangeLow: 5, PrefLen: 5, RangeLen: 0},
	{RangeLow: 6, PrefLen: 5, RangeLen: 1},
	{RangeLow: 8, PrefLen: 6, RangeLen: 1},
	{RangeLow: 10, PrefLen: 7, RangeLen: 0},
	{RangeLow: 11, PrefLen: 7, RangeLen: 1},
	{RangeLow: 13, PrefLen: 7, RangeLen: 2},
	{RangeLow: 17, PrefLen: 7, RangeLen: 3},
	{RangeLow: 25, PrefLen: 7, RangeLen: 4},
	{RangeLow: 41, PrefLen: 8, RangeLen: 5},
	{RangeLow: 73, PrefLen: 8, RangeLen: 32},
})

// table B.13 (Table M): DT
var huffTableB13 = newHuffTable([]huffLine{
	{RangeLow: 1, PrefLen: 1, RangeLen: 0},
	{RangeLow: 2, PrefLen: 3, RangeLen: 0},
	{RangeLow: 3, PrefLen: 4, RangeLen: 0},
	{RangeLow: 4, PrefLen: 5, RangeLen: 0},
	{RangeLow: 5, PrefLen: 4, RangeLen: 1},
	{RangeLow: 7, PrefLen: 3, RangeLen: 3},
	{RangeLow: 15, PrefLen: 6, RangeLen: 1},
	{RangeLow: 17, PrefLen: 6, RangeLen: 2},
	{RangeLow: 21, PrefLen: 6, RangeLen: 3},
	{RangeLow: 29, PrefLen: 6, RangeLen: 4},
	{RangeLow: 45, PrefLen: 6, RangeLen: 5},
	{RangeLow: 77, PrefLen: 7, RangeLen: 6},
	{RangeLow: 141, PrefLen: 7, RangeLen: 32},
})

var huffTableB14 = newHuffTable([]huffLine{
	{RangeLow: -2, PrefLen: 3, RangeLen: 0},
	{RangeLow: -1, PrefLen: 3, RangeLen: 0},
	{RangeLow: 0, PrefLen: 1, RangeLen: 0},
	{RangeLow: 1, PrefLen: 3, RangeLen: 0},
	{RangeLow: 2, PrefLen: 3, RangeLen: 0},
})

var huffTableB15 = newHuffTable([]huffLine{
	{RangeLow: -24, PrefLen: 7, RangeLen: 4},
	{RangeLow: -8, PrefLen: 6, RangeLen: 2},
	{RangeLow: -4, PrefLen: 5, RangeLen: 1},
	{RangeLow: -2, PrefLen: 4, RangeLen: 0},
	{RangeLow: -1, PrefLen: 3, RangeLen: 0},
	{RangeLow: 0, PrefLen: 1, RangeLen: 0},
	{RangeLow: 1, PrefLen: 3, RangeLen: 0},
	{RangeLow: 2, PrefLen: 4, RangeLen: 0},
	{RangeLow: 3, PrefLen: 5, RangeLen: 1},
	{RangeLow: 5, PrefLen: 6, RangeLen: 2},
	{RangeLow: 9, PrefLen: 7, RangeLen: 4},
	{RangeLow: -25, PrefLen: 7, RangeLen: 32, IsLower: true},
	{RangeLow: 25, PrefLen: 7, RangeLen: 32},
})
