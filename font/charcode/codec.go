// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package charcode

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/bits"
	"slices"
)

// Code represents a character code within a [Codec].
// The code is a uint32 value, where the bytes of the code are stored in
// little-endian order.  The number of bytes in the code is determined by
// the corresponding [Codec].
type Code uint32

// MaxCodeLength is the largest length, in bytes, of a character code
// supported by this library.
const MaxCodeLength = 4

// A Codec efficiently converts between sequences of bytes and uint32 character codes.
type Codec struct {
	// bound and child form a linearised lookup tree, one node per index.
	// The root group of nodes starts at index 0.
	//
	// The current input byte is compared to bound[i].  If the input byte is
	// less than or equal to bound[i], the input byte is consumed and child[i]
	// determines how to continue:
	//
	//   - If child[i] equals validLeaf, the consumed byte is the last byte
	//     of a valid character code and the lookup stops.
	//   - If child[i] equals invalidConsume(k), for k∈{0,1,2,3}, the input
	//     starts with an invalid character code.  The lookup stops and
	//     an additional k bytes must be consumed to continue.
	//   - Otherwise, child[i] is the index of the first node of the group
	//     which processes the next input byte.
	//
	// If the input byte is greater than bound[i], the lookup continues with
	// node i+1.  The last node of every group has bound 0xFF.
	//
	// Indices are stored as int, so that they cannot overflow for any slice
	// which can be allocated.
	bound []byte
	child []int

	// ranges is the normalized code space range, see [normalize].
	ranges CodeSpaceRange
}

// These constants represent special values of the child field in a [Codec].
// Real child indices are always positive, since the root group starts at
// index 0 and every other group comes after it.
const (
	validLeaf       = 0
	invalidConsume0 = -1
	invalidConsume1 = -2
	invalidConsume2 = -3
	invalidConsume3 = -4
)

// invalidConsume returns the child value for an invalid code where k more
// bytes must be consumed.
func invalidConsume(k int) int {
	return -1 - k
}

// NewCodec returns a new Codec for the given code space range.
//
// An error is returned if the code space range is invalid, that is if one of
// its ranges is invalid or if a code in one range is a proper prefix of a code
// in another range.  If the lookup tree for the code space range would be
// too large, the error is [ErrTooComplex].
func NewCodec(ranges CodeSpaceRange) (*Codec, error) {
	for _, r := range ranges {
		if !r.IsValid() {
			return nil, errInvalidCodeSpaceRange
		}
	}
	ranges = normalize(ranges)

	b := newBuilder(ranges, treeBudget(len(ranges)))
	root, err := b.build()
	if err != nil {
		return nil, err
	}
	c := b.linearize(root)
	c.ranges = ranges
	return c, nil
}

// treeBudget returns the number of construction steps allowed for the lookup
// tree of a code space range with n ranges.  The trees of all predefined
// CMaps need at most 156 steps in total.  Some code space ranges need trees
// which grow with a high power of n; for these, construction fails with
// [ErrTooComplex].
func treeBudget(n int) int {
	return 256*n + 4096
}

// Decode decodes the first character code of an input byte sequence.
// The method returns the character code, the number of bytes consumed,
// and whether the character code is valid.
//
// The returned code contains the consumed bytes, with the first byte
// in the least significant position.
//
// At the end of input, (0, 0, false) is returned.
//
// The return value consumed is always less than or equal to the length of s.
// If the length of s is non-zero, consumed is greater than zero.
func (c *Codec) Decode(s []byte) (code Code, consumed int, valid bool) {
	bound, child := c.bound, c.child
	cur := 0
	for {
		if len(s) == 0 {
			return
		}
		var b byte
		b, s = s[0], s[1:]

		// Since codes are at most 4 bytes long, consumed can only be 0, 1, 2,
		// or 3 at this point.
		code |= Code(b) << (8 * consumed)
		consumed++

		// Use a linear scan to find the correct child node.  If performance
		// ever becomes an issue, the code could be restructured to use a
		// binary tree here.
		for b > bound[cur] {
			cur++
		}

		next := child[cur]
		switch next {
		case validLeaf:
			valid = true
			return
		case invalidConsume3:
			if len(s) > 0 {
				code |= Code(s[0]) << (8 * consumed)
				s = s[1:]
				consumed++
			}
			fallthrough
		case invalidConsume2:
			if len(s) > 0 {
				code |= Code(s[0]) << (8 * consumed)
				s = s[1:]
				consumed++
			}
			fallthrough
		case invalidConsume1:
			if len(s) > 0 {
				code |= Code(s[0]) << (8 * consumed)
				consumed++
			}
			fallthrough
		case invalidConsume0:
			return
		default:
			cur = next
		}
	}
}

