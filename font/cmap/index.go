// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"cmp"
	"container/heap"
	"encoding/binary"
	"errors"
	"iter"
	"math/bits"
	"runtime"
	"slices"
	"sync"
	"weak"

	"seehuhn.de/go/pdf/internal/limits"
)

// box is a rectangular set of character codes: a code is in the box if it
// has the same length as first and last, and every byte of the code lies
// between the corresponding bytes of first and last.  The box belongs to the
// mapping entry id.  When several boxes contain a code, the smallest id wins.
type box struct {
	first, last []byte
	id          int
}

// codeIndex finds the winning box for a character code.
//
// Boxes are stored in a tree with one level per byte of the code.  At each
// byte but the last, the byte interval of a box is split into the canonical
// nodes of a binary segment tree over the byte values 00 to FF: one node for
// a constant byte or for 00 to FF, and at most 14 nodes in general.  A box is
// stored as one piece for each combination of nodes of its leading bytes, so
// that a box of 2, 3 or 4 bytes gives at most 14, 196 or 2,744 pieces.  For
// each combination, the last-byte intervals of the pieces are merged into
// disjoint segments, which record the winning id.
//
// A lookup follows, for each leading byte of the code, those segment-tree
// nodes containing the byte which hold pieces, and bisects the segments at
// every combination reached.  If the leading bytes of all boxes are
// constant, this is a single map probe and a bisection.  In general, a lookup
// of an l-byte code takes at most 9^(l-1) probes and bisections.
//
// A box with constant leading bytes is stored as a single piece.  The
// pieces beyond one per box are limited to [limits.MaxCMapIndexExtraPieces],
// taking the boxes with the fewest pieces first, so that building the index
// takes time linear in the number of boxes plus a fixed bound.  Boxes which
// do not fit ("leftover" boxes) are checked one by one.  CMaps read from a file are rejected if an index
// along their usecmap chain has leftover boxes (see [chainIndex.tooComplex]),
// and writing such a CMap fails.
type codeIndex struct {
	// nodes maps the key of a tree node, see [nodeKey], to the node
	nodes map[string]indexNode

	// depths has bit d set if some box has a segment-tree node at depth d
	// for one of its leading bytes
	depths uint16

	leftover []box // sorted by id

	// empty is the winning id for the empty code, or -1
	empty int
}

// indexNode is a node of the tree in a [codeIndex].  Above the last byte,
// mask has bit d set if the node has children at depth d of the segment tree
// for the next byte.  At the last byte, segs holds the disjoint segments,
// sorted.
type indexNode struct {
	mask uint16
	segs []segment
}

// treeNode is a node of the segment tree over the byte values: the node at
// depth d with index k covers the byte values k<<(8-d) to ((k+1)<<(8-d))-1.
type treeNode struct {
	depth, index byte
}

// nodeKey returns the start of the key of a node of the tree in a
// [codeIndex], for codes of length l.  The key of a node below is extended
// by the depth and index of one [treeNode] for each leading byte.
func nodeKey(buf []byte, l int) []byte {
	return binary.AppendUvarint(buf, uint64(l))
}

// errTooComplex is the error for CMaps whose chain index is too complex,
// see [chainIndex.tooComplex].
var errTooComplex = errors.New("CMap too complex")

// segment records that codes whose last byte is in lo..hi map to id.
type segment struct {
	lo, hi byte
	id     int
}

// canonicalNodes appends to nodes the canonical nodes of the segment tree
// which together cover the byte values lo..hi.  These are at most 14 nodes.
func canonicalNodes(nodes []treeNode, lo, hi byte) []treeNode {
	for a := int(lo); a <= int(hi); {
		s := 8
		for a&(1<<s-1) != 0 || a+1<<s-1 > int(hi) {
			s--
		}
		nodes = append(nodes, treeNode{depth: byte(8 - s), index: byte(a >> s)})
		a += 1 << s
	}
	return nodes
}

// extraPieces returns the number of pieces beyond the first of a valid,
// non-empty box in a [codeIndex].  Results larger than
// [limits.MaxCMapIndexExtraPieces] are reported as
// limits.MaxCMapIndexExtraPieces+1.
func extraPieces(first, last []byte) int {
	var buf [14]treeNode
	n := 1
	for i := range len(first) - 1 {
		n *= len(canonicalNodes(buf[:0], first[i], last[i]))
		if n-1 > limits.MaxCMapIndexExtraPieces {
			return limits.MaxCMapIndexExtraPieces + 1
		}
	}
	return n - 1
}

