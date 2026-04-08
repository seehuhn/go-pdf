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

func TestRefinementDecodeTemplate1(t *testing.T) {
	// from mq_test_vectors.txt: mq_refinement_template1
	// reference=diagonal 16x16, target=checkerboard 16x16
	// template=1, ref_dx=1, ref_dy=-2
	ref := makeDiagonal(16, 16)
	expected := makeCheckerboard(16, 16)

	data := hexBytes("BB D0 31 DE B9 FF")
	dec := newMQDecoder(data)

	p := &refinementParams{
		Width:     16,
		Height:    16,
		Template:  1,
		Reference: ref,
		RefDX:     1,
		RefDY:     -2,
	}
	got, err := decodeRefinementRegion(dec, p, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !bitmapsEqual(got, expected) {
		t.Errorf("refinement template 1 decode failed")
	}
}

func TestRefinementRoundTrip(t *testing.T) {
	ref := makeDiagonal(16, 16)
	target := makeCheckerboard(16, 16)

	for _, tmpl := range []int{0, 1} {
		t.Run(fmt.Sprintf("template%d", tmpl), func(t *testing.T) {
			p := &refinementParams{
				Width:     16,
				Height:    16,
				Template:  tmpl,
				Reference: ref,
				RefDX:     0,
				RefDY:     0,
				ATX:       [2]int8{-1, -1},
				ATY:       [2]int8{-1, -1},
			}

			data := encodeRefinementRegion(target, p)

			// decode
			dec := newMQDecoder(data)
			got, err := decodeRefinementRegion(dec, p, nil)
			if err != nil {
				t.Fatal(err)
			}

			if !bitmapsEqual(got, target) {
				t.Errorf("round-trip failed (data=%d bytes)", len(data))
			}
		})
	}
}
