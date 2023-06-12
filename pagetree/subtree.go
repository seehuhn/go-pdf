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
	"math"

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

	inherit(parentDict, childNodes)

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

func sanitize(pageDict pdf.Dict) {
	if _, ok := pageDict["Resources"]; !ok {
		// This field is required by the spec.  If the called failed to assign
		// a value, we use an empty dict (indicating that no resources are
		// required by the page).
		pageDict["Resources"] = pdf.Dict{}
	}

	if _, ok := pageDict["Rotate"]; !ok {
		// This is the default value.  We add it here to help the function
		// [inherit].  Our convention:
		//
		// - 0: At least one page in this subtree inherits the value
		//   and expects the inherited value to be 0.
		// - unset (on internal nodes): all child nodes set the value
		//   explicitly, so the value in the current node does not matter.
		// - anything else: the value is set explicitly in this node.
		//
		// If possible, [inherit] will remove this entry again before writing
		// the node to the output.
		pageDict["Rotate"] = pdf.Integer(0)
	}
}

// inherit extracts inheritable attributes from the child nodes
// and adds them to the parentDict.
func inherit(parentDict pdf.Dict, childNodes []*nodeInfo) {
	// We don't try to inherit /Resources, since (a) it is uncommon
	// for different pages to use exactly the same resources, and
	// (b) the PDF-2.0 spec recommends against it.
	inheritKey("MediaBox", parentDict, childNodes)
	inheritKey("CropBox", parentDict, childNodes)
	inheritRotate(parentDict, childNodes)
}

func inheritKey(key pdf.Name, parentDict pdf.Dict, childNodes []*nodeInfo) {
	n := len(childNodes)
	repr := make([]string, n)
	count := make(map[string]int)

	for i, node := range childNodes {
		val, ok := node.dict[key]
		if !ok {
			// If a child lacks the field, we can't use inheritance,
			// because there is no way to override an inherited
			// value with the "unset" value.
			return
		}
		r, err := pdf.Format(val)
		if err != nil {
			// Can't format value??!  Let someone else deal with this.
			return
		}
		repr[i] = r
		count[r]++
	}

	bestDiff := math.MaxInt
	var bestRepr string
	l := len(key) + 3 // len("/" + key + " " + ... + "\n")
	for repr, k := range count {
		// Compute the change in (uncompressed) file size if we were to use
		// `repr` for the parent instead of leaving the parent value unset.
		var diff int
		diff += (l + len(repr))     // add entry in parent dict
		diff -= k * (l + len(repr)) // remove entry from k child dicts

		if diff < bestDiff {
			bestDiff = diff
			bestRepr = repr
		}
	}

	if bestRepr == "" {
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
		if repr[i] == bestRepr {
			delete(child.dict, key)
		}
	}
}

func inheritRotate(parentDict pdf.Dict, childNodes []*nodeInfo) {
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
			// Missing entries indicate that this child does not
			// use the inherited value.
			continue
		}
		r, err := pdf.Format(val)
		if err != nil {
			// Can't format value??!  Let someone else deal with this.
			return
		}
		repr[i] = r
		count[r]++
		if r == defaultString {
			// This indicates that the child needs to inherit the
			// default value or have the default value set explicitly.
			numDefault++
			delete(node.dict, key)
		}
	}

	bestDiff := 0
	bestRepr := defaultString
	l := len(key) + 3 // len("/" + key + " " + ... + "\n")
	for repr, k := range count {
		if repr == defaultString {
			continue
		}

		// Compute the change in (uncompressed) file size if we were to use
		// `repr` for the parent instead of leaving the parent value unset.
		var diff int
		diff += (l + len(repr))                       // add entry in parent dict
		diff -= k * (l + len(repr))                   // remove entry from k child dicts
		diff += numDefault * (l + len(defaultString)) // explicitly set defaults where needed

		if diff <= bestDiff {
			// Use "<=" instead of "<" to prefer moving the value to the
			// parent.  This potentially allows to move the values further
			// towards the root.
			bestDiff = diff
			bestRepr = repr
		}
	}

	if bestRepr == defaultString {
		if numDefault != 0 {
			// Record the fact that our children need to inherit the
			// default value.
			parentDict[key] = defaultValue
		}
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
