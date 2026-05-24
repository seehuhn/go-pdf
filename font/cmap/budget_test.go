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

package cmap

import (
	"testing"
	"time"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/internal/streamlimits"
)

// a 4-byte code space that accepts every code, paired below with a CID/bf
// range spanning the whole space (2^32 codes).
var fullByteRange = charcode.CodeSpaceRange{
	{Low: []byte{0, 0, 0, 0}, High: []byte{0xFF, 0xFF, 0xFF, 0xFF}},
}

// TestFileAllBudget checks that File.All stops enumerating once it reaches
// streamlimits.MaxCMapMappings, so a wide cidrange cannot drive an unbounded
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

	limit := streamlimits.MaxCMapMappings
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

	limit := streamlimits.MaxCMapMappings
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
