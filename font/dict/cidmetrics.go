// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict

import (
	"errors"
	"iter"
	"maps"
	"slices"
	"sort"

	"seehuhn.de/go/postscript/cid"
)

// MaxCID is the largest CID allowed in a PDF file.
const MaxCID = 65535

// CIDRun gives metrics for the CIDs First, ..., Last.
//
// If Values is nil, every CID in the run has metrics Value (range form).
// Otherwise CID First+i has metrics Values[i], and len(Values) must equal
// Last-First+1 (list form).  Value is not used in list form.
type CIDRun[T any] struct {
	First, Last cid.CID
	Value       T
	Values      []T
}

// CIDMetrics holds per-CID metrics as a list of runs.  The runs must be
// sorted by First and pairwise disjoint, and every run must satisfy
// First <= Last <= [MaxCID].  Encoding a font dictionary whose CIDMetrics
// violates these rules returns an error.
//
// Values read from a file may share backing arrays with other font
// dictionaries.  They must be copied before they are modified.
type CIDMetrics[T any] []CIDRun[T]

// Get returns the metrics for CID c.  The second return value is false if
// there are no metrics for c.
func (m CIDMetrics[T]) Get(c cid.CID) (T, bool) {
	i := sort.Search(len(m), func(i int) bool { return m[i].Last >= c })
	if i < len(m) && m[i].First <= c {
		r := &m[i]
		if r.Values != nil {
			return r.Values[c-r.First], true
		}
		return r.Value, true
	}
	var zero T
	return zero, false
}

// All returns an iterator over all CIDs which have metrics, in increasing
// order of CID.
func (m CIDMetrics[T]) All() iter.Seq2[cid.CID, T] {
	return func(yield func(cid.CID, T) bool) {
		for i := range m {
			r := &m[i]
			for c := r.First; c <= r.Last; c++ {
				v := r.Value
				if r.Values != nil {
					v = r.Values[c-r.First]
				}
				if !yield(c, v) {
					return
				}
				if c == r.Last { // avoid overflow
					break
				}
			}
		}
	}
}

// validate checks the invariants documented on CIDMetrics.
func (m CIDMetrics[T]) validate() error {
	for i := range m {
		r := &m[i]
		if r.Last > MaxCID {
			return errors.New("CID out of range")
		}
		if r.Last < r.First {
			return errors.New("invalid CID run")
		}
		if r.Values != nil && len(r.Values) != int(r.Last-r.First)+1 {
			return errors.New("wrong number of values in CID run")
		}
		if i > 0 && r.First <= m[i-1].Last {
			return errors.New("CID runs overlap or are out of order")
		}
	}
	return nil
}

// CIDMetricsFromMap converts a map to a compact list of runs.  Stretches of
// at least three consecutive CIDs with equal metrics become range runs, and
// all other CIDs are collected in list runs.  The result is nil if m is
// empty.
func CIDMetricsFromMap[T comparable](m map[cid.CID]T) CIDMetrics[T] {
	if len(m) == 0 {
		return nil
	}
	keys := slices.Sorted(maps.Keys(m))

	var res CIDMetrics[T]
	var list *CIDRun[T] // open list run, if any
	flush := func() {
		if list != nil {
			res = append(res, *list)
			list = nil
		}
	}

	for i := 0; i < len(keys); {
		c := keys[i]
		v := m[c]

		// extent of the stretch of consecutive CIDs with value v
		j := i + 1
		for j < len(keys) && keys[j] == keys[j-1]+1 && m[keys[j]] == v {
			j++
		}

		if j-i >= 3 {
			flush()
			res = append(res, CIDRun[T]{First: c, Last: keys[j-1], Value: v})
			i = j
			continue
		}

		if list != nil && list.Last+1 != c {
			flush()
		}
		if list == nil {
			list = &CIDRun[T]{First: c, Last: c - 1}
		}
		for ; i < j; i++ {
			list.Values = append(list.Values, v)
			list.Last = keys[i]
		}
	}
	flush()
	return res
}
