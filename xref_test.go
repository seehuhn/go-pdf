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
			_, _, err := checkXRefStreamDict(tc.dict)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
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

// TestWideGenerationXRefTable verifies that writing a generation > 65535
// to a classic xref table is rejected, while the same value round-trips
// through an xref stream.
func TestWideGenerationXRefTable(t *testing.T) {
	tests := []struct {
		name    string
		opt     *WriterOptions
		wantErr bool
	}{
		// V1.7 default: xref stream form, accepts wide generation
		{"stream form", nil, false},
		// HumanReadable forces classic xref table form, rejects wide generation
		{"table form", &WriterOptions{HumanReadable: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			w, err := NewWriter(buf, V1_7, tt.opt)
			if err != nil {
				t.Fatal(err)
			}
			w.GetMeta().Catalog.Pages = w.Alloc()

			ref := NewReference(42, 100000)
			err = w.Put(ref, Integer(1))
			if err != nil {
				t.Fatal(err)
			}
			err = w.Close()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
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
