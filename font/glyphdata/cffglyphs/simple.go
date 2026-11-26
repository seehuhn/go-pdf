// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cffglyphs

import (
	"slices"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font/encoding"
)

// NewSimpleSelector returns a function which selects the glyph of a simple
// font with an embedded CFF program.
//
// The argument of the function is a pseudo-CID: 0 selects the .notdef glyph,
// and a value c in the range 1 to 256 selects the glyph shown for the
// character code c-1.  The glyph is looked up by the name which enc gives for
// the code, or through the built-in encoding of the font program where enc
// asks for it.  Where several glyphs have the same name, the one with the
// lowest glyph ID is selected.  The second result is false if the code
// selects no glyph.
func NewSimpleSelector(cf *cff.Font, enc encoding.Simple) func(cid.CID) (glyph.ID, bool) {
	nameToGID := make(map[string]glyph.ID, len(cf.Glyphs))
	for gid, g := range slices.Backward(cf.Glyphs) {
		if g.Name != "" {
			nameToGID[g.Name] = glyph.ID(gid)
		}
	}
	return func(c cid.CID) (glyph.ID, bool) {
		if c == 0 {
			return 0, true
		}
		if c > 256 {
			return 0, false
		}
		code := byte(c - 1)
		name, ok := enc(code)
		switch {
		case !ok || name == ".notdef":
			return 0, false
		case name == encoding.UseBuiltin:
			if int(code) >= len(cf.Encoding) || cf.Encoding[code] == 0 {
				return 0, false
			}
			return cf.Encoding[code], true
		}
		gid, ok := nameToGID[name]
		return gid, ok
	}
}
