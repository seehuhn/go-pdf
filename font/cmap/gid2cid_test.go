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

package cmap

import (
	"testing"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"
)

// TestGIDToCIDUnassigned checks that a CID no glyph has been assigned to maps
// to glyph 0, in particular one which does not fit into a [glyph.ID].
func TestGIDToCIDUnassigned(t *testing.T) {
	sequential := NewGIDToCIDSequential()
	sequential.CID(1, "A")

	identity := NewGIDToCIDIdentity()

	for _, g := range []struct {
		name string
		g2c  GIDToCID
	}{
		{"sequential", sequential},
		{"identity", identity},
	} {
		t.Run(g.name, func(t *testing.T) {
			for _, c := range []cid.CID{1 << 16, 1<<16 + 1, 1 << 20, 1<<20 + 3, 1<<32 - 1} {
				if gid := g.g2c.GID(c); gid != 0 {
					t.Errorf("CID %d maps to glyph %d, want 0", c, gid)
				}
			}
		})
	}
}

// TestGIDToCIDIdentityRoundTrip checks that the identity mapping is its own
// inverse over the range of a [glyph.ID].
func TestGIDToCIDIdentityRoundTrip(t *testing.T) {
	g2c := NewGIDToCIDIdentity()
	for _, gid := range []glyph.ID{0, 1, 1000, 0xFFFF} {
		if got := g2c.GID(g2c.CID(gid, "")); got != gid {
			t.Errorf("glyph %d round trips to %d", gid, got)
		}
	}
}

func TestGIDToCIDFromMap(t *testing.T) {
	ros := &cid.SystemInfo{Registry: "Test", Ordering: "Private"}
	table := map[glyph.ID]cid.CID{3: 100, 5: 101, 7: 101}
	g2c := NewGIDToCIDFromMap(ros, table)

	// the table is copied
	table[3] = 999

	if got := g2c.ROS(); got != ros {
		t.Errorf("ROS %v, want %v", got, ros)
	}
	for gid, want := range map[glyph.ID]cid.CID{3: 100, 5: 101, 7: 101, 9: 0} {
		if got := g2c.CID(gid, ""); got != want {
			t.Errorf("glyph %d written as CID %d, want %d", gid, got, want)
		}
	}
	// a shared CID selects the smallest glyph, an unknown CID selects none
	for c, want := range map[cid.CID]glyph.ID{100: 3, 101: 5, 102: 0, 0: 0} {
		if got := g2c.GID(c); got != want {
			t.Errorf("CID %d selects glyph %d, want %d", c, got, want)
		}
	}
}
