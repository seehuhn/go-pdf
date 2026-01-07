// Package navnode implements navigation nodes for sub-page navigation.
//
// Navigation nodes allow navigating between different states of the same page
// during presentations. They form a doubly-linked list in PDF format but are
// represented as a slice in Go for ease of use.
//
// This feature was introduced in PDF 1.5 and is typically used with optional
// content groups to show/hide elements like bullet points in presentations.
//
// See section 12.4.4.2 of ISO 32000-2:2020.
package navnode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
)

// Node represents a navigation node dictionary (Table 165).
//
// Each node specifies actions to execute when navigating forward or backward
// within a page's presentation states.
type Node struct {
	// NA is the action (or sequence of actions) executed on forward navigation.
	NA action.Action

	// PA is the action (or sequence of actions) executed on backward navigation.
	PA action.Action

	// Dur is the auto-advance duration in seconds.
	// A value of 0 means no automatic advance.
	Dur float64
}

// Encode writes the navigation node list to PDF format.
// The slice is converted to a doubly-linked list of node dictionaries.
// Returns a reference to the first node, or nil if the slice is empty.
func Encode(rm *pdf.ResourceManager, nodes []*Node) (pdf.Native, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	if err := pdf.CheckVersion(rm.Out, "navigation nodes", pdf.V1_5); err != nil {
		return nil, err
	}

	// allocate references for all nodes
	refs := make([]pdf.Reference, len(nodes))
	for i := range nodes {
		refs[i] = rm.Out.Alloc()
	}

	// build and write each node dictionary
	for i, node := range nodes {
		dict := pdf.Dict{}

		if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
			dict["Type"] = pdf.Name("NavNode")
		}

		if node.NA != nil {
			na, err := node.NA.Encode(rm)
			if err != nil {
				return nil, pdf.Wrap(err, "NA")
			}
			dict["NA"] = na
		}

		if node.PA != nil {
			pa, err := node.PA.Encode(rm)
			if err != nil {
				return nil, pdf.Wrap(err, "PA")
			}
			dict["PA"] = pa
		}

		if node.Dur > 0 {
			dict["Dur"] = pdf.Number(node.Dur)
		}

		// link to next node
		if i+1 < len(nodes) {
			dict["Next"] = refs[i+1]
		}

		// link to previous node
		if i > 0 {
			dict["Prev"] = refs[i-1]
		}

		if err := rm.Out.Put(refs[i], dict); err != nil {
			return nil, err
		}
	}

	return refs[0], nil
}

// Decode reads a navigation node list from PDF format.
// The doubly-linked list starting at obj is flattened into a slice.
func Decode(x *pdf.Extractor, obj pdf.Object) ([]*Node, error) {
	if obj == nil {
		return nil, nil
	}

	cc := pdf.NewCycleChecker()
	var nodes []*Node

	for obj != nil {
		if err := cc.Check(obj); err != nil {
			return nil, err
		}

		dict, err := x.GetDict(obj)
		if err != nil {
			return nil, pdf.Wrap(err, "navigation node")
		}
		if dict == nil {
			break
		}

		node, err := decodeNode(x, dict)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)

		// follow Next pointer (ignore Prev - we reconstruct it on write)
		obj = dict["Next"]
	}

	return nodes, nil
}

func decodeNode(x *pdf.Extractor, dict pdf.Dict) (*Node, error) {
	node := &Node{}

	// NA (optional)
	if naObj := dict["NA"]; naObj != nil {
		na, err := action.Decode(x, naObj)
		if err != nil {
			return nil, pdf.Wrap(err, "NA")
		}
		node.NA = na
	}

	// PA (optional)
	if paObj := dict["PA"]; paObj != nil {
		pa, err := action.Decode(x, paObj)
		if err != nil {
			return nil, pdf.Wrap(err, "PA")
		}
		node.PA = pa
	}

	// Dur (optional)
	if durObj := dict["Dur"]; durObj != nil {
		dur, err := x.GetNumber(durObj)
		if err == nil && dur > 0 {
			node.Dur = dur
		}
	}

	return node, nil
}