// piecesFit reports whether an index for the given boxes, as built by
// [newCodeIndex], would have no leftover boxes, that is whether the pieces
// beyond one per box fit into [limits.MaxCMapIndexExtraPieces].  Boxes
// which are not stored as
// pieces, because they contain no codes or only the empty code, are
// ignored as in newCodeIndex.  This takes time linear in the number of
// boxes.
func piecesFit(boxes []box) bool {
	total := 0
	for _, bx := range boxes {
		if !boxIsValid(bx.first, bx.last) || len(bx.first) == 0 {
			continue
		}
		// both terms are at most MaxCMapIndexExtraPieces+1, so the sum
		// cannot overflow
		total += extraPieces(bx.first, bx.last)
		if total > limits.MaxCMapIndexExtraPieces {
			return false
		}
	}
	return true
}

// indexBuilder holds the state of [newCodeIndex].
type indexBuilder struct {
	x *codeIndex

	// the keys of the nodes at the last byte, and their pieces
	keyID  map[string]int
	keys   []string
	pieces []piece
}

// piece is the part of a box which belongs to the last-byte node keys[key].
type piece struct {
	key    int
	lo, hi byte
	id     int
}

// newCodeIndex builds an index for the given boxes.  Boxes which contain
// no codes are ignored.
func newCodeIndex(boxes []box) *codeIndex {
	x := &codeIndex{nodes: make(map[string]indexNode), empty: -1}
	b := &indexBuilder{x: x, keyID: make(map[string]int)}

	type candidate struct {
		box
		extraPieces int
	}
	var cands []candidate
	for _, bx := range boxes {
		if !boxIsValid(bx.first, bx.last) {
			continue
		}
		if len(bx.first) == 0 {
			if x.empty < 0 || bx.id < x.empty {
				x.empty = bx.id
			}
			continue
		}
		cands = append(cands, candidate{bx, extraPieces(bx.first, bx.last)})
	}

	// Insert the cheapest boxes first, so that as few boxes as possible
	// are left over.  A box is left over only if the remaining budget is
	// smaller than its extra pieces, so there are leftover boxes exactly
	// when the total of the extra pieces exceeds the budget, as
	// [piecesFit] reports.
	slices.SortStableFunc(cands, func(a, b candidate) int {
		return cmp.Compare(a.extraPieces, b.extraPieces)
	})
	budget := limits.MaxCMapIndexExtraPieces
	var keyBuf [16]byte
	for _, c := range cands {
		if c.extraPieces > budget {
			x.leftover = append(x.leftover, c.box)
			continue
		}
		budget -= c.extraPieces
		b.insert(nodeKey(keyBuf[:0], len(c.first)), c.box, 0)
	}
	slices.SortFunc(x.leftover, func(a, b box) int { return cmp.Compare(a.id, b.id) })

	slices.SortFunc(b.pieces, func(a, b piece) int {
		if c := cmp.Compare(a.key, b.key); c != 0 {
			return c
		}
		return cmp.Compare(a.lo, b.lo)
	})
	var group []interval
	for start := 0; start < len(b.pieces); {
		end := start + 1
		for end < len(b.pieces) && b.pieces[end].key == b.pieces[start].key {
			end++
		}
		group = group[:0]
		for _, p := range b.pieces[start:end] {
			group = append(group, interval{int(p.lo), int(p.hi), p.id})
		}
		x.nodes[b.keys[b.pieces[start].key]] = indexNode{segs: envelope(group)}
		start = end
	}

	return x
}

// insert stores the pieces of bx below the node with the given key, which
// is at byte pos of the code.
func (b *indexBuilder) insert(key []byte, bx box, pos int) {
	l := len(bx.first)
	if pos == l-1 {
		id, ok := b.keyID[string(key)]
		if !ok {
			k := string(key)
			id = len(b.keys)
			b.keyID[k] = id
			b.keys = append(b.keys, k)
		}
		b.pieces = append(b.pieces, piece{key: id, lo: bx.first[pos], hi: bx.last[pos], id: bx.id})
		return
	}

	var buf [14]treeNode
	for _, nd := range canonicalNodes(buf[:0], bx.first[pos], bx.last[pos]) {
		bit := uint16(1) << nd.depth
		if n := b.x.nodes[string(key)]; n.mask&bit == 0 {
			n.mask |= bit
			b.x.nodes[string(key)] = n
		}
		b.x.depths |= bit
		b.insert(append(key, nd.depth, nd.index), bx, pos+1)
	}
}

