// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
)

// UTF8H and UTF8V are templates for horizontal and vertical writing: a code
// space with no mappings.  A composite font written with one allocates codes
// for its text from the UTF-8 code space, so that the strings in the content
// stream are the text itself, and embeds the CMap this produces.
//
// The templates must not be modified.
var (
	UTF8H = &File{Name: "UTF-8-H", CodeSpaceRange: charcode.UTF8}
	UTF8V = &File{Name: "UTF-8-V", WMode: font.Vertical, CodeSpaceRange: charcode.UTF8}
)
