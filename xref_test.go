// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package pdf

import (
	"bytes"
	"strings"
	"testing"
)

func TestFindXref(t *testing.T) {
	in := "%PDF-1.7\nhello\nstartxref\n9\n%%EOF"
	r := &Reader{
		r: strings.NewReader(in),
	}
	start, err := r.findXRef(int64(len(in)))
	if err != nil {
		t.Error(err)
	}
	if start != 9 {
		t.Errorf("wrong xref start, expected 9 but got %d", start)
	}
}

func TestDecodeInt(t *testing.T) {
	cases := []struct {
		name    string
		buf     []byte
		want    int64
		wantErr bool
	}{
		{"empty", nil, 0, false},
		{"one byte", []byte{0x42}, 0x42, false},
		{"three bytes", []byte{0x01, 0x02, 0x03}, 0x010203, false},
		{"max int64", []byte{0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 1<<63 - 1, false},
		// W=[*,8,*] with the high bit set used to wrap to a negative
		// signed int64; now it must be rejected outright.
		{"high bit overflow", []byte{0x80, 0, 0, 0, 0, 0, 0, 0}, 0, true},
		{"all ones overflow", []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeInt(tc.buf)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCheckXRefStreamDict(t *testing.T) {
	cases := []struct {
		name    string
		dict    Dict
		wantErr bool
	}{
		{
			name: "valid",
			dict: Dict{
				"Size": Integer(10),
				"W":    Array{Integer(1), Integer(2), Integer(1)},
			},
		},
		{
			name: "Size above cap",
			dict: Dict{
				"Size": Integer(maxXRefSize + 1),
				"W":    Array{Integer(1), Integer(2), Integer(1)},
			},
			wantErr: true,
		},
		{
			name: "Index subsection extends past Size",
			dict: Dict{
				"Size":  Integer(10),
				"W":     Array{Integer(1), Integer(2), Integer(1)},
				"Index": Array{Integer(0), Integer(11)},
			},
			wantErr: true,
		},
		{
			name: "Index subsection start past Size",
			dict: Dict{
				"Size":  Integer(10),
				"W":     Array{Integer(1), Integer(2), Integer(1)},
				"Index": Array{Integer(11), Integer(1)},
			},
			wantErr: true,
		},
		{
			name: "Index covering exactly Size",
			dict: Dict{
				"Size":  Integer(10),
				"W":     Array{Integer(1), Integer(2), Integer(1)},
				"Index": Array{Integer(0), Integer(10)},
			},
		},
		{
			name: "multiple Index subsections within Size",
			dict: Dict{
				"Size":  Integer(100),
				"W":     Array{Integer(1), Integer(2), Integer(1)},
				"Index": Array{Integer(0), Integer(10), Integer(50), Integer(20)},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := checkXRefStreamDict(tc.dict, 1<<20)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// TestXRefStreamEntryCountCap verifies that the number of declared entries is
// bounded in proportion to the stream's raw size: the same /Size is rejected
// for a tiny stream but accepted once the raw length is large enough.
func TestXRefStreamEntryCountCap(t *testing.T) {
	dict := Dict{
		"Size": Integer(100000),
		"W":    Array{Integer(1), Integer(2), Integer(1)},
	}

	if _, _, err := checkXRefStreamDict(dict, 10); err == nil {
		t.Error("100000 entries from a 10-byte stream should be rejected")
	}
	if _, _, err := checkXRefStreamDict(dict, 100000); err != nil {
		t.Errorf("100000 entries from a 100000-byte stream should be accepted: %v", err)
	}
}

// TestXRefStreamObjectNumberOverflow verifies that compressed-object
// (type-2) xref-stream entries whose field-2 object number exceeds
// MaxUint32 are skipped rather than silently truncated.
func TestXRefStreamObjectNumberOverflow(t *testing.T) {
	xref := map[uint32]*xRefEntry{}
	// W = [1 8 1]: type byte, 8-byte object number, 1-byte index
	w := []int{1, 8, 1}
	ss := []*xRefSubSection{{Start: 5, Size: 1}}

	// type 2, object number 0x1_0000_0000 (> MaxUint32), index 0
	data := []byte{2, 0, 0, 0, 1, 0, 0, 0, 0, 0}
	if err := decodeXRefStream(xref, bytes.NewReader(data), w, ss); err != nil {
		t.Fatalf("decodeXRefStream: %v", err)
	}
	if _, ok := xref[5]; ok {
		t.Errorf("entry with object number > MaxUint32 should have been skipped")
	}
}

// TestXRefStreamGenerationOverflow verifies that xref-stream entries whose
// field-3 (generation) exceeds 65535 are silently dropped rather than
// truncated.  PDF 32000-2 §7.5.4 caps generations at 65535, and §7.6.3.2
// reserves only 2 bytes for the generation in per-object key derivation;
// preserving an out-of-range entry would collide with another object's
// encryption key.
func TestXRefStreamGenerationOverflow(t *testing.T) {
	xref := map[uint32]*xRefEntry{}
	// W = [1 1 8]: type byte, 1-byte byte-offset, 8-byte generation
	w := []int{1, 1, 8}
	ss := []*xRefSubSection{{Start: 5, Size: 1}}

	// type 1 (in-use), byte offset 0, generation 0x1_0000 (> 65535).
	// Generation is 8 bytes big-endian: 0x00 0x00 0x00 0x00 0x00 0x01 0x00 0x00
	data := []byte{1, 0, 0, 0, 0, 0, 0, 1, 0, 0}
	if err := decodeXRefStream(xref, bytes.NewReader(data), w, ss); err != nil {
		t.Fatalf("decodeXRefStream: %v", err)
	}
	if _, ok := xref[5]; ok {
		t.Errorf("entry with generation > 65535 should have been skipped")
	}
}

// TestAllocOverflowPanics verifies that Writer.Alloc panics rather than
// minting object numbers >= 2^24, which would collide on encryption-key
// derivation (PDF 32000-2 §7.6.3.2).
func TestAllocOverflowPanics(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.nextRef = maxXRefSize

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Alloc at nextRef=maxXRefSize did not panic")
		}
	}()
	w.Alloc()
}

// TestNewReferenceOverflowPanics verifies that NewReference panics for
// object numbers >= 2^24, since the encryption-key derivation only uses
// the low 3 bytes (PDF 32000-2 §7.6.3.2).
func TestNewReferenceOverflowPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewReference(maxXRefSize, 0) did not panic")
		}
	}()
	NewReference(maxXRefSize, 0)
}

func TestLastOccurence(t *testing.T) {
	buf := make([]byte, 2048)
	pat := "ABC"
	copy(buf[1023:], pat)

	r := &Reader{
		r: bytes.NewReader(buf),
	}
	pos, err := r.lastOccurence(pat, int64(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	if pos != 1023 {
		t.Errorf("found wrong position: expected 1023, got %d", pos)
	}
}