// interval is a byte interval lo..hi, belonging to the entry id.
type interval struct {
	lo, hi int
	id     int
}

// envelope returns, for intervals sorted by lo, the disjoint segments which
// give the smallest id covering each byte value.  Adjacent segments with the
// same id are merged.
func envelope(intervals []interval) []segment {
	var res []segment
	var active intervalHeap
	next := 0 // the next interval to activate
	x := 0
	for x < 256 {
		for next < len(intervals) && intervals[next].lo <= x {
			heap.Push(&active, intervals[next])
			next++
		}
		for len(active) > 0 && active[0].hi < x {
			heap.Pop(&active)
		}
		if len(active) == 0 {
			if next >= len(intervals) {
				break
			}
			x = intervals[next].lo
			continue
		}

		// the winner stays the same until it ends, or until a new
		// interval starts
		top := active[0]
		end := top.hi
		if next < len(intervals) && intervals[next].lo-1 < end {
			end = intervals[next].lo - 1
		}
		if n := len(res); n > 0 && res[n-1].id == top.id && int(res[n-1].hi)+1 == x {
			res[n-1].hi = byte(end)
		} else {
			res = append(res, segment{lo: byte(x), hi: byte(end), id: top.id})
		}
		x = end + 1
	}
	return slices.Clip(res)
}

// intervalHeap is a min-heap of intervals, ordered by id.
type intervalHeap []interval

