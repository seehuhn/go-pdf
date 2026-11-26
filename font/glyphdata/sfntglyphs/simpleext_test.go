// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package sfntglyphs_test

import (
	"testing"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata/sfntglyphs"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func TestNewSimpleSelector(t *testing.T) {
	f := makefont.OpenType()
	cf := f.AsCFF()
	if cf == nil {
		t.Fatal("no CFF outlines")
	}
	var gid glyph.ID
	var name string
	for i, g := range cf.Glyphs {
		if i > 0 && g.Name != "" && g.Name != ".notdef" {
			gid, name = glyph.ID(i), g.Name
			break
		}
	}
	if gid == 0 {
		t.Fatal("no named glyph")
	}

	enc := encoding.Simple(func(code byte) (string, bool) {
		switch code {
		case 10:
			return name, true
		case 11:
			return ".notdef", true
		case 12:
			return "nosuchglyph", true
		}
		return "", false
	})
	sel := sfntglyphs.NewSimpleSelector(f, false, enc)

	if got, ok := sel(0); got != 0 || !ok {
		t.Errorf("CID 0: got %d, %v", got, ok)
	}
	if got, ok := sel(11); got != gid || !ok {
		t.Errorf("named glyph: got %d, %v; want %d", got, ok, gid)
	}
	for _, c := range []cid.CID{12, 13, 14, 257} {
		if _, ok := sel(c); ok {
			t.Errorf("CID %d selects a glyph", c)
		}
	}
}

func TestNewSimpleSelectorUnicodeFallback(t *testing.T) {
	f := makefont.OpenType()
	want := sfntglyphs.UnicodeLookup(f)("A")
	if want == 0 {
		t.Fatal("no glyph for A")
	}
	for _, g := range f.AsCFF().Glyphs {
		if g.Name == "uni0041" {
			t.Fatal("test name is present in the program")
		}
	}

	enc := encoding.Simple(func(code byte) (string, bool) {
		if code == 10 {
			return "uni0041", true
		}
		return "", false
	})
	sel := sfntglyphs.NewSimpleSelector(f, false, enc)
	if got, ok := sel(11); got != want || !ok {
		t.Errorf("got %d, %v; want %d", got, ok, want)
	}
}

func TestUnicodeLookup(t *testing.T) {
	f := makefont.TrueType()
	lookup := sfntglyphs.UnicodeLookup(f)
	if lookup("A") == 0 {
		t.Error("no glyph for A")
	}
	for _, text := range []string{"", "AB", "\xff"} {
		if gid := lookup(text); gid != 0 {
			t.Errorf("%q: got glyph %d, want none", text, gid)
		}
	}
}
