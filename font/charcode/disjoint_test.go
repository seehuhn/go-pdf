// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package charcode

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDisjointRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	for range 3000 {
		csr := randomRanges(rng)
		got, err := csr.Disjoint()
		if err != nil {
			t.Fatalf("ranges %v: %v", csr, err)
		}
		if !got.Equivalent(csr) {
			t.Fatalf("ranges %v: Disjoint gives %v", csr, got)
		}
		for i := range got {
			for j := range i {
				if len(got[i].Low) == len(got[j].Low) && rangesOverlap(got[i], got[j]) {
					t.Fatalf("ranges %v: %v and %v overlap", csr, got[j], got[i])
				}
			}
		}
	}
}

func TestDisjointKeepsOrder(t *testing.T) {
	csr := CodeSpaceRange{
		{Low: []byte{0x81, 0x40}, High: []byte{0x9F, 0xFC}},
		{Low: []byte{0x00}, High: []byte{0x80}},
		{Low: []byte{0x82, 0x50}, High: []byte{0x83, 0x60}}, // contained in the first
		{Low: []byte{0xA0}, High: []byte{0xDF}},
		{Low: []byte{0x00}, High: []byte{0x80}}, // repeated
	}
	got, err := csr.Disjoint()
	if err != nil {
		t.Fatal(err)
	}
	want := CodeSpaceRange{csr[0], csr[1], csr[3]}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Disjoint (-want +got):\n%s", d)
	}

	if got, _ := UTF8.Disjoint(); !cmp.Equal(got, UTF8) {
		t.Errorf("UTF8 changed to %v", got)
	}
}

func TestDisjointSplits(t *testing.T) {
	csr := CodeSpaceRange{
		{Low: []byte{0x00, 0x00}, High: []byte{0x80, 0xFF}},
		{Low: []byte{0x40, 0x00}, High: []byte{0xFF, 0xFF}},
	}
	got, err := csr.Disjoint()
	if err != nil {
		t.Fatal(err)
	}
	want := CodeSpaceRange{csr[0], {Low: []byte{0x81, 0x00}, High: []byte{0xFF, 0xFF}}}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Disjoint (-want +got):\n%s", d)
	}
}

// TestDisjointTooComplex checks that the work for heavily overlapping
// ranges is bounded linearly in the number of ranges.
func TestDisjointTooComplex(t *testing.T) {
	crosses := func(k int) CodeSpaceRange {
		var csr CodeSpaceRange
		for i := range k {
			b := byte(i)
			csr = append(csr,
				Range{Low: []byte{b, 0x00, 0x00}, High: []byte{b, 0xFF, 0xFF}},
				Range{Low: []byte{0x00, b, 0x00}, High: []byte{0xFF, b, 0xFF}},
				Range{Low: []byte{0x00, 0x00, b}, High: []byte{0xFF, 0xFF, b}})
		}
		return csr
	}
	allocs := func(k int) float64 {
		csr := crosses(k)
		return testing.AllocsPerRun(3, func() {
			if _, err := csr.Disjoint(); err != ErrTooComplex {
				t.Fatalf("%d ranges: got error %v, want ErrTooComplex", len(csr), err)
			}
		})
	}
	a1, a2 := allocs(64), allocs(128)
	t.Logf("%g allocations for 192 ranges, %g for 384", a1, a2)
	if a2 > 3*a1 {
		t.Errorf("allocations grow too fast: %g for 192 ranges, %g for 384", a1, a2)
	}
}

// stripes returns n pairwise disjoint three-byte ranges which all cover
// the full first byte.
func stripes(n int) CodeSpaceRange {
	csr := make(CodeSpaceRange, n)
	for k := range csr {
		a, b := byte(k/256), byte(k%256)
		csr[k] = Range{Low: []byte{0x00, a, b}, High: []byte{0xFF, a, b}}
	}
	return csr
}

// TestDisjointWideFirstByte checks that disjoint ranges which share a wide
// first byte are accepted, as they are by [NewCodec].
func TestDisjointWideFirstByte(t *testing.T) {
	csr := stripes(2048)
	if _, err := NewCodec(csr); err != nil {
		t.Fatal(err)
	}
	got, err := csr.Disjoint()
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(csr, got); d != "" {
		t.Errorf("Disjoint (-want +got):\n%s", d)
	}
}