func (h intervalHeap) Len() int           { return len(h) }
func (h intervalHeap) Less(i, j int) bool { return h[i].id < h[j].id }
func (h intervalHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *intervalHeap) Push(x any)        { *h = append(*h, x.(interval)) }
func (h *intervalHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// lookup returns the smallest id of all boxes which contain code, or -1 if
// no box contains code.
func (x *codeIndex) lookup(code []byte) int {
	l := len(code)
	if l == 0 {
		return x.empty
	}

	var buf [16]byte
	key := nodeKey(buf[:0], l)
	var best int
	if x.depths&^(1<<8) == 0 {
		// all leading bytes are constant: the path is given by the code
		for _, c := range code[:l-1] {
			key = append(key, 8, c)
		}
		best = x.lookupLast(key, code[l-1])
	} else {
		best = x.search(key, code, 0)
	}

	for _, w := range x.leftover {
		if best >= 0 && w.id > best {
			break
		}
		if boxContains(w.first, w.last, code) {
			best = w.id
			break
		}
	}
	return best
}

// search returns the smallest id below the node with the given key, which
// is at byte pos of the code, among the boxes which contain code, or -1.
func (x *codeIndex) search(key, code []byte, pos int) int {
	if pos == len(code)-1 {
		return x.lookupLast(key, code[pos])
	}
	n, ok := x.nodes[string(key)]
	if !ok {
		return -1
	}
	best := -1
	c := code[pos]
	for mask := n.mask; mask != 0; mask &= mask - 1 {
		d := bits.TrailingZeros16(mask)
		id := x.search(append(key, byte(d), c>>(8-d)), code, pos+1)
		if id >= 0 && (best < 0 || id < best) {
			best = id
		}
	}
	return best
}

// lookupLast returns the id of the segment containing the last byte c in the
// node with the given key, or -1.
func (x *codeIndex) lookupLast(key []byte, c byte) int {
	segs := x.nodes[string(key)].segs
	i, _ := slices.BinarySearchFunc(segs, c, func(s segment, c byte) int {
		return cmp.Compare(s.hi, c)
	})
	if i < len(segs) && segs[i].lo <= c {
		return segs[i].id
	}
	return -1
}

// boxIsValid reports whether first and last describe a non-empty box.
// Unlike [rangeIsValid], this allows the box of the empty code.
func boxIsValid(first, last []byte) bool {
	if len(first) != len(last) {
		return false
	}
	for i := range first {
		if first[i] > last[i] {
			return false
		}
	}
	return true
}

// boxContains reports whether code lies in the box first..last.
func boxContains(first, last, code []byte) bool {
	if len(first) != len(code) || len(last) != len(code) {
		return false
	}
	for i, b := range code {
		if b < first[i] || b > last[i] {
			return false
		}
	}
	return true
}

// indexedBoxes appends to boxes the parts of the range first..last whose
// codes have an index, as computed by [rangeIndex], of at most maxIndex.
// Codes with larger index are treated as unmapped.  The argument maxIndex
// must lie between 0 and math.MaxInt32.  The result consists of at most
// len(first)+1 boxes.
func indexedBoxes(boxes []box, first, last []byte, maxIndex, id int) []box {
	if !boxIsValid(first, last) {
		return boxes
	}

	// mixed-radix digits of maxIndex, most significant digit first
	l := len(first)
	digits := make([]int, l)
	rest := int64(maxIndex)
	for i := l - 1; i >= 0; i-- {
		span := int64(last[i]) - int64(first[i]) + 1
		digits[i] = int(rest % span)
		rest /= span
	}
	if rest > 0 {
		// all indices are at most maxIndex
		return append(boxes, box{first, last, id})
	}

	// The codes with index at most maxIndex are those which agree with
	// the limit in the first j positions and are smaller at position j, for
	// some j, together with the limit itself.
	for j := range l {
		if digits[j] > 0 {
			lo := slices.Clone(first)
			hi := slices.Clone(last)
			for i := range j {
				lo[i] = first[i] + byte(digits[i])
				hi[i] = lo[i]
			}
			hi[j] = first[j] + byte(digits[j]-1)
			boxes = append(boxes, box{lo, hi, id})
		}
	}
	limit := slices.Clone(first)
	for i := range l {
		limit[i] = first[i] + byte(digits[i])
	}
	return append(boxes, box{limit, limit, id})
}

// levelIndex is the index of the mappings of a single CMap in a usecmap
// chain, without those of the CMaps it inherits from.  Entries are numbered
// in order of precedence, so that a smaller id takes precedence over a
// larger one.
type levelIndex interface {
	// numEntries returns the number of mapping entries.
	numEntries() int

	// entryCodes iterates over the codes of entry id, together with their
	// position in the entry.
	entryCodes(id int) iter.Seq2[int, []byte]

	// lookup returns the id of the entry which maps code, or -1.
	lookup(code []byte) int

	// hasLeftover reports whether one of the indices of the level has
	// leftover boxes, see [codeIndex].
	hasLeftover() bool
}

// chainIndex is the index of a CMap together with the CMaps it inherits
// from.  Each CMap in the chain has its own [levelIndex], and the chain
// indices of a child and its parent share the parent's part, so that the
// indices of all CMaps in a chain take space linear in the size of the
// chain.
type chainIndex[V levelIndex] struct {
	level  V
	parent *chainIndex[V]

	// depth is the number of CMaps in the chain, see [File.Chain].
	// If the Parent links form a cycle, the parent links of the indices
	// form the same cycle, and depth stops the walk after each CMap has
	// been visited once.
	depth int

	// leftover reports whether an index along the chain has leftover
	// boxes, see [codeIndex]
	leftover bool
}

// tooComplex reports whether an index along the chain x has leftover boxes,
// which lookups check one by one, see [codeIndex].
func (x *chainIndex[V]) tooComplex() bool {
	return x.leftover
}

// lookup returns the level of the chain which maps code, together with the
// id of the winning entry within that level.  The id is -1 if no level maps
// code.  Earlier levels take precedence over later ones.
func (x *chainIndex[V]) lookup(code []byte) (V, int) {
	y := x
	for range x.depth {
		if id := y.level.lookup(code); id >= 0 {
			return y.level, id
		}
		y = y.parent
	}
	var zero V
	return zero, -1
}

// mapped calls yield for every code which the chain maps, with the level and
// id of the winning entry and the position of the code in that entry.  Each
// code is reported once, entries with higher precedence first.  The
// enumeration stops when yield returns false, or after
// [limits.MaxCMapMappings] codes have been considered.
//
// The codes of all but the last level are recorded, so that codes which an
// earlier level maps can be skipped in constant time.  This takes memory
// proportional to the number of codes considered.
func (x *chainIndex[V]) mapped(yield func(level V, id, pos int, code []byte) bool) {
	budget := limits.MaxCMapMappings
	var seen map[string]struct{}
	y := x
	for k := range x.depth {
		last := k == x.depth-1
		for id := range y.level.numEntries() {
			for pos, code := range y.level.entryCodes(id) {
				if budget <= 0 {
					return
				}
				budget--
				if y.level.lookup(code) != id {
					continue
				}
				if _, ok := seen[string(code)]; ok {
					continue
				}
				if !last {
					if seen == nil {
						seen = make(map[string]struct{})
					}
					seen[string(code)] = struct{}{}
				}
				if !yield(y.level, id, pos, code) {
					return
				}
			}
		}
		y = y.parent
	}
}

// walkChain returns p and its ancestors, as given by parent, starting with
// p.  If the parent links form a cycle, each element is listed once.
// For a nil p, the result is nil.
func walkChain[T any](p *T, parent func(*T) *T) []*T {
	var res []*T
	seen := make(map[*T]bool)
	for g := p; g != nil && !seen[g]; g = parent(g) {
		seen[g] = true
		res = append(res, g)
	}
	return res
}

// errChainTooDeep is the error for usecmap chains with more than
// [limits.MaxCMapChainDepth] CMaps.
var errChainTooDeep = errors.New("usecmap chain too deep")

// errChainCycle is the error for writing a CMap whose Parent links form a
// cycle.
var errChainCycle = errors.New("usecmap chain forms a cycle")

// parentTooDeep reports whether a CMap with the given parent chain would have
// a chain with more than [limits.MaxCMapChainDepth] CMaps.  The argument is
// nil for a CMap without parent.
func parentTooDeep[V levelIndex](parent *chainIndex[V]) bool {
	return parent != nil && parent.depth >= limits.MaxCMapChainDepth
}

// checkWritable returns an error if the chain starting at p cannot be
// written in a form which reads back: if the parent links form a cycle,
// if the chain has more than [limits.MaxCMapChainDepth] CMaps, or if its
// index would be too complex, see [chainIndex.tooComplex].  The check takes
// time linear in the number of entries and caches nothing, so that the chain
// may still be modified afterwards.
func checkWritable[T any](p *T, parent func(*T) *T, fits func(*T) bool) error {
	chain := walkChain(p, parent)
	if len(chain) > 0 && parent(chain[len(chain)-1]) != nil {
		return errChainCycle
	}
	if len(chain) > limits.MaxCMapChainDepth {
		return errChainTooDeep
	}
	for _, g := range chain {
		if !fits(g) {
			return errTooComplex
		}
	}
	return nil
}

// cachedChainIndex returns the chain index for p, building the indices of
// p and of its ancestors, as given by parent, on first use.  Only the levels
// which have no cached index yet are built.  Indices are held in cache,
// keyed by a weak pointer to the CMap, and are dropped once the CMap is
// garbage collected.  The argument p must not be nil.
func cachedChainIndex[T any, V levelIndex](cache *sync.Map, p *T, parent func(*T) *T, build func(*T) V) *chainIndex[V] {
	if v, ok := cache.Load(weak.Make(p)); ok {
		return v.(*chainIndex[V])
	}

	// the levels without a cached index, starting with p
	var todo []*T
	pos := make(map[*T]int)
	var tail *chainIndex[V] // the index of the first cached ancestor
	cycleStart := -1
	for g := p; g != nil; g = parent(g) {
		if v, ok := cache.Load(weak.Make(g)); ok {
			tail = v.(*chainIndex[V])
			break
		}
		if i, ok := pos[g]; ok {
			cycleStart = i
			break
		}
		pos[g] = len(todo)
		todo = append(todo, g)
	}

	nodes := make([]*chainIndex[V], len(todo))
	for i, g := range todo {
		nodes[i] = &chainIndex[V]{level: build(g)}
	}
	end := len(todo)
	if cycleStart >= 0 {
		// The members of the cycle all have the whole cycle as their chain.
		leftover := false
		for _, x := range nodes[cycleStart:] {
			leftover = leftover || x.level.hasLeftover()
		}
		for i := cycleStart; i < len(todo); i++ {
			x := nodes[i]
			x.parent = nodes[cycleStart+(i+1-cycleStart)%(len(todo)-cycleStart)]
			x.depth = len(todo) - cycleStart
			x.leftover = leftover
		}
		end = cycleStart
	}
	for i := end - 1; i >= 0; i-- {
		x := nodes[i]
		if i+1 < len(todo) {
			x.parent = nodes[i+1]
		} else {
			x.parent = tail
		}
		x.leftover = x.level.hasLeftover()
		if x.parent != nil {
			x.depth = x.parent.depth
			x.leftover = x.leftover || x.parent.leftover
		}
		x.depth++
	}

	res := nodes[0]
	for i, g := range todo {
		key := weak.Make(g)
		v, loaded := cache.LoadOrStore(key, nodes[i])
		if !loaded {
			runtime.AddCleanup(g, func(key weak.Pointer[T]) { cache.Delete(key) }, key)
		}
		if i == 0 {
			res = v.(*chainIndex[V])
		}
	}
	return res
}
