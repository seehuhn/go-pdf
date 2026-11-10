// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

import "fmt"

// Object represents the current graphics object context in a content stream.
// See Figure 9 (p. 113) of PDF 32000-1:2008.
type Object byte

func (s Object) String() string {
	switch s {
	case ObjPage:
		return "page"
	case ObjPath:
		return "path"
	case ObjText:
		return "text"
	case ObjClippingPath:
		return "clipping path"
	case ObjType3Start:
		return "type3 start"
	default:
		return fmt.Sprintf("objectType(%d)", s)
	}
}

const (
	ObjPage         Object = 1 << iota // Page-level context (initial state)
	ObjPath                            // Path construction in progress
	ObjText                            // Inside text object (BT...ET)
	ObjClippingPath                    // Clipping path operator executed
	ObjType3Start                      // Awaiting d0 or d1 for Type 3 font glyph

	// ObjAny is a bitmask matching any object state.
	ObjAny = ObjPage | ObjPath | ObjText | ObjClippingPath | ObjType3Start
)
