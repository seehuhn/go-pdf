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
