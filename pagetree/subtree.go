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
	if a < 0 || b > len(nodes) || b-a < 2 || b-a > maxDegree {
		// TODO(voss): remove
		panic(fmt.Errorf("invalid subtree node range %d, %d", a, b))
	}
	if a == b {
		return nodes
	}
	childNodes := nodes[a:b]

	parentDict := pdf.Dict{
		"Type": pdf.Name("Pages"),
	}

	extractInheritable(parentDict, childNodes)

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
	parentDict["Kids"] = kids
	parentDict["Count"] = pageCount

	parentNode := &nodeInfo{
		dictInfo: &dictInfo{
			dict: parentDict,
			ref:  parentRef,
		},
		pageCount: pageCount,
		depth:     maxDepth + 1,
	}

	nodes[a] = parentNode
	nodes = append(nodes[:a+1], nodes[b:]...)
	return nodes
}

// extractInheritable extracts inheritable attributes from the child nodes
// and adds them to the parentDict.
func extractInheritable(parentDict pdf.Dict, childNodes []*nodeInfo) {
	inheritRotate(parentDict, childNodes)

	// TODO(voss): the PDF-2.0 spec says that Resources "should not"
	// be inherited for newly written documents.  Should we obey this?
}

func inheritRotate(parentDict pdf.Dict, childNodes []*nodeInfo) {
	// TODO(voss): in an internal node, an unset Rotate value
	// can mean either of two things:
	//   - some children need to inherit 0 (via the default value)
	//   - all children set their own value, so the parent value
	//     does not matter.
	// This distinction is important for follow-up merges.
	// Implement this distinction.

	n := len(childNodes)
	repr := make([]string, n)
	count := make(map[string]int)

	key := pdf.Name("Rotate")

	defaultValue := pdf.Integer(0)
	defaultString, _ := pdf.Format(defaultValue)
	numDefault := 0
	for i, node := range childNodes {
		val, ok := node.dict[key]
		if !ok {
			val = defaultValue
		}
		r, err := pdf.Format(val)
		if err != nil {
			// Can't format value??!  Let someone else deal with this.
			return
		}
		repr[i] = r
		count[r]++
		if r == defaultString {
			numDefault++
			delete(node.dict, key)
		}
	}

	var bestDiff int
	bestRepr := defaultString
	for r, k := range count {
		if r == defaultString {
			continue
		}

		// Compute the change in (uncompressed) file size if we
		// were to use this value for the parent instead of leaving
		// the parent value unset.
		var diff int
		diff += 1 + len(key) + 1 + len(r) + 1       // entry in parent dict
		diff -= k * (1 + len(key) + 1 + len(r) + 1) // entries in child dicts
		diff += numDefault * (1 + len(key) + 1 + len(defaultString) + 1)

		if diff <= bestDiff {
			bestDiff = diff
			bestRepr = r
		}
	}

	if bestRepr == defaultString {
		return
	}
	// find a PDF object corresponding to bestRepr and copy this to the parent
	for i, node := range childNodes {
		if repr[i] == bestRepr {
			parentDict[key] = node.dict[key]
			break
		}
	}
	for i, child := range childNodes {
		switch repr[i] {
		case bestRepr:
			delete(child.dict, key)
		case defaultString:
			child.dict[key] = defaultValue
		}
	}
}
