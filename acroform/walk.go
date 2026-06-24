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

import "iter"

// AllFields returns an iterator over the form's terminal fields, in depth-first
// tree order. Each field is yielded with its fully qualified name, formed by
// joining the partial names of the field and its ancestors with a period;
// nodes without a partial name contribute nothing to the name.
func (f *InteractiveForm) AllFields() iter.Seq2[string, Field] {
	return func(yield func(string, Field) bool) {
		for _, node := range f.Fields {
			if !walkFields(node, "", yield) {
				return
			}
		}
	}
}

// walkFields visits the terminal fields of the subtree rooted at node,
// prefixing each fully qualified name with prefix. It returns false as soon as
// yield returns false, to stop the iteration.
func walkFields(node Node, prefix string, yield func(string, Field) bool) bool {
	name := joinName(prefix, node.PartialName())
	switch t := node.(type) {
	case *Group:
		for _, kid := range t.Kids {
			if !walkFields(kid, name, yield) {
				return false
			}
		}
		return true
	case Field:
		return yield(name, t)
	default:
		return true
	}
}

// joinName appends a partial name to a prefix, separated by a period. Either
// part may be empty.
func joinName(prefix, partial string) string {
	switch {
	case partial == "":
		return prefix
	case prefix == "":
		return partial
	default:
		return prefix + "." + partial
	}
}
