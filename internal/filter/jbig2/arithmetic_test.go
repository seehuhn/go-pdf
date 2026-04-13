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
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func hexBytes(s string) []byte {
	s = strings.ReplaceAll(s, " ", "")
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func TestMQBinary(t *testing.T) {
	// From mq_test_vectors.txt: mq_binary
	decisions := []int{
		0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1,
		0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1,
		0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1,
		0, 1,
	}
	contexts := []int{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0x00, 0x01,
	}
	expected := hexBytes("39 56 72 A7 FF AC")

	enc := newMQEncoder()
	ctxs := make([]byte, 16) // 16 contexts, all initialized to 0

	for i := range decisions {
		enc.encode(&ctxs[contexts[i]], decisions[i])
	}
	enc.flush()

	got := enc.bytes()
	if fmt.Sprintf("%X", got) != fmt.Sprintf("%X", expected) {
		t.Errorf("mq_binary:\n  got  %X\n  want %X", got, expected)
	}
}

func TestMQRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		decisions []int
		nCtx      int
	}{
		{"single_0", []int{0}, 1},
		{"single_1", []int{1}, 1},
		{"two_same", []int{0, 0}, 1},
		{"two_diff", []int{0, 1}, 1},
		{"ten_mixed", []int{0, 1, 0, 0, 1, 1, 0, 1, 0, 0}, 1},
		{"multi_ctx", []int{0, 1, 0, 1, 0, 1, 0, 1, 0, 1}, 5},
		{"many_zeros", make([]int, 100), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := newMQEncoder()
			ctxs := make([]byte, max(tt.nCtx, 1))
			for i, d := range tt.decisions {
				enc.encode(&ctxs[i%len(ctxs)], d)
			}
			enc.flush()
			data := enc.bytes()

			dec := newMQDecoder(data)
			ctxs = make([]byte, max(tt.nCtx, 1))
			for i, want := range tt.decisions {
				got := dec.decode(&ctxs[i%len(ctxs)])
				if got != want {
					t.Fatalf("decision %d: got %d, want %d (data=%X)", i, got, want, data)
				}
			}
		})
	}
}

func TestMQDecodeKnownVector(t *testing.T) {
	// Decode the mq_binary test vector and verify decisions
	data := hexBytes("39 56 72 A7 FF")
	decisions := []int{
		0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1,
		0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1,
		0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1,
		0, 1,
	}
	contexts := []int{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0x00, 0x01,
	}

	dec := newMQDecoder(data)
	ctxs := make([]byte, 16)
	for i, want := range decisions {
		got := dec.decode(&ctxs[contexts[i]])
		if got != want {
			t.Fatalf("decision %d (ctx %02X): got %d, want %d", i, contexts[i], got, want)
		}
	}
}
