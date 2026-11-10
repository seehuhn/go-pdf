// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSegmentRoundTrip(t *testing.T) {
	segs := []*segment{
		{number: 0, typ: segPageInfo, page: 1, data: pageInfoData(8, 2, 0)},
		{number: 1, typ: segSymbolDict, page: 0, retainBits: 0x01, data: []byte{1, 2, 3}},
		{number: 2, typ: segImmediateText, page: 1, refs: []uint32{1}, retainBits: 0x02,
			deferredNonRetain: true, data: []byte{4}},
		{number: 3, typ: segEndOfPage, page: 1, pageLong: true, data: []byte{}},
		{number: 300, typ: segImmediateText, page: 256, pageLong: true, refs: []uint32{1, 2, 299}, data: []byte{}},
		{number: 70000, typ: segImmediateText, page: 2, refs: []uint32{65537, 3, 1, 0}, data: []byte{}},
	}
	buf := encodeSegments(segs)
	got, err := parseSegments(buf)
	if err != nil {
		t.Fatal(err)
	}
	opt := cmp.AllowUnexported(segment{})
	if diff := cmp.Diff(segs, got, opt); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
	if again := encodeSegments(got); !bytes.Equal(again, buf) {
		t.Errorf("re-encoded stream differs")
	}

	// a truncated stream is accepted only at segment boundaries
	ends := map[int]bool{0: true}
	p := 0
	for _, s := range segs {
		p += s.headerLen() + len(s.data)
		ends[p] = true
	}
	if p != len(buf) {
		t.Errorf("header lengths add up to %d, stream has %d bytes", p, len(buf))
	}
	for i := range len(buf) {
		if _, err := parseSegments(buf[:i]); (err == nil) != ends[i] {
			t.Errorf("prefix of %d bytes: err = %v", i, err)
		}
	}
}

func TestSegmentFlagBits(t *testing.T) {
	// segment 5, type 6 with the deferred non-retain bit, one referred-to
	// segment with retain bits 0b00011, page 1, no data
	in := []byte{0, 0, 0, 5, 0x86, 0x23, 4, 1, 0, 0, 0, 0}
	segs, err := parseSegments(in)
	if err != nil {
		t.Fatal(err)
	}
	if out := encodeSegments(segs); !bytes.Equal(out, in) {
		t.Errorf("got %X, want %X", out, in)
	}
}

func TestAppendEndOfPage(t *testing.T) {
	segs := []*segment{
		{number: 4, typ: segPageInfo, page: 3},
		{number: 7, typ: segImmediateGeneric, page: 3},
		{number: 5, typ: segImmediateGeneric, page: 3},
	}
	segs, err := appendEndOfPage(segs)
	if err != nil {
		t.Fatal(err)
	}
	eop := segs[len(segs)-1]
	if eop.typ != segEndOfPage || eop.number != 8 || eop.page != 3 {
		t.Errorf("got end of page segment %+v", eop)
	}
	if _, err := appendEndOfPage(segs); err == nil {
		t.Error("second end of page segment accepted")
	}
	if _, err := appendEndOfPage(segs[1:2]); err == nil {
		t.Error("stream without page information accepted")
	}
}
