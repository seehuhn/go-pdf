// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package charcode

import (
	"bytes"
	"testing"
)

func TestSeqGen(t *testing.T) {
	testCases := []struct {
		name       string
		ranges     []CodeSpaceRange
		wantValues [][]byte  // Expected test values at each position
		wantSeqs   [][4]byte // Expected sequences from All()
	}{
		{
			name: "simple single-byte range",
			ranges: []CodeSpaceRange{
				{{Low: []byte{1}, High: []byte{2}}},
			},
			wantValues: [][]byte{
				{0, 1, 3}, // Position 0: breaks at 0,1,3
				{0},
				{0},
				{0},
			},
			wantSeqs: [][4]byte{
				{0, 0, 0, 0},
				{1, 0, 0, 0},
				{3, 0, 0, 0},
			},
		},
		{
			name: "two overlapping two-byte ranges",
			ranges: []CodeSpaceRange{
				{
					{Low: []byte{1, 1}, High: []byte{1, 2}},
					{Low: []byte{2, 1}, High: []byte{2, 1}},
				},
			},
			wantValues: [][]byte{
				{0, 1, 2, 3},
				{0, 1, 2, 3},
				{0},
				{0},
			},
			wantSeqs: [][4]byte{
				{0, 0, 0, 0}, {0, 1, 0, 0}, {0, 2, 0, 0}, {0, 3, 0, 0},
				{1, 0, 0, 0}, {1, 1, 0, 0}, {1, 2, 0, 0}, {1, 3, 0, 0},
				{2, 0, 0, 0}, {2, 1, 0, 0}, {2, 2, 0, 0}, {2, 3, 0, 0},
				{3, 0, 0, 0}, {3, 1, 0, 0}, {3, 2, 0, 0}, {3, 3, 0, 0},
			},
		},
		{
			name: "all bytes used",
			ranges: []CodeSpaceRange{
				{{Low: []byte{0, 0, 0, 0}, High: []byte{0xFF, 0xFF, 0xFF, 0x7F}}},
				{{Low: []byte{0, 0, 0, 0}, High: []byte{0xFF, 0x7F, 0xFF, 0xFF}}},
			},
			wantValues: [][]byte{
				{0},
				{0, 128},
				{0},
				{0, 128},
			},
			wantSeqs: [][4]byte{
				{0, 0, 0, 0}, {0, 0, 0, 128},
				{0, 128, 0, 0}, {0, 128, 0, 128},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test findTestSequenceBytes
			got := testSequences(tc.ranges...)

			if len(got) != len(tc.wantValues) {
				t.Errorf("len(findTestSequenceBytes) = %d, want %d", len(got), len(tc.wantValues))
			}
			for i := range got {
				if !bytes.Equal(got[i], tc.wantValues[i]) {
					t.Errorf("position %d values = %v, want %v", i, got[i], tc.wantValues[i])
				}
			}

			// Test All() iterator
			var gotSeqs [][4]byte
			for seq := range got.All() {
				var seqArray [4]byte
				copy(seqArray[:], seq)
				gotSeqs = append(gotSeqs, seqArray)
			}

			if len(gotSeqs) != len(tc.wantSeqs) {
				t.Errorf("got %d sequences, want %d", len(gotSeqs), len(tc.wantSeqs))
			}
			for i := range gotSeqs {
				if i >= len(tc.wantSeqs) {
					break
				}
				if gotSeqs[i] != tc.wantSeqs[i] {
					t.Errorf("sequence %d = %v, want %v", i, gotSeqs[i], tc.wantSeqs[i])
				}
			}
		})
	}
}