// AppendCode converts a code to bytes and appends the bytes to the
// given slice.
func (c *Codec) AppendCode(s []byte, code Code) []byte {
	// The child links form an acyclic graph in which every path has at most
	// four links, one per code byte.  Thus, the loop will always terminate.

	cur := 0 // root
	for {
		b := byte(code)
		s = append(s, b)
		code >>= 8

		for b > c.bound[cur] {
			cur++
		}

		next := c.child[cur]
		switch next {
		case validLeaf:
			return s
		case invalidConsume3:
			s = append(s, byte(code))
			code >>= 8
			fallthrough
		case invalidConsume2:
			s = append(s, byte(code))
			code >>= 8
			fallthrough
		case invalidConsume1:
			s = append(s, byte(code))
			fallthrough
		case invalidConsume0:
			return s
		default:
			cur = next
		}
	}
}

// CodeSpaceRange returns the code space range used to create the Codec.
// The ranges are sorted by length, Low and High, and duplicates are removed.
// Two codecs return equal code space ranges if and only if they were made
// from the same ranges, up to order and repetition.
func (c *Codec) CodeSpaceRange() CodeSpaceRange {
	return deepCopy(c.ranges)
}

// Equal reports whether c and other were made from the same code space
// ranges, up to order and repetition.  This agrees with comparing the
// results of [Codec.CodeSpaceRange], without copying them.
func (c *Codec) Equal(other *Codec) bool {
	return slices.EqualFunc(c.ranges, other.ranges, func(r, s Range) bool {
		return compareRange(r, s) == 0
	})
}

// deepCopy returns a copy of ranges which shares no storage with ranges.
func deepCopy(ranges CodeSpaceRange) CodeSpaceRange {
	total := 0
	for _, r := range ranges {
		total += len(r.Low) + len(r.High)
	}
	buf := make([]byte, 0, total)
	res := make(CodeSpaceRange, len(ranges))
	for i, r := range ranges {
		k := len(buf)
		buf = append(buf, r.Low...)
		buf = append(buf, r.High...)
		res[i] = Range{Low: buf[k : k+len(r.Low) : k+len(r.Low)], High: buf[k+len(r.Low) : len(buf) : len(buf)]}
	}
	return res
}

// normalize returns a sorted copy of ranges, with duplicates removed.
func normalize(ranges CodeSpaceRange) CodeSpaceRange {
	res := deepCopy(ranges)
	slices.SortFunc(res, compareRange)
	return slices.CompactFunc(res, func(r, s Range) bool {
		return compareRange(r, s) == 0
	})
}

// compareRange orders ranges by length, then by Low, then by High.
func compareRange(r, s Range) int {
	return compareSuffix(r, s, 0)
}

// builder constructs the lookup tree of a [Codec] as a DAG of node groups.
//
// The group which processes the input byte at position depth depends only on
// the set of ranges compatible with the bytes consumed so far, and only
// through the suffixes Low[depth:], High[depth:] of these ranges.  The
// builder memoises groups on this set of suffixes, so that every distinct
// suffix set is expanded only once, and it interns groups on their contents,
// so that every distinct group is stored only once.
type builder struct {
	ranges CodeSpaceRange

	// rank[d][i] identifies the suffix of ranges[i] from position d:
	// two ranges have the same rank at depth d if and only if their
	// suffixes are equal.  Ranks follow the lexicographic order of the
	// suffixes.  rank[d][i] is -1 if ranges[i] is not longer than d bytes.
	rank [4][]int32

	// bySuffix maps a canonical suffix set (see suffixKey) to a group id.
	bySuffix map[string]int

	// byContent maps the contents of a group (see contentKey) to a group id.
	byContent map[string]int

	// groups holds the distinct groups; group id g is stored at groups[g-1].
	groups [][]entry

	// steps counts the elementary operations of the construction.
	// The construction is abandoned once steps exceeds budget.  Memory use
	// is proportional to steps.
	steps, budget int
}

// entry is a node of a group, before linearization.  The child field holds
// one of the special child values, or a group id (always positive).
type entry struct {
	high  byte
	child int
}

