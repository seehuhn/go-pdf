// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cffglyphs_test

import (
	"slices"
	"testing"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func TestNewSimpleSelector(t *testing.T) {
	cf := makefont.OpenType().AsCFF()
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
	sel := cffglyphs.NewSimpleSelector(cf, enc)

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

func TestNewSimpleSelectorDuplicateName(t *testing.T) {
	cf := makefont.OpenType().AsCFF()
	if len(cf.Glyphs) < 4 {
		t.Fatal("too few glyphs")
	}
	o := *cf.Outlines
	o.Glyphs = slices.Clone(o.Glyphs)
	for _, gid := range []int{1, 3} {
		g := *o.Glyphs[gid]
		g.Name = "dup"
		o.Glyphs[gid] = &g
	}
	cf.Outlines = &o

	enc := encoding.Simple(func(code byte) (string, bool) { return "dup", true })
	sel := cffglyphs.NewSimpleSelector(cf, enc)
	if got, ok := sel(1); got != 1 || !ok {
		t.Errorf("got glyph %d, %v; want 1", got, ok)
	}
}

func TestNewSimpleSelectorBuiltin(t *testing.T) {
	cf := makefont.OpenType().AsCFF()
	if len(cf.Glyphs) < 3 {
		t.Fatal("too few glyphs")
	}
	cf.Encoding = []glyph.ID{0, 1, 2}

	enc := encoding.Simple(func(code byte) (string, bool) {
		return encoding.UseBuiltin, true
	})
	sel := cffglyphs.NewSimpleSelector(cf, enc)

	if got, ok := sel(3); got != 2 || !ok {
		t.Errorf("code 2: got %d, %v; want 2", got, ok)
	}
	if got, ok := sel(2); got != 1 || !ok {
		t.Errorf("code 1: got %d, %v; want 1", got, ok)
	}
	for _, c := range []cid.CID{4, 100, 256} {
		if _, ok := sel(c); ok {
			t.Errorf("CID %d, beyond the built-in encoding, selects a glyph", c)
		}
	}
}

func TestNewSimpleSelectorBuiltinUnencoded(t *testing.T) {
	cf := makefont.OpenType().AsCFF()
	cf.Encoding = []glyph.ID{0, 2}
	enc := encoding.Simple(func(code byte) (string, bool) { return encoding.UseBuiltin, true })
	sel := cffglyphs.NewSimpleSelector(cf, enc)

	if got, ok := sel(1); ok {
		t.Errorf("unencoded code selects glyph %d", got)
	}
	if got, ok := sel(2); got != 2 || !ok {
		t.Errorf("encoded code: got %d, %v; want 2", got, ok)
	}
}
