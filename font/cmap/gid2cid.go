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

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"
)

// GIDToCID encodes a mapping from Glyph Identifier (GID) values to Character
// Identifier (CID) values.
type GIDToCID interface {
	// TODO(voss): change the second argument to string
	CID(glyph.ID, []rune) cid.CID

	GID(cid.CID) glyph.ID

	ROS() *CIDSystemInfo

	GIDToCID(numGlyph int) []cid.CID
}

// NewGIDToCIDSequential returns a GIDToCID which assigns CID values
// sequentially, starting with 1.
func NewGIDToCIDSequential() GIDToCID {
	g2c := make(map[glyph.ID]cid.CID)
	c2g := make(map[cid.CID]glyph.ID)

	g2c[0] = 0
	c2g[0] = 0

	return &gidToCIDSequential{
		g2c: g2c,
		c2g: c2g,
	}
}

type gidToCIDSequential struct {
	g2c map[glyph.ID]cid.CID
	c2g map[cid.CID]glyph.ID
}

// GID implements the [GIDToCID] interface.
func (g *gidToCIDSequential) CID(gid glyph.ID, _ []rune) cid.CID {
	cidVal, ok := g.g2c[gid]
	if !ok {
		cidVal = cid.CID(len(g.g2c))
		g.g2c[gid] = cidVal
		g.c2g[cidVal] = gid
	}
	return cidVal
}

func (g *gidToCIDSequential) GID(cid cid.CID) glyph.ID {
	return g.c2g[cid]
}

// ROS implements the [GIDToCID] interface.
func (g *gidToCIDSequential) ROS() *CIDSystemInfo {
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

	return &CIDSystemInfo{
		Registry:   "Seehuhn",
		Ordering:   fmt.Sprintf("%x", sum[:8]),
		Supplement: 0,
	}
}

// GIDToCID implements the [GIDToCID] interface.
func (g *gidToCIDSequential) GIDToCID(numGlyph int) []cid.CID {
	res := make([]cid.CID, numGlyph)
	for gid, cid := range g.g2c {
		res[gid] = cid
	}
	return res
}

// NewGIDToCIDIdentity returns a GIDToCID which uses the GID values
// directly as CID values.
func NewGIDToCIDIdentity() GIDToCID {
	return &gidToCIDIdentity{}
}

type gidToCIDIdentity struct{}

// GID implements the [GIDToCID] interface.
func (g *gidToCIDIdentity) CID(gid glyph.ID, _ []rune) cid.CID {
	return cid.CID(gid)
}

// CID implements the [GIDToCID] interface.
func (g *gidToCIDIdentity) GID(cid cid.CID) glyph.ID {
	return glyph.ID(cid)
}

// ROS implements the [GIDToCID] interface.
func (g *gidToCIDIdentity) ROS() *CIDSystemInfo {
	return &CIDSystemInfo{
		Registry:   "Adobe",
		Ordering:   "Identity",
		Supplement: 0,
	}
}

// GIDToCID implements the [GIDToCID] interface.
func (g *gidToCIDIdentity) GIDToCID(numGlyph int) []cid.CID {
	res := make([]cid.CID, numGlyph)
	for i := range res {
		res[i] = cid.CID(i)
	}
	return res
}
