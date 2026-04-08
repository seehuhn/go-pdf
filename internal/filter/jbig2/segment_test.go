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
	"bytes"
	"testing"
)

func TestParseSegmentHeaderShortForm(t *testing.T) {
	// example from spec §7.2.8: segment #32, type 6, 3 referred-to segments
	data := []byte{
		0x00, 0x00, 0x00, 0x20, // segment number = 32
		0x86,             // flags: type=6, page assoc=1 byte, deferred=1
		0x6B,             // count=3 (bits 5-7), retention flags in bits 0-4
		0x02, 0x1E, 0x05, // referred-to segments: 2, 30, 5 (1 byte each, since seg# ≤ 256)
		0x04,                   // page association = 4
		0x00, 0x00, 0x00, 0x0A, // data length = 10
	}

	h, err := parseSegmentHeader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if h.Number != 32 {
		t.Errorf("segment number: got %d, want 32", h.Number)
	}
	if h.Type != 6 {
		t.Errorf("type: got %d, want 6", h.Type)
	}
	if len(h.RefSegments) != 3 {
		t.Fatalf("ref count: got %d, want 3", len(h.RefSegments))
	}
	wantRefs := []uint32{2, 30, 5}
	for i, want := range wantRefs {
		if h.RefSegments[i] != want {
			t.Errorf("ref[%d]: got %d, want %d", i, h.RefSegments[i], want)
		}
	}
	if h.PageAssoc != 4 {
		t.Errorf("page assoc: got %d, want 4", h.PageAssoc)
	}
	if h.DataLength != 10 {
		t.Errorf("data length: got %d, want 10", h.DataLength)
	}
}

func TestParseSegmentHeaderLongForm(t *testing.T) {
	// example from spec §7.2.8: segment #564, type 0, 9 referred-to segments,
	// 4-byte page association
	data := []byte{
		0x00, 0x00, 0x02, 0x34, // segment number = 564
		0x40,                   // flags: type=0, page assoc=4 bytes
		0xE0, 0x00, 0x00, 0x09, // long form: count=9
		0x02, 0xFD, // retention flags (2 bytes)
		0x01, 0x00, 0x00, 0x02, 0x00, 0x1E, // referred-to segments (2 bytes each)
		0x00, 0x05, 0x02, 0x00, 0x02, 0x01, // since seg #564 > 256 and ≤ 65536
		0x02, 0x02, 0x02, 0x03, 0x02, 0x04,
		0x00, 0x00, 0x04, 0x01, // page association = 1025
		0x00, 0x00, 0x01, 0x00, // data length = 256
	}

	h, err := parseSegmentHeader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if h.Number != 564 {
		t.Errorf("segment number: got %d, want 564", h.Number)
	}
	if h.Type != 0 {
		t.Errorf("type: got %d, want 0", h.Type)
	}
	if len(h.RefSegments) != 9 {
		t.Fatalf("ref count: got %d, want 9", len(h.RefSegments))
	}
	wantRefs := []uint32{0x0100, 0x0002, 0x001E, 0x0005, 0x0200, 0x0201, 0x0202, 0x0203, 0x0204}
	for i, want := range wantRefs {
		if h.RefSegments[i] != want {
			t.Errorf("ref[%d]: got %d, want %d", i, h.RefSegments[i], want)
		}
	}
	if h.PageAssoc != 1025 {
		t.Errorf("page assoc: got %d, want 1025", h.PageAssoc)
	}
}

func TestParseSegmentHeaderLongFormRetentionSize(t *testing.T) {
	// 7 referred-to segments: retention = ceil((7+1)/8) = 1 byte.
	// With the old formula (n+8)/7 this gave 2 bytes, causing a parse error.
	data := []byte{
		0x00, 0x00, 0x00, 0x08, // segment number = 8
		0x00,                   // flags: type=0, page assoc=1 byte
		0xE0, 0x00, 0x00, 0x07, // long form: count=7
		0x00,                                     // retention flags (1 byte)
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, // 7 referred-to segments (1 byte each)
		0x01,                   // page association = 1
		0x00, 0x00, 0x00, 0x00, // data length = 0
	}

	h, err := parseSegmentHeader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(h.RefSegments) != 7 {
		t.Fatalf("ref count: got %d, want 7", len(h.RefSegments))
	}
	for i := range 7 {
		want := uint32(i + 1)
		if h.RefSegments[i] != want {
			t.Errorf("ref[%d]: got %d, want %d", i, h.RefSegments[i], want)
		}
	}
	if h.PageAssoc != 1 {
		t.Errorf("page assoc: got %d, want 1", h.PageAssoc)
	}
}
