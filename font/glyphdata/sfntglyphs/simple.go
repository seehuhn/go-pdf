// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package sfntglyphs

import (
	"unicode/utf8"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/font/subset"
)

// NewSimpleSelector returns a function which selects the glyph of a simple
// font with an embedded sfnt program, of any outline flavour.
//
// The argument of the function is a pseudo-CID: 0 selects the .notdef glyph,
// and a value c in the range 1 to 256 selects the glyph shown for the
// character code c-1.  The second result is false if the code selects no
// glyph.
//
// The argument symbolic gives the Symbolic flag of the font descriptor.  It
// only affects programs which do not have CFF outlines, see
// [NewTrueTypeSelector].
//
// For a program with CFF outlines, glyphs are selected by name, as in
// [cffglyphs.NewSimpleSelector]; a name which the program lacks is looked up
// in the Unicode cmap of the font.  For all other programs the selection is
// the one of [NewTrueTypeSelector].
func NewSimpleSelector(font *sfnt.Font, symbolic bool, enc encoding.Simple) func(cid.CID) (glyph.ID, bool) {
	cf := font.AsCFF()
	if cf == nil {
		return NewTrueTypeSelector(font, symbolic, enc)
	}

	byName := cffglyphs.NewSimpleSelector(cf, enc)
	lookup := UnicodeLookup(font)
	_, psName := subset.Split(font.FontName)
	return func(c cid.CID) (glyph.ID, bool) {
		if gid, ok := byName(c); ok {
			return gid, true
		}
		if c == 0 || c > 256 {
			return 0, false
		}
		name, ok := enc(byte(c - 1))
		if !ok || name == encoding.UseBuiltin || name == ".notdef" {
			return 0, false
		}
		gid := lookup(names.ToUnicode(name, psName))
		return gid, gid != 0
	}
}

// UnicodeLookup returns a function which maps a text consisting of a single
// character to the glyph of the font for this character, using the Unicode
// cmap of the font.  The function returns 0 if the text is not a single
// character, or if the font has no glyph for it.
func UnicodeLookup(font *sfnt.Font) func(text string) glyph.ID {
	uni := unicodeCmap(font)
	numGlyphs := font.NumGlyphs()
	return func(text string) glyph.ID {
		if uni == nil {
			return 0
		}
		r, size := utf8.DecodeRuneInString(text)
		if size == 0 || size != len(text) || r == utf8.RuneError && size == 1 {
			return 0
		}
		gid := uni.Lookup(r)
		if int(gid) >= numGlyphs {
			return 0
		}
		return gid
	}
}

// unicodeCmap returns the Unicode cmap subtable of an sfnt font, or nil if
// the font has none.
func unicodeCmap(f *sfnt.Font) cmap.Subtable {
	for _, key := range []struct{ pid, eid uint16 }{{3, 10}, {3, 1}, {0, 4}, {0, 3}} {
		if sub, err := f.CMapTable.GetNoLang(key.pid, key.eid); err == nil && sub != nil {
			return sub
		}
	}
	return nil
}
