// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package acroform

// PDF 2.0 sections: 12.7.4.1 12.7.4.2

// Group is an interior node of a field tree which can be used to group fields
// together.
//
// There are two reasons to use a group:
//  1. To give a common partial field name to a set of fields, which is then
//     prepended to the fully qualified names of all descendants.
//  2. If some attributes have the same value for all descendants,
//     PDF file size will be reduced by storing the attribute once in the group
//     instead of repeating it for each descendant.
type Group struct {
	// Name (optional) is the partial field name shared by the group's
	// descendants. An empty value contributes no component to their fully
	// qualified names.
	//
	// This corresponds to the /T entry in the PDF dictionary.
	Name string

	// Children holds the group's children: sub-groups and terminal fields.
	//
	// This corresponds to the /Kids entry in the PDF dictionary.
	Children []Node
}

// PartialName implements the [Node] interface.
func (g *Group) PartialName() string { return g.Name }

var _ Node = (*Group)(nil)
