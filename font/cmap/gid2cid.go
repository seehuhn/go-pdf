// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"slices"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/pdf/font"
	pscid "seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"
)

// NewGIDToCIDSequential returns a GIDToCID which assigns CID values
// sequentially, starting with 1.
func NewGIDToCIDSequential() font.GIDToCID {
	return &gidToCIDSequential{
		g2c: make(map[glyph.ID]pscid.CID),
		c2g: make(map[pscid.CID]glyph.ID),
	}
}

type gidToCIDSequential struct {
	g2c map[glyph.ID]pscid.CID
	c2g map[pscid.CID]glyph.ID
}

// GID implements the [GIDToCID] interface.
func (g *gidToCIDSequential) CID(gid glyph.ID, _ []rune) pscid.CID {
	cid, ok := g.g2c[gid]
	if !ok {
		cid = pscid.CID(len(g.g2c) + 1)
		g.g2c[gid] = cid
		g.c2g[cid] = gid
	}
	return cid
}

func (g *gidToCIDSequential) GID(cid pscid.CID) glyph.ID {
	return g.c2g[cid]
}

// ROS implements the [GIDToCID] interface.
func (g *gidToCIDSequential) ROS() *font.CIDSystemInfo {
	h := sha256.New()
	h.Write([]byte("seehuhn.de/go/pdf/font/cmap.gidToCIDSequential\n"))
	binary.Write(h, binary.BigEndian, uint64(len(g.g2c)))
	gg := maps.Keys(g.g2c)
	slices.Sort(gg)
	for _, gid := range gg {
		binary.Write(h, binary.BigEndian, gid)
		binary.Write(h, binary.BigEndian, g.g2c[gid])
	}
	sum := h.Sum(nil)

	return &font.CIDSystemInfo{
		Registry:   "Seehuhn",
		Ordering:   fmt.Sprintf("%x", sum[:8]),
		Supplement: 0,
	}
}

// GIDToCID implements the [GIDToCID] interface.
func (g *gidToCIDSequential) GIDToCID(numGlyph int) []pscid.CID {
	res := make([]pscid.CID, numGlyph)
	for gid, cid := range g.g2c {
		res[gid] = cid
	}
	return res
}

// NewGIDToCIDIdentity returns a GIDToCID which uses the GID values
// directly as CID values.
func NewGIDToCIDIdentity() font.GIDToCID {
	return &gidToCIDIdentity{}
}

type gidToCIDIdentity struct{}

// GID implements the [GIDToCID] interface.
func (g *gidToCIDIdentity) CID(gid glyph.ID, _ []rune) pscid.CID {
	return pscid.CID(gid)
}

// CID implements the [GIDToCID] interface.
func (g *gidToCIDIdentity) GID(cid pscid.CID) glyph.ID {
	return glyph.ID(cid)
}

// ROS implements the [GIDToCID] interface.
func (g *gidToCIDIdentity) ROS() *font.CIDSystemInfo {
	return &font.CIDSystemInfo{
		Registry:   "Adobe",
		Ordering:   "Identity",
		Supplement: 0,
	}
}

// GIDToCID implements the [GIDToCID] interface.
func (g *gidToCIDIdentity) GIDToCID(numGlyph int) []pscid.CID {
	res := make([]pscid.CID, numGlyph)
	for i := range res {
		res[i] = pscid.CID(i)
	}
	return res
}
