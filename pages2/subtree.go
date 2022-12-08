// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package pages2

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
)

type dictInfo struct {
	dict pdf.Dict // a \Page or \Pages object
	ref  *pdf.Reference
}

type nodeInfo struct {
	*dictInfo
	count pdf.Integer
	depth int // upper bound
}

// AppendPage adds a new page to the page tree.
func (t *Tree) AppendPage(pageDict pdf.Dict) (*pdf.Reference, error) {
	if t.isClosed {
		return nil, errors.New("page tree is closed")
	}

	ref := t.w.Alloc()
	node := &nodeInfo{
		dictInfo: &dictInfo{
			dict: pageDict,
			ref:  ref,
		},
		count: 1,
		depth: 0,
	}
	t.tail = append(t.tail, node)

	for {
		n := len(t.tail)
		if n < maxDegree || t.tail[n-1].depth < t.tail[n-maxDegree].depth {
			break
		}
		if n > maxDegree && t.tail[n-maxDegree].depth+1 != t.tail[n-maxDegree-1].depth {
			panic("missed a collapse") // TODO(voss): remove
		}
		tail, err := t.makeInternalNode(t.tail, n-maxDegree, n)
		if err != nil {
			return nil, err
		}
		t.tail = tail
	}

	return ref, nil
}

func (t *Tree) merge(a, b []*nodeInfo) ([]*nodeInfo, error) {
	var err error

	if len(a) == 0 {
		return b, nil
	}
	if len(b) == 0 {
		return a, nil
	}

	nextDepth := b[0].depth
	for len(a) > 1 && a[len(a)-1].depth < nextDepth {
		start := len(a) - maxDegree
		if start < 0 {
			start = 0
		}
		for start > 0 && a[start-1].depth == a[start].depth {
			start++
		}
		a, err = t.makeInternalNode(a, start, len(a))
		if err != nil {
			return nil, err
		}
	}
	if len(a) == 1 && a[0].depth < nextDepth {
		a[0].depth = nextDepth
	}
	prevDepth := a[len(a)-1].depth

	pos := len(a)
	a = append(a, b...)

	// Now the nodes are in order of decreasing depth, but there may
	// still be too many consecutive nodes with the same depth:
	// - `a` could end with up to maxDegree nodes of depth prevDepth (>= nextDepth)
	// - `b` could start with maxDegree-1 nodes of depth nextDepth

	start := pos
	for start > 0 && a[start-1].depth == prevDepth {
		start--
	}
	end := pos + 1
	for end < len(a) && a[end].depth == nextDepth {
		end++
	}

	for depth := nextDepth; ; depth++ {
		changed := false
		for end >= start+maxDegree {
			a, err = t.makeInternalNode(a, start, start+maxDegree)
			if err != nil {
				return nil, err
			}
			start++
			end -= maxDegree - 1
			changed = true
		}

		if depth >= prevDepth && !changed || start == 0 {
			break
		}

		end = start
		for start > 0 && a[start-1].depth == depth+1 {
			start--
		}
	}

	return a, nil
}

// makeInternalNode collapses nodes a, ..., b-1 into a new \Pages object.
func (t *Tree) makeInternalNode(nodes []*nodeInfo, a, b int) ([]*nodeInfo, error) {
	if a < 0 || b > len(nodes) || b-a < 2 {
		return nil, fmt.Errorf("invalid subtree node range %d, %d", a, b)
	}
	if a == b {
		return nodes, nil
	}

	childNodes := nodes[a:b]

	childRefs := make([]*pdf.Reference, len(childNodes))
	childDicts := make([]pdf.Object, len(childNodes))
	parentRef := t.w.Alloc()
	var total pdf.Integer
	depth := 0
	for i, node := range childNodes {
		node.dict["Parent"] = parentRef
		childDicts[i] = node.dict
		childRefs[i] = node.ref

		total += node.count
		if node.depth > depth {
			depth = node.depth
		}
	}
	_, err := t.w.WriteCompressed(childRefs, childDicts...)
	if err != nil {
		return nil, err
	}

	kids := make(pdf.Array, len(childRefs))
	for i, ref := range childRefs {
		kids[i] = ref
	}
	parentNode := &nodeInfo{
		dictInfo: &dictInfo{
			dict: pdf.Dict{
				"Type":  pdf.Name("Pages"),
				"Kids":  kids,
				"Count": total,
			},
			ref: parentRef,
		},
		count: total,
		depth: depth,
	}

	nodes[a] = parentNode
	nodes = append(nodes[:a+1], nodes[b:]...)
	return nodes, nil
}

// wrapIfNeeded ensures that the given dictionary is a /Pages object.
// A wrapper /Pages object is created if necessary.
func (t *Tree) wrapIfNeeded(info *dictInfo) (*dictInfo, error) {
	if info.dict["Type"] == pdf.Name("Pages") {
		return info, nil
	}

	wrapperRef := t.w.Alloc()
	info.dict["Parent"] = wrapperRef
	ref, err := t.w.Write(info.dict, info.ref)
	if err != nil {
		return nil, err
	}

	wrapper := pdf.Dict{
		"Type":  pdf.Name("Pages"),
		"Count": pdf.Integer(1),
		"Kids":  pdf.Array{ref},
	}

	return &dictInfo{dict: wrapper, ref: wrapperRef}, nil
}

const maxDegree = 16
