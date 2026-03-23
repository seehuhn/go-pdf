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

package oc

// OrderItem represents an item in the Order array of an optional content
// configuration dictionary. It is either a *[Group] or an *[OrderGroup].
type OrderItem interface {
	orderItem()
}

// OrderGroup represents a labeled nested array in the Order hierarchy.
type OrderGroup struct {
	// Label (optional) is a non-selectable text label for this sub-group.
	Label string

	// Children contains the nested groups and sub-groups.
	Children []OrderItem
}

func (*Group) orderItem()      {}
func (*OrderGroup) orderItem() {}
