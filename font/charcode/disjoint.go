// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package charcode

import (
	"cmp"
	"slices"
)

// Disjoint returns a code space range which describes the same codes as
// csr, and in which no two ranges overlap.  PDF files must not contain
// overlapping code space ranges, so this is used before a code space range
// is written.
//
// All ranges in csr must be valid.  A range contained in an earlier range
// is left out.  If the remaining ranges do not overlap, they are returned in
// their original order.  Otherwise, every range which overlaps an earlier
// one is replaced by the parts not covered by earlier ranges.  If the
// overlaps need more steps than [NewCodec] may use for the lookup tree, the
// error is [ErrTooComplex].
func (csr CodeSpaceRange) Disjoint() (CodeSpaceRange, error) {
	b := &stepBudget{limit: treeBudget(len(csr))}

	// Repeated ranges would contribute a quadratic number of pairs, so
	// they are removed before the search.
	order := make([]int, len(csr))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(i, j int) int {
		if c := compareRange(csr[i], csr[j]); c != 0 {
			return c
		}
		return cmp.Compare(i, j)
	})
	dropped := make([]bool, len(csr))
	for k := 1; k < len(order); k++ {
		if compareRange(csr[order[k-1]], csr[order[k]]) == 0 {
			dropped[order[k]] = true
		}
	}

	f := &pairFinder{csr: csr, b: b}
	for i := range order {
		order[i] = i
	}
	order = slices.DeleteFunc(order, func(i int) bool { return dropped[i] })
	slices.SortStableFunc(order, func(i, j int) int {
		return cmp.Compare(len(csr[i].Low), len(csr[j].Low))
	})
	for len(order) > 0 {
		n := len(csr[order[0]].Low)
		end := 1
		for end < len(order) && len(csr[order[end]].Low) == n {
			end++
		}
		if err := f.stream(order[:end], order[:end], n-1); err != nil {
			return nil, err
		}
		order = order[end:]
	}
	pairs := f.pairs

	for _, p := range pairs {
		if rangeContains(csr[p[0]], csr[p[1]]) {
			dropped[p[1]] = true
		}
	}
	pairs = slices.DeleteFunc(pairs, func(p [2]int) bool {
		return dropped[p[0]] || dropped[p[1]]
	})
	if len(pairs) == 0 {
		var kept CodeSpaceRange
		for i, r := range csr {
			if !dropped[i] {
				kept = append(kept, r)
			}
		}
		return kept, nil
	}

	// earlier[j] lists the kept ranges before j which overlap j, in order
	slices.SortFunc(pairs, func(p, q [2]int) int {
		if c := cmp.Compare(p[1], q[1]); c != 0 {
			return c
		}
		return cmp.Compare(p[0], q[0])
	})
	earlier := make([][]int, len(csr))
	for _, p := range pairs {
		earlier[p[1]] = append(earlier[p[1]], p[0])
	}

	// Pieces of earlier ranges which do not overlap r leave the pieces of
	// r unchanged, so only the pieces of overlapping ranges are visited.
	pieces := make([]CodeSpaceRange, len(csr))
	var res CodeSpaceRange
	for j, r := range csr {
		if dropped[j] {
			continue
		}
		cur := CodeSpaceRange{r}
	subtract:
		for _, i := range earlier[j] {
			for _, q := range pieces[i] {
				var next CodeSpaceRange
				for _, p := range cur {
					if err := b.spend(1); err != nil {
						return nil, err
					}
					if rangesOverlap(p, q) {
						if err := b.spend(2 * len(p.Low)); err != nil {
							return nil, err
						}
					}
					next = subtractRange(next, p, q)
				}
				cur = next
				if len(cur) == 0 {
					break subtract
				}
			}
		}
		pieces[j] = cur
		res = append(res, cur...)
	}
	return res, nil
}

// stepBudget counts the steps of a computation, see [treeBudget].
type stepBudget struct {
	steps, limit int
}

func (b *stepBudget) spend(k int) error {
	b.steps += k
	if b.steps > b.limit {
		return ErrTooComplex
	}
	return nil
}

// pairFinder lists the pairs of overlapping ranges in a code space range.
//
// Two ranges overlap if and only if, at every byte position, the low byte
// of one range lies in the interval of the other.  To make the choice
// unique, every range r is given the key (r.Low[d], index of r) at byte
// position d, and the interval of r at position d consists of the keys
// greater than the key of r, with first component at most r.High[d].
// For two distinct ranges which overlap at position d, exactly one has its
// key in the interval of the other.
//
// The search is a multi-level segment tree, built on the fly over the keys
// at the last byte position and recursing to the earlier positions for
// ranges which span a segment.  For n ranges of length m, the work is
// O(n log^m n) plus the number of overlapping pairs, and only the pairs are
// charged to the budget.
type pairFinder struct {
	csr   CodeSpaceRange
	b     *stepBudget
	pairs [][2]int

	// work counts the elementary steps of the search, for testing
	work int
}

// bruteForceLimit is the number of keys below which a segment is not split
// further.
const bruteForceLimit = 8

