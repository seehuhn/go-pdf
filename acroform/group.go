// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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