// findPairs returns the overlapping pairs of distinct ranges in csr, sorted,
// together with the work of the search.
func findPairs(csr CodeSpaceRange, b *stepBudget) ([][2]int, int, error) {
	f := &pairFinder{csr: csr, b: b}
	byLen := make(map[int][]int)
	for i, r := range csr {
		byLen[len(r.Low)] = append(byLen[len(r.Low)], i)
	}
	for n, idx := range byLen {
		if err := f.stream(idx, idx, n-1); err != nil {
			return nil, f.work, err
		}
	}
	slices.SortFunc(f.pairs, func(p, q [2]int) int {
		if p[0] != q[0] {
			return p[0] - q[0]
		}
		return p[1] - q[1]
	})
	return f.pairs, f.work, nil
}

func TestPairFinderRandom(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 14))
	for range 300 {
		// narrow intervals with few distinct values, so that there are
		// repeated keys, repeated ranges and a moderate number of overlaps
		n := 1 + rng.IntN(200)
		width := 1 + rng.IntN(40)
		csr := make(CodeSpaceRange, n)
		for k := range csr {
			l := 1 + rng.IntN(4)
			lo, hi := make([]byte, l), make([]byte, l)
			for d := range l {
				lo[d] = byte(rng.IntN(64))
				hi[d] = lo[d] + byte(rng.IntN(width))
			}
			csr[k] = Range{Low: lo, High: hi}
		}

		var want [][2]int
		for i := range csr {
			for j := i + 1; j < len(csr); j++ {
				if len(csr[i].Low) == len(csr[j].Low) && rangesOverlap(csr[i], csr[j]) {
					want = append(want, [2]int{i, j})
				}
			}
		}
		got, _, err := findPairs(csr, &stepBudget{limit: math.MaxInt})
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(want, got); d != "" {
			t.Fatalf("ranges %v: pairs differ (-want +got):\n%s", csr, d)
		}
	}
}

// TestDisjointScaling checks that the search for overlaps in disjoint
// ranges charges nothing to the budget, and that its work grows like
// n log² n for three-byte ranges.
func TestDisjointScaling(t *testing.T) {
	families := map[string]func(n int) CodeSpaceRange{
		"stripes": stripes,
		"slabs":   func(n int) CodeSpaceRange { return slabs(n / 3) },
		"grid": func(n int) CodeSpaceRange {
			// n/4 vertical and n/4 horizontal bars in the first two bytes,
			// separated by the last byte
			var csr CodeSpaceRange
			for k := range n / 4 {
				a, c := byte(k%256), byte(2*(k/256))
				csr = append(csr,
					Range{Low: []byte{a, 0x00, c}, High: []byte{a, 0xFF, c}},
					Range{Low: []byte{0x00, a, c + 1}, High: []byte{0xFF, a, c + 1}})
			}
			return csr
		},
	}
	for name, family := range families {
		t.Run(name, func(t *testing.T) {
			var prev float64
			for n := 64; n <= 4096; n *= 2 {
				csr := family(n)
				if name == "slabs" && n > 3*85 {
					break // slabs(k) needs 3k < 256
				}
				pairs, work, err := findPairs(csr, &stepBudget{})
				if err != nil {
					t.Fatalf("%d ranges: %v", len(csr), err)
				}
				if len(pairs) > 0 {
					t.Fatalf("%d ranges: unexpected overlaps %v", len(csr), pairs[0])
				}
				if got, err := csr.Disjoint(); err != nil || len(got) != len(csr) {
					t.Fatalf("%d ranges: Disjoint gives %d ranges, %v", len(csr), len(got), err)
				}

				m := float64(len(csr))
				lg := math.Log2(m)
				norm := float64(work) / (m * lg * lg)
				t.Logf("%d ranges: work %d, %.2f n log² n", len(csr), work, norm)
				if prev > 0 && norm > 1.5*prev {
					t.Errorf("%d ranges: work %d grows faster than n log² n", len(csr), work)
				}
				prev = max(prev, norm)
			}
		})
	}
}