// compareKeys compares the keys of ranges i and j at position d.
func (f *pairFinder) compareKeys(i, j, d int) int {
	if c := cmp.Compare(f.csr[i].Low[d], f.csr[j].Low[d]); c != 0 {
		return c
	}
	return cmp.Compare(i, j)
}

// keyLess reports whether the key of range i at position d is less than
// the key of range j.
func (f *pairFinder) keyLess(i, j, d int) bool {
	return f.compareKeys(i, j, d) < 0
}

// contains reports whether the key of range p at position d lies in the
// interval of range i.
func (f *pairFinder) contains(i, p, d int) bool {
	return f.keyLess(i, p, d) && f.csr[p].Low[d] <= f.csr[i].High[d]
}

// meets reports whether the interval of range i at position d contains
// a key between the keys of ranges a and b, inclusive.
func (f *pairFinder) meets(i, a, b, d int) bool {
	return f.keyLess(i, b, d) && f.csr[a].Low[d] <= f.csr[i].High[d]
}

// stream records the pairs (i, p) with i in is and p in ps, such that the
// two ranges overlap at the positions before d, and the key of p at
// position d lies in the interval of i.  Ranges in is and ps must have
// length greater than d.
func (f *pairFinder) stream(is, ps []int, d int) error {
	if len(is) == 0 || len(ps) == 0 {
		return nil
	}
	f.work += sortCost(len(ps))
	ps = slices.Clone(ps)
	slices.SortFunc(ps, func(i, j int) int {
		return f.compareKeys(i, j, d)
	})
	return f.split(is, ps, d)
}

// split is like stream, but needs ps sorted by key at position d.
func (f *pairFinder) split(is, ps []int, d int) error {
	if len(is) == 0 {
		return nil
	}

	if len(ps) <= bruteForceLimit {
		for _, i := range is {
			for _, p := range ps {
				f.work++
				if f.contains(i, p, d) && rangesOverlap(f.csr[i].prefix(d), f.csr[p].prefix(d)) {
					if err := f.emit(i, p); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}

	first, last := ps[0], ps[len(ps)-1]
	mid := len(ps) / 2
	var span, left, right []int
	for _, i := range is {
		f.work++
		switch {
		case f.contains(i, first, d) && f.contains(i, last, d):
			span = append(span, i)
		default:
			if f.meets(i, first, ps[mid-1], d) {
				left = append(left, i)
			}
			if f.meets(i, ps[mid], last, d) {
				right = append(right, i)
			}
		}
	}

	if len(span) > 0 {
		if d == 0 {
			for _, i := range span {
				for _, p := range ps {
					if err := f.emit(i, p); err != nil {
						return err
					}
				}
			}
		} else {
			// at position d-1, either range may hold the key
			if err := f.stream(span, ps, d-1); err != nil {
				return err
			}
			if err := f.stream(ps, span, d-1); err != nil {
				return err
			}
		}
	}

	if err := f.split(left, ps[:mid], d); err != nil {
		return err
	}
	return f.split(right, ps[mid:], d)
}

// emit records that ranges i and j overlap.
func (f *pairFinder) emit(i, j int) error {
	f.work++
	if err := f.b.spend(1); err != nil {
		return err
	}
	f.pairs = append(f.pairs, [2]int{min(i, j), max(i, j)})
	return nil
}

// prefix returns the first d bytes of r.
func (r Range) prefix(d int) Range {
	return Range{Low: r.Low[:d], High: r.High[:d]}
}

// rangesOverlap reports whether two ranges of the same length have codes in
// common.
func rangesOverlap(r, s Range) bool {
	for d := range r.Low {
		if r.High[d] < s.Low[d] || s.High[d] < r.Low[d] {
			return false
		}
	}
	return true
}

// rangeContains reports whether every code in r is also in q.
func rangeContains(q, r Range) bool {
	if len(q.Low) != len(r.Low) {
		return false
	}
	for d := range r.Low {
		if r.Low[d] < q.Low[d] || r.High[d] > q.High[d] {
			return false
		}
	}
	return true
}

// subtractRange appends to res the parts of p which are not in q, as at
// most 2·len(p.Low) disjoint ranges.
func subtractRange(res CodeSpaceRange, p, q Range) CodeSpaceRange {
	if len(p.Low) != len(q.Low) || !rangesOverlap(p, q) {
		return append(res, p)
	}
	lo, hi := slices.Clone(p.Low), slices.Clone(p.High)
	for d := range lo {
		if lo[d] < q.Low[d] {
			part := Range{Low: slices.Clone(lo), High: slices.Clone(hi)}
			part.High[d] = q.Low[d] - 1
			res = append(res, part)
			lo[d] = q.Low[d]
		}
		if hi[d] > q.High[d] {
			part := Range{Low: slices.Clone(lo), High: slices.Clone(hi)}
			part.Low[d] = q.High[d] + 1
			res = append(res, part)
			hi[d] = q.High[d]
		}
	}
	return res
}
