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

// Group is an interior node of a field tree. It is not a field of its own; it
// exists only to give its descendants a common partial-name prefix. In a PDF
// file a group corresponds to a non-terminal field dictionary, which a spec
// calls "merely a container for inheritable attributes" (12.7.4.1); the
// inheritable attributes themselves are managed by the encoder and are not part
// of this type.
type Group struct {
	// Name (optional) is the partial field name shared by the group's
	// descendants. An empty value contributes no component to their fully
	// qualified names.
	//
	// This corresponds to the /T entry in the PDF field dictionary.
	Name string

	// Kids holds the group's children: sub-groups and terminal fields.
	Kids []TreeNode
}

// PartialName implements the [TreeNode] interface.
func (g *Group) PartialName() string { return g.Name }

var _ TreeNode = (*Group)(nil)
