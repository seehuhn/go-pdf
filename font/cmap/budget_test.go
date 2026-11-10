// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"testing"
	"time"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/internal/limits"
)

// a 4-byte code space that accepts every code, paired below with a CID/bf
// range spanning the whole space (2^32 codes).
var fullByteRange = charcode.CodeSpaceRange{
	{Low: []byte{0, 0, 0, 0}, High: []byte{0xFF, 0xFF, 0xFF, 0xFF}},
}

// TestFileAllBudget checks that File.All stops enumerating once it reaches
// limits.MaxCMapMappings, so a wide cidrange cannot drive an unbounded
// loop or unbounded map growth.
func TestFileAllBudget(t *testing.T) {
	codec, err := charcode.NewCodec(fullByteRange)
	if err != nil {
		t.Fatal(err)
	}
	f := &File{
		Name:           "Evil",
		ROS:            &cid.SystemInfo{Registry: "A", Ordering: "B"},
		CodeSpaceRange: fullByteRange,
		CIDRanges: []Range{
			{First: []byte{0, 0, 0, 0}, Last: []byte{0xFF, 0xFF, 0xFF, 0xFF}, Value: 1},
		},
	}

	limit := limits.MaxCMapMappings
	count := 0
	for range f.All(codec) {
		count++
		if count > limit {
			break
		}
	}
	if count > limit {
		t.Errorf("File.All did not stop at the budget of %d entries", limit)
	}
}

// TestFileAllBudgetInvalidCodes checks that the budget bounds iteration even
// when every enumerated code is rejected by the codec and nothing is yielded.
// Without a cap the bare loop would spin through all 2^32 byte sequences.
func TestFileAllBudgetInvalidCodes(t *testing.T) {
	// code space accepts only single-byte codes, so every 4-byte code the
	// range enumerates is rejected by Decode and never yielded.
	cs := charcode.CodeSpaceRange{{Low: []byte{0}, High: []byte{0xFF}}}
	codec, err := charcode.NewCodec(cs)
	if err != nil {
		t.Fatal(err)
	}
	f := &File{
		Name:           "Evil",
		ROS:            &cid.SystemInfo{Registry: "A", Ordering: "B"},
		CodeSpaceRange: cs,
		CIDRanges: []Range{
			{First: []byte{0, 0, 0, 0}, Last: []byte{0xFF, 0xFF, 0xFF, 0xFF}, Value: 1},
		},
	}

	done := make(chan struct{})
	go func() {
		for range f.All(codec) {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("File.All did not terminate on an all-invalid wide range")
	}
}

// TestToUnicodeAllBudget checks the same budget for ToUnicodeFile.All driven by
// a wide bfrange.
func TestToUnicodeAllBudget(t *testing.T) {
	codec, err := charcode.NewCodec(fullByteRange)
	if err != nil {
		t.Fatal(err)
	}
	tu := &ToUnicodeFile{
		CodeSpaceRange: fullByteRange,
		Ranges: []ToUnicodeRange{
			{
				First:  []byte{0, 0, 0, 0},
				Last:   []byte{0xFF, 0xFF, 0xFF, 0xFF},
				Values: []string{"A"},
			},
		},
	}

	limit := limits.MaxCMapMappings
	count := 0
	for range tu.All(codec) {
		count++
		if count > limit {
			break
		}
	}
	if count > limit {
		t.Errorf("ToUnicodeFile.All did not stop at the budget of %d entries", limit)
	}
}
