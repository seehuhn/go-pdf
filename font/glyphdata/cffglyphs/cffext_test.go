// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cffglyphs_test

import (
	"testing"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font/glyphdata/cffglyphs"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

func TestCIDToGID(t *testing.T) {
	cf := makefont.OpenTypeCID().AsCFF()
	n := cf.NumGlyphs()
	if n < 3 {
		t.Fatal("font too small")
	}
	// reverse the CIDs
	cf.Outlines = func() *cff.Outlines {
		o := *cf.Outlines
		o.GIDToCID = make([]cid.CID, n)
		for i := range o.GIDToCID {
			o.GIDToCID[i] = cid.CID(n - 1 - i)
		}
		return &o
	}()
	toGID := cffglyphs.CIDToGID(cf)
	for g := range n {
		if got := toGID(cid.CID(n - 1 - g)); got != glyph.ID(g) {
			t.Errorf("CID %d: got glyph %d, want %d", n-1-g, got, g)
		}
	}
	if got := toGID(cid.CID(n + 5)); got != 0 {
		t.Errorf("unknown CID: got glyph %d, want 0", got)
	}

	plain := makefont.OpenType().AsCFF()
	if got := cffglyphs.CIDToGID(plain)(2); got != 2 {
		t.Errorf("name-keyed font: got glyph %d, want 2", got)
	}
}

func TestCIDToGIDNotCIDKeyedRange(t *testing.T) {
	cf := makefont.OpenType().AsCFF()
	if got := cffglyphs.CIDToGID(cf)(cid.CID(len(cf.Glyphs))); got != 0 {
		t.Errorf("CID beyond the glyphs: got glyph %d, want 0", got)
	}
}