func newBuilder(ranges CodeSpaceRange, budget int) *builder {
	b := &builder{
		ranges:    ranges,
		budget:    budget,
		bySuffix:  make(map[string]int),
		byContent: make(map[string]int),
	}

	idx := make([]int, 0, len(ranges))
	for d := range b.rank {
		rank := make([]int32, len(ranges))
		idx = idx[:0]
		for i, r := range ranges {
			rank[i] = -1
			if len(r.Low) > d {
				idx = append(idx, i)
			}
		}
		slices.SortFunc(idx, func(i, j int) int {
			return compareSuffix(ranges[i], ranges[j], d)
		})
		var next int32
		for k, i := range idx {
			if k > 0 && compareSuffix(ranges[idx[k-1]], ranges[i], d) != 0 {
				next++
			}
			rank[i] = next
		}
		b.rank[d] = rank
	}

	return b
}

// spend adds k to the step count, and returns [ErrTooComplex] if the budget
// is exhausted.
func (b *builder) spend(k int) error {
	b.steps += k
	if b.steps > b.budget {
		return ErrTooComplex
	}
	return nil
}

// sortCost returns the number of steps charged for sorting m elements.
func sortCost(m int) int {
	return m * bits.Len(uint(m))
}

// compareSuffix compares the suffixes of r and s from position d.
// Shorter suffixes come first.
func compareSuffix(r, s Range, d int) int {
	if len(r.Low) != len(s.Low) {
		return len(r.Low) - len(s.Low)
	}
	if c := bytes.Compare(r.Low[d:], s.Low[d:]); c != 0 {
		return c
	}
	return bytes.Compare(r.High[d:], s.High[d:])
}

// build constructs the lookup DAG and returns the id of the root group.
func (b *builder) build() (int, error) {
	if err := b.spend(len(b.ranges)); err != nil {
		return 0, err
	}
	all := make([]int, len(b.ranges))
	for i := range all {
		all[i] = i
	}
	slices.SortStableFunc(all, func(i, j int) int {
		return int(b.rank[0][i] - b.rank[0][j])
	})
	all = slices.CompactFunc(all, func(i, j int) bool {
		return b.rank[0][i] == b.rank[0][j]
	})
	return b.group(all, 0)
}

// group returns the id of the group which processes the input byte at
// position depth, given the set of ranges compatible with the bytes before.
//
// The list set contains each range suffix at most once (as given by the
// ranks at this depth), sorted by rank.  All ranges in the set are longer
// than depth.
func (b *builder) group(set []int, depth int) (int, error) {
	if err := b.spend(len(set) + 1); err != nil { // building the key
		return 0, err
	}
	key := b.suffixKey(set, depth)
	if id, ok := b.bySuffix[key]; ok {
		return id, nil
	}
	if err := b.spend(sortCost(2*len(set) + 2)); err != nil { // sorting the breaks
		return 0, err
	}

	// The breaks divide the byte values into intervals, such that every
	// range in the set either covers an interval completely, or not at all.
	breaks := make([]int, 0, 2*len(set)+2)
	breaks = append(breaks, 0, 256)
	minLen := depth + 1 // only used if the set is empty
	for k, i := range set {
		r := b.ranges[i]
		breaks = append(breaks, int(r.Low[depth]), int(r.High[depth])+1)
		if k == 0 || len(r.Low) < minLen {
			minLen = len(r.Low)
		}
	}
	slices.Sort(breaks)
	breaks = slices.Compact(breaks)

	// Visit the ranges in order of their suffix rank at the next depth, so
	// that the child sets come out in canonical order.  Ranges which end at
	// this depth (rank -1) come first.
	var next []int32
	var children []int
	if depth+1 < len(b.rank) {
		next = b.rank[depth+1]
		children = slices.Clone(set)
		slices.SortStableFunc(children, func(i, j int) int {
			return int(next[i] - next[j])
		})
	} else {
		children = set
	}

	// List the ranges which cover each interval, in the order of children.
	// The ranges for interval j are overlap[start[j]:start[j+1]].
	var idx [257]int
	for j, x := range breaks {
		idx[x] = j
	}
	numIntervals := len(breaks) - 1
	start := make([]int, numIntervals+1)
	for _, i := range children {
		r := b.ranges[i]
		start[idx[r.Low[depth]]]++
		start[idx[int(r.High[depth])+1]]--
	}
	total, depthCount := 0, 0
	for j := range numIntervals {
		depthCount += start[j]
		start[j] = total
		total += depthCount
	}
	start[numIntervals] = total
	if err := b.spend(total); err != nil {
		return 0, err
	}
	overlap := make([]int, total)
	fill := slices.Clone(start[:numIntervals])
	for _, i := range children {
		r := b.ranges[i]
		for j := idx[r.Low[depth]]; j < idx[int(r.High[depth])+1]; j++ {
			overlap[fill[j]] = i
			fill[j]++
		}
	}

	entries := make([]entry, 0, numIntervals)
	var childSet []int
	for j := range numIntervals {
		childSet = childSet[:0]
		numLeaves := 0
		for _, i := range overlap[start[j]:start[j+1]] {
			r := b.ranges[i]
			if len(r.Low) == depth+1 {
				numLeaves++
				continue
			}
			if n := len(childSet); n > 0 && next[childSet[n-1]] == next[i] {
				continue // same suffix as the previous range
			}
			childSet = append(childSet, i)
		}

		var child int
		switch {
		case numLeaves == 0 && len(childSet) == 0:
			// There are no ranges which overlap [low, high],
			// and thus byte values in [low, high] are invalid here.

			// Section 9.7.6.3 (Handling undefined characters) of ISO
			// 32000-2:2020 specifies the following: In case of an invalid
			// code, the number of input bytes consumed is the length of the
			// shortest code which contains the longest possible prefix of the
			// given code.
			child = invalidConsume(minLen - (depth + 1))
		case len(childSet) == 0:
			// All ranges end with this byte, so this is a leaf.
			child = validLeaf
		case numLeaves == 0:
			// All ranges have more bytes, so we use a sub-tree to
			// decide which range to use, based on the following bytes.
			var err error
			child, err = b.group(childSet, depth+1)
			if err != nil {
				return 0, err
			}
		default:
			// Some ranges end with this byte, while other ranges allow for
			// more bytes.  This is an error, since no code may be a prefix of
			// another code.
			return 0, errInvalidCodeSpaceRange
		}
		entries = append(entries, entry{high: byte(breaks[j+1] - 1), child: child})
	}

	content := contentKey(entries)
	id, ok := b.byContent[content]
	if !ok {
		b.groups = append(b.groups, entries)
		id = len(b.groups)
		b.byContent[content] = id
	}
	b.bySuffix[key] = id
	return id, nil
}

