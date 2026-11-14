// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap_test

import (
	"errors"
	"io/fs"
	"maps"
	"testing"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/mapping"
	"seehuhn.de/go/pdf/internal/debug/makefont"
)

// encodableCIDs returns the CIDs the CMap has a code for, a code mapped by
// both a CMap and its parent belonging to the child.
func encodableCIDs(t *testing.T, f *cmap.File) map[cid.CID]bool {
	t.Helper()
	codec, err := f.Codec()
	if err != nil {
		t.Fatal(err)
	}
	byCode := maps.Collect(f.All(codec))
	res := make(map[cid.CID]bool)
	for _, c := range byCode {
		res[c] = true
	}
	return res
}

// TestGIDToCIDFromCMapEncodable checks that every glyph is written as a CID
// the CMap has a code for, and that every such CID selects its glyph.
func TestGIDToCIDFromCMapEncodable(t *testing.T) {
	info := makefont.TrueType()
	lookup, err := info.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}
	f, err := cmap.Predefined("UniJIS-UCS2-H")
	if err != nil {
		t.Fatal(err)
	}
	m, err := mapping.GetCIDTextMapping(f.ROS.Registry, f.ROS.Ordering)
	if err != nil {
		t.Fatal(err)
	}

	g2c, err := cmap.NewGIDToCIDFromCMap(f, lookup)
	if err != nil {
		t.Fatal(err)
	}

	encodable := encodableCIDs(t, f)
	numShared := 0
	for c := range encodable {
		rr := []rune(m[c])
		if len(rr) != 1 {
			continue
		}
		gid := lookup.Lookup(rr[0])
		if gid == 0 {
			continue
		}
		if got := g2c.GID(c); got != gid {
			t.Errorf("CID %d selects glyph %d, want %d", c, got, gid)
		}
		back := g2c.CID(gid, m[c])
		if !encodable[back] {
			t.Errorf("glyph %d is written as CID %d, which the CMap cannot encode", gid, back)
		}
		if back > c {
			t.Errorf("glyph %d is written as CID %d, want at most %d", gid, back, c)
		} else if back < c {
			numShared++
		}
	}
	if numShared == 0 {
		t.Error("no glyph is shared by several CIDs, so nothing was checked")
	}
}

// TestGIDToCIDFromCMapChildWins checks that a code the CMap remaps takes the
// child's CID: UniJIS-UCS2-HW-H maps ASCII to half-width glyphs, unlike its
// parent UniJIS-UCS2-H.
func TestGIDToCIDFromCMapChildWins(t *testing.T) {
	info := makefont.TrueType()
	lookup, err := info.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}
	f, err := cmap.Predefined("UniJIS-UCS2-HW-H")
	if err != nil {
		t.Fatal(err)
	}
	g2c, err := cmap.NewGIDToCIDFromCMap(f, lookup)
	if err != nil {
		t.Fatal(err)
	}

	gid := lookup.Lookup('A')
	want := f.LookupCID([]byte{0x00, 'A'})
	if want == 34 {
		t.Fatal("test CMap does not remap ASCII")
	}
	if got := g2c.CID(gid, "A"); got != want {
		t.Errorf("A is written as CID %d, want %d", got, want)
	}
}

func TestGIDToCIDFromCMapUnknownCollection(t *testing.T) {
	info := makefont.TrueType()
	lookup, err := info.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}
	f := &cmap.File{
		ROS:            &cid.SystemInfo{Registry: "Test", Ordering: "Unknown"},
		CodeSpaceRange: charcode.UCS2,
		CIDRanges:      []cmap.Range{{First: []byte{0, 0}, Last: []byte{0xFF, 0xFF}, Value: 0}},
	}
	_, err = cmap.NewGIDToCIDFromCMap(f, lookup)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("got %v, want an error wrapping fs.ErrNotExist", err)
	}
}

// TestGIDToCIDFromCMapNotdef checks that CID 0 is glyph 0 even though the
// Japan1 collection implies U+FFFD for CID 0 and the font has a glyph for it.
func TestGIDToCIDFromCMapNotdef(t *testing.T) {
	info := makefont.TrueType()
	lookup, err := info.CMapTable.GetBest()
	if err != nil {
		t.Fatal(err)
	}
	f, err := cmap.Predefined("Adobe-Japan1-7")
	if err != nil {
		t.Fatal(err)
	}
	g2c, err := cmap.NewGIDToCIDFromCMap(f, lookup)
	if err != nil {
		t.Fatal(err)
	}

	fffd := lookup.Lookup(0xFFFD)
	if fffd == 0 {
		t.Fatal("test font has no glyph for U+FFFD")
	}
	if got := g2c.GID(0); got != 0 {
		t.Errorf("CID 0 selects glyph %d, want 0", got)
	}
	if got := g2c.CID(0, ""); got != 0 {
		t.Errorf("glyph 0 is written as CID %d, want 0", got)
	}
	for c := range encodableCIDs(t, f) {
		if g2c.GID(c) == fffd {
			t.Errorf("CID %d selects the U+FFFD glyph", c)
		}
	}
}
