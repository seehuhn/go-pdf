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

package pagetree

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

type dictInfo struct {
	dict pdf.Dict // a \Page or \Pages object
	ref  pdf.Reference
}

type nodeInfo struct {
	*dictInfo
	pageCount pdf.Integer
	depth     int // upper bound
}

func (w *Writer) merge(a, b []*nodeInfo) []*nodeInfo {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
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
		a = w.mergeNodes(a, start, len(a))
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
			a = w.mergeNodes(a, start, start+maxDegree)
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

	return a
}

// mergeNodes collapses nodes a, ..., b-1 into a new internal node.
func (w *Writer) mergeNodes(nodes []*nodeInfo, a, b int) []*nodeInfo {
	// TODO(voss): move inheritable attributes to the new node,
	// where possible.

	if a < 0 || b > len(nodes) || b-a < 2 || b-a > maxDegree {
		// TODO(voss): remove
		panic(fmt.Errorf("invalid subtree node range %d, %d", a, b))
	}
	if a == b {
		return nodes
	}

	childNodes := nodes[a:b]

	kids := make(pdf.Array, len(childNodes))
	parentRef := w.Out.Alloc()
	var pageCount pdf.Integer
	maxDepth := 0
	for i, node := range childNodes {
		node.dict["Parent"] = parentRef
		kids[i] = node.ref

		w.outRefs = append(w.outRefs, node.ref)
		w.outObjects = append(w.outObjects, node.dict)

		pageCount += node.pageCount
		if node.depth > maxDepth {
			maxDepth = node.depth
		}
	}

	parentNode := &nodeInfo{
		dictInfo: &dictInfo{
			dict: pdf.Dict{
				"Type":  pdf.Name("Pages"),
				"Kids":  kids,
				"Count": pageCount,
			},
			ref: parentRef,
		},
		pageCount: pageCount,
		depth:     maxDepth + 1,
	}

	nodes[a] = parentNode
	nodes = append(nodes[:a+1], nodes[b:]...)
	return nodes
}
