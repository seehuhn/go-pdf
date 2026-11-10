// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package ccittfax

import (
	"math/rand"
	"testing"
)

// The 2D codec resolves changing elements through the precomputed
// changingElements index (findB1B2FromChanges, nextTwoChanges) instead of
// the original pixel-by-pixel scans, which were O(changes × Columns) for
// rows with many transitions.  These tests pin the fast lookups to the
// reference scans (findB1B2 and endOfRun) across exhaustive inputs.

func TestChangingElementsMatchesScan(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, cols := range []int{1, 2, 7, 8, 9, 16, 31, 100, 257} {
		for _, blackIs1 := range []bool{false, true} {
			p := Params{Columns: cols, K: -1, BlackIs1: blackIs1}
			stride := (cols + 7) / 8
			for range 300 {
				line := make([]byte, stride)
				for i := range line {
					line[i] = byte(rng.Intn(256))
				}
				if cols%8 != 0 { // clear bits beyond Columns
					line[stride-1] &^= byte((1 << (8 - cols%8)) - 1)
				}
				changes := p.changingElements(line, nil)

				// brute-force reference transition list
				var want []int
				prev := p.whiteBit()
				for x := range cols {
					if c := p.getPixel(line, x); c != prev {
						want = append(want, x)
						prev = c
					}
				}
				if len(changes) != len(want) {
					t.Fatalf("cols=%d: changes=%v want=%v", cols, changes, want)
				}
				for i := range want {
					if changes[i] != want[i] {
						t.Fatalf("cols=%d: changes=%v want=%v", cols, changes, want)
					}
				}

				for a0 := -1; a0 <= cols; a0++ {
					// the imaginary pixel before column 0 is white
					pa0 := p.whiteBit()
					if a0 >= 0 {
						pa0 = p.getPixel(line, a0)
					}
					// nextTwoChanges vs endOfRun-based a1/a2 (encoder
					// invariant: the run at a0 has colour currentBit)
					for _, cb := range []byte{0, 1} {
						if pa0 != cb {
							continue
						}
						ra1 := p.endOfRun(line, a0+1, cb)
						ra2 := p.endOfRun(line, ra1+1, 1-cb)
						ga1, ga2 := nextTwoChanges(changes, cols, a0)
						if ga1 != ra1 || ga2 != ra2 {
							t.Fatalf("nextTwoChanges cols=%d a0=%d cb=%d: want (%d,%d) got (%d,%d)",
								cols, a0, cb, ra1, ra2, ga1, ga2)
						}
					}
					// findB1B2FromChanges vs findB1B2
					for _, cb := range []byte{0, 1} {
						if a0 == -1 && cb != p.whiteBit() {
							continue // precondition of findB1B2
						}
						wb1, wb2 := p.findB1B2(line, a0, cb)
						gb1, gb2 := findB1B2FromChanges(changes, cols, a0, cb, p.whiteBit())
						if wb1 != gb1 || wb2 != gb2 {
							t.Fatalf("findB1B2 cols=%d a0=%d cb=%d: want (%d,%d) got (%d,%d)",
								cols, a0, cb, wb1, wb2, gb1, gb2)
						}
					}
				}
			}
		}
	}
}