// suffixKey returns a canonical representation of a suffix set.
func (b *builder) suffixKey(set []int, depth int) string {
	rank := b.rank[depth]
	buf := make([]byte, 1+4*len(set))
	buf[0] = byte(depth) // ranks are only comparable within one depth
	for k, i := range set {
		binary.LittleEndian.PutUint32(buf[1+4*k:], uint32(rank[i]))
	}
	return string(buf)
}

// contentKey returns a representation of the contents of a group.
// Two groups have the same key if and only if they describe the same
// lookup sub-tree, since child groups are interned before their parents.
func contentKey(entries []entry) string {
	buf := make([]byte, 0, 9*len(entries))
	for _, e := range entries {
		buf = append(buf, e.high)
		buf = binary.AppendVarint(buf, int64(e.child))
	}
	return string(buf)
}

// linearize converts the DAG with the given root group into a [Codec].
//
// Groups are laid out in depth-first order, every group after the group
// which first refers to it.  A group shared by several parents is placed
// once, so a child link can point to an earlier node, but the links
// remain acyclic.
func (b *builder) linearize(root int) *Codec {
	c := &Codec{}
	pos := make([]int, len(b.groups))
	for g := range pos {
		pos[g] = -1 // not yet placed
	}

	var place func(g int) int
	place = func(g int) int {
		entries := b.groups[g-1]
		base := len(c.bound)
		for _, e := range entries {
			c.bound = append(c.bound, e.high)
			c.child = append(c.child, validLeaf)
		}
		for k, e := range entries {
			child := e.child
			if child > 0 {
				if pos[child-1] < 0 {
					pos[child-1] = place(child)
				}
				child = pos[child-1]
			}
			c.child[base+k] = child
		}
		return base
	}
	pos[root-1] = place(root)

	return c
}

// SimpleCodec is the codec for the code space range [Simple], which
// consists of all one-byte codes.
var SimpleCodec = must(NewCodec(Simple))

func must(codec *Codec, err error) *Codec {
	if err != nil {
		panic(err)
	}
	return codec
}

// ErrTooComplex is returned by [NewCodec] if the lookup tree for a code
// space range would be too large.  The limit is proportional to the number
// of ranges.
var ErrTooComplex = errors.New("code space range too complex")

var errInvalidCodeSpaceRange = errors.New("invalid code space range")
