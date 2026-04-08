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

import "testing"

func TestHuffmanTableB1(t *testing.T) {
	testCases := []int64{0, 1, 15, 16, 100, 271, 272, 1000, 65807, 65808, 100000}
	for _, v := range testCases {
		w := newBitWriter()
		if err := huffTableB1.encode(w, v); err != nil {
			t.Errorf("encode(%d): %v", v, err)
			continue
		}
		w.align()
		r := newHuffReader(w.bytes())
		got := huffTableB1.decode(r)
		if got != v {
			t.Errorf("Table B.1: encode/decode(%d) = %d", v, got)
		}
	}
}

func TestHuffmanTableB2OOB(t *testing.T) {
	w := newBitWriter()
	if err := huffTableB2.encodeOOB(w); err != nil {
		t.Fatal(err)
	}
	w.align()
	r := newHuffReader(w.bytes())
	got := huffTableB2.decode(r)
	if got != oobResult {
		t.Errorf("Table B.2 OOB: got %d, want OOB", got)
	}
}

func TestHuffmanInvalidCode(t *testing.T) {
	// incomplete table: only codes 0 and 10 are defined;
	// bit pattern 11... has no match and should trigger an error
	incomplete := newHuffTable([]huffLine{
		{RangeLow: 0, PrefLen: 1, RangeLen: 0}, // code 0 -> value 0
		{RangeLow: 1, PrefLen: 2, RangeLen: 0}, // code 10 -> value 1
		// code 11 is unassigned
	})
	incomplete.assignCodes()

	// data 0xC0 = 11000000: starts with unassigned code 11
	data := []byte{0xC0, 0x00, 0x00, 0x00, 0x00}
	hr := newHuffReader(data)

	incomplete.decode(hr)
	if hr.err == nil {
		t.Fatalf("expected error for invalid Huffman code")
	}

	// subsequent decode should short-circuit
	got := incomplete.decode(hr)
	if got != 0 {
		t.Errorf("decode after error: got %d, want 0", got)
	}
	if hr.err == nil {
		t.Errorf("error should persist after second decode")
	}
}

func TestHuffmanValidCodeNoError(t *testing.T) {
	w := newBitWriter()
	if err := huffTableB1.encode(w, 5); err != nil {
		t.Fatal(err)
	}
	w.align()
	hr := newHuffReader(w.bytes())
	got := huffTableB1.decode(hr)
	if got != 5 {
		t.Errorf("decode: got %d, want 5", got)
	}
	if hr.err != nil {
		t.Errorf("unexpected error: %v", hr.err)
	}
}

func TestHuffmanDecodeEOF(t *testing.T) {
	// empty data: decode should set an error, not silently return a value
	hr := newHuffReader(nil)
	got := huffTableB1.decode(hr)
	if hr.err == nil {
		t.Fatalf("expected error on empty data, got value %d", got)
	}

	// one valid code followed by EOF: first decode succeeds,
	// second should error
	// Table B.1 code 0 (length 1) decodes to range [0,15]
	hr2 := newHuffReader([]byte{0x00}) // 8 zero bits
	v := huffTableB1.decode(hr2)
	if hr2.err != nil {
		t.Fatalf("unexpected error on first decode: %v", hr2.err)
	}
	if v < 0 || v > 15 {
		t.Fatalf("first decode: got %d, want 0-15", v)
	}
	// the first decode consumed 1 prefix bit + 4 range bits = 5 bits;
	// 3 bits remain, not enough for any full code + range
	// eventually the reader runs out and should error
	huffTableB1.decode(hr2)
	if hr2.err == nil {
		t.Fatalf("expected error on second decode from truncated data")
	}
}
