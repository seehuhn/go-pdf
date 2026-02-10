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

package sfntglyphs

import (
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
)

// NewTrueTypeSelector returns a function that maps pseudo-CIDs to glyph IDs
// for a simple TrueType or OpenType/glyf font.
//
// CID 0 always maps to GID 0 (.notdef). For CIDs 1-256, the code byte is
// CID-1, and the glyph is selected using the methods from PDF spec section
// 9.6.5.4. The method order depends on the symbolic flag and whether an
// encoding is present (enc != nil).
func NewTrueTypeSelector(font *sfnt.Font, symbolic bool, enc encoding.Simple) func(cid.CID) (glyph.ID, bool) {
	var ll []ttLookup
	if enc != nil {
		ll = methodD(ll, font)
		if symbolic {
			ll = methodB(ll, font)
			ll = methodE(ll, font)
			ll = methodA(ll, font)
		} else {
			ll = methodC(ll, font)
			ll = methodE(ll, font)
			ll = methodB(ll, font)
		}
	} else {
		ll = methodB(ll, font)
		ll = methodA(ll, font)
	}

	numGlyphs := font.Outlines.NumGlyphs()

	return func(c cid.CID) (glyph.ID, bool) {
		if c == 0 {
			return 0, true
		}
		if c > 256 {
			return 0, false
		}

		code := byte(c - 1)
		var name string
		if enc != nil {
			name = enc(code)
		}

		// TODO(voss): reconsider fallback behaviour.  Currently, if a
		// method's cmap table is present but returns GID 0, we fall
		// through to the next method.  It may be more correct to commit
		// to a method once its table is found, even if the result is
		// GID 0 (.notdef).
		for _, lookup := range ll {
			gid, ok := lookup(code, name)
			if ok {
				if int(gid) >= numGlyphs {
					gid = 0
				}
				return gid, true
			}
		}
		return 0, false
	}
}

type ttLookup func(code byte, name string) (glyph.ID, bool)

func methodA(ll []ttLookup, font *sfnt.Font) []ttLookup {
	table, err := font.CMapTable.GetNoLang(1, 0)
	if err != nil {
		return ll
	}
	return append(ll, func(code byte, name string) (glyph.ID, bool) {
		gid := table.Lookup(rune(code))
		return gid, gid != 0
	})
}

func methodB(ll []ttLookup, font *sfnt.Font) []ttLookup {
	table, err := font.CMapTable.GetNoLang(3, 0)
	if err != nil {
		return ll
	}
	return append(ll, func(code byte, name string) (glyph.ID, bool) {
		for _, base := range []rune{0x0000, 0xF000, 0xF100, 0xF200} {
			gid := table.Lookup(rune(code) + base)
			if gid != 0 {
				return gid, true
			}
		}
		return 0, false
	})
}

func methodC(ll []ttLookup, font *sfnt.Font) []ttLookup {
	table, err := font.CMapTable.GetNoLang(1, 0)
	if err != nil {
		return ll
	}

	macOSRomanInv := make(map[string]rune)
	for c, name := range pdfenc.MacRomanAlt.Encoding {
		if name == ".notdef" {
			continue
		}
		if _, ok := macOSRomanInv[name]; !ok {
			macOSRomanInv[name] = rune(c)
		}
	}

	return append(ll, func(code byte, name string) (glyph.ID, bool) {
		if name == encoding.UseBuiltin {
			return 0, false
		}
		r, ok := macOSRomanInv[name]
		if !ok {
			return 0, false
		}
		gid := table.Lookup(r)
		return gid, gid != 0
	})
}

func methodD(ll []ttLookup, font *sfnt.Font) []ttLookup {
	table, err := font.CMapTable.GetNoLang(3, 1)
	if err != nil {
		return ll
	}

	return append(ll, func(code byte, name string) (glyph.ID, bool) {
		switch name {
		case ".notdef":
			return 0, true
		case encoding.UseBuiltin:
			return 0, false
		}

		s := names.ToUnicode(name, font.PostScriptName())
		rr := []rune(s)
		if len(rr) != 1 {
			return 0, false
		}
		r := rr[0]

		gid := table.Lookup(r)
		return gid, gid != 0
	})
}

func methodE(ll []ttLookup, font *sfnt.Font) []ttLookup {
	outlines := font.Outlines.(*glyf.Outlines)
	if len(outlines.Names) == 0 {
		return ll
	}

	m := make(map[string]glyph.ID)
	for gid, name := range outlines.Names {
		if name == "" {
			continue
		}
		m[name] = glyph.ID(gid)
	}

	return append(ll, func(code byte, name string) (glyph.ID, bool) {
		gid := m[name]
		return gid, gid != 0
	})
}
