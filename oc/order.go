// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package oc

import "seehuhn.de/go/pdf"

// OrderItem represents an item in the Order array of an optional content
// configuration dictionary. It is either a [*Group] or an [*OrderGroup].
type OrderItem interface {
	orderItem()
}

// OrderGroup represents a labeled nested array in the Order hierarchy.
type OrderGroup struct {
	// Label (optional) is a non-selectable text label for this sub-group.
	Label pdf.TextString

	// Children contains the nested groups and sub-groups.
	Children []OrderItem
}

func (*Group) orderItem()      {}
func (*OrderGroup) orderItem() {}
