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
	"testing"
)

func TestIntegerRoundTrip(t *testing.T) {
	values := []int64{0, 1, 2, 3, 4, 19, 20, 83, 84, 339, 340, 4435, 4436, 10000,
		-1, -2, -3, -4, -19, -20, -83, -84, -339, -340, -4435, -4436}

	for _, v := range values {
		t.Run(fmt.Sprintf("%d", v), func(t *testing.T) {
			enc := newMQEncoder()
			ic := &intCtx{}
			ic.encode(enc, v)
			enc.flush()

			dec := newMQDecoder(enc.bytes())
			ic2 := &intCtx{}
			got := ic2.decode(dec)
			if got != v {
				t.Errorf("got %d, want %d (data=%X)", got, v, enc.bytes())
			}
		})
	}
}

func TestIntegerEdgeCases(t *testing.T) {
	// from mq_test_vectors.txt: mq_integer_edge_cases
	values := []int64{0, -1, 1, -2, 2, 127, 128, -128, -129, 255, 256, -256, 32767}
	expected := hexBytes("9C 35 4E 34 D1 2A 1F AE B6 CD CC 9E 47 C9 4E 1C 3F FF AC")

	enc := newMQEncoder()
	ic := &intCtx{}
	for _, v := range values {
		ic.encode(enc, v)
	}
	enc.flush()
	got := enc.bytes()

	if fmt.Sprintf("%X", got) != fmt.Sprintf("%X", expected) {
		t.Errorf("edge cases:\n  got  %X\n  want %X", got, expected)
	}
}

func TestIAIDRoundTrip(t *testing.T) {
	codeLen := 3
	values := []int{0, 1, 2, 3, 4, 5, 6, 7, 0, 7, 3, 5}

	enc := newMQEncoder()
	ic, _ := newIAIDCtx(codeLen)
	for _, v := range values {
		ic.encode(enc, codeLen, v)
	}
	enc.flush()

	dec := newMQDecoder(enc.bytes())
	ic2, _ := newIAIDCtx(codeLen)
	for i, want := range values {
		got := ic2.decode(dec, codeLen)
		if got != want {
			t.Fatalf("IAID[%d]: got %d, want %d", i, got, want)
		}
	}
}

func TestIAIDVector(t *testing.T) {
	// from mq_test_vectors.txt: mq_iaid
	codeLen := 2
	values := []int{0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3,
		0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3}
	expected := hexBytes("70 B6 EC BE AA AA AA AA AF FF AC")

	enc := newMQEncoder()
	ic, _ := newIAIDCtx(codeLen)
	for _, v := range values {
		ic.encode(enc, codeLen, v)
	}
	enc.flush()
	got := enc.bytes()

	if fmt.Sprintf("%X", got) != fmt.Sprintf("%X", expected) {
		t.Errorf("iaid:\n  got  %X\n  want %X", got, expected)
	}
}
