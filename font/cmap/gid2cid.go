// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package cmap

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"maps"
	"math"
	"slices"
	"sync"

	"seehuhn.de/go/pdf/font/mapping"
	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"
)

// GIDToCID encodes a mapping from Glyph Identifier (GID) values to Character
// Identifier (CID) values.
//
// CID 0 and glyph 0 always correspond: GID(0) is 0, and glyph 0 is written
// as CID 0.
//
// Implementations must be safe for concurrent use.
type GIDToCID interface {
	CID(glyph.ID, string) cid.CID

	GID(cid.CID) glyph.ID

	ROS() *cid.SystemInfo
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
	// mu guards the two maps.  Entries are only ever added, never changed,
	// so a CID once assigned stays valid.
	mu  sync.RWMutex
	g2c map[glyph.ID]cid.CID
	c2g map[cid.CID]glyph.ID
}

// CID implements the [GIDToCID] interface.
func (g *gidToCIDSequential) CID(gid glyph.ID, _ string) cid.CID {
	g.mu.Lock()
	defer g.mu.Unlock()

	cidVal, ok := g.g2c[gid]
	if !ok {
		cidVal = cid.CID(len(g.g2c))
		g.g2c[gid] = cidVal
		g.c2g[cidVal] = gid
	}
	return cidVal
}

// GID implements the [GIDToCID] interface.
func (g *gidToCIDSequential) GID(cid cid.CID) glyph.ID {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.c2g[cid]
}

// ROS implements the [GIDToCID] interface.
func (g *gidToCIDSequential) ROS() *cid.SystemInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	h := sha256.New()
	h.Write([]byte("seehuhn.de/go/pdf/font/cmap.gidToCIDSequential\n"))
	binary.Write(h, binary.BigEndian, uint64(len(g.g2c)))
	gg := slices.Sorted(maps.Keys(g.g2c))
	for _, gid := range gg {
		binary.Write(h, binary.BigEndian, gid)
		binary.Write(h, binary.BigEndian, g.g2c[gid])
	}
	sum := h.Sum(nil)

	return &cid.SystemInfo{
		Registry:   "Seehuhn",
		Ordering:   fmt.Sprintf("%x", sum[:8]),
		Supplement: 0,
	}
}

// NewGIDToCIDIdentity returns a GIDToCID which uses the GID values
// directly as CID values.
func NewGIDToCIDIdentity() GIDToCID {
	return &gidToCIDIdentity{}
}

type gidToCIDIdentity struct{}

// CID implements the [GIDToCID] interface.
func (g *gidToCIDIdentity) CID(gid glyph.ID, _ string) cid.CID {
	return cid.CID(gid)
}

// GID implements the [GIDToCID] interface.
func (g *gidToCIDIdentity) GID(c cid.CID) glyph.ID {
	// the Identity ordering has no CIDs beyond the range of a GID
	if c > math.MaxUint16 {
		return 0
	}
	return glyph.ID(c)
}

// ROS implements the [GIDToCID] interface.
func (g *gidToCIDIdentity) ROS() *cid.SystemInfo {
	return &cid.SystemInfo{
		Registry:   "Adobe",
		Ordering:   "Identity",
		Supplement: 0,
	}
}

type gidToCIDFromCMap struct {
	ros *cid.SystemInfo
	g2c map[glyph.ID]cid.CID
	c2g map[cid.CID]glyph.ID
}

// NewGIDToCIDFromCMap returns a GIDToCID for fonts written with the CMap f.
// Every CID handed out is one the CMap has a code for.  Glyphs are matched
// to CIDs via the text the CMap's character collection assigns to each CID,
// looked up in lookup.  Where several such CIDs share a glyph, each of them
// selects that glyph, while the glyph itself is written as the smallest of
// them.  CID 0 is always glyph 0, and no text is matched to it.
//
// The error wraps [fs.ErrNotExist] if no text mapping is known for the
// collection, as for Adobe-Identity.
func NewGIDToCIDFromCMap(f *File, lookup interface{ Lookup(rune) glyph.ID }) (GIDToCID, error) {
	codec, err := f.Codec()
	if err != nil {
		return nil, err
	}
	m, err := mapping.GetCIDTextMapping(f.ROS.Registry, f.ROS.Ordering)
	if err != nil {
		return nil, err
	}

	// A code mapped by both a CMap and its parent belongs to the child, so
	// the last mapping seen for a code is the one which counts.
	encodable := maps.Collect(f.All(codec))

	g2c := make(map[glyph.ID]cid.CID)
	c2g := make(map[cid.CID]glyph.ID)
	for _, cidVal := range encodable {
		if cidVal == 0 {
			continue
		}
		rr := []rune(m[cidVal])
		if len(rr) != 1 {
			continue
		}
		gid := lookup.Lookup(rr[0])
		if gid == 0 {
			continue
		}
		c2g[cidVal] = gid
		if other, ok := g2c[gid]; !ok || cidVal < other {
			g2c[gid] = cidVal
		}
	}
	return &gidToCIDFromCMap{ros: f.ROS, g2c: g2c, c2g: c2g}, nil
}

// CID implements the [GIDToCID] interface.
func (g *gidToCIDFromCMap) CID(gid glyph.ID, _ string) cid.CID {
	return g.g2c[gid]
}

// GID implements the [GIDToCID] interface.
func (g *gidToCIDFromCMap) GID(c cid.CID) glyph.ID {
	return g.c2g[c]
}

// ROS implements the [GIDToCID] interface.
func (g *gidToCIDFromCMap) ROS() *cid.SystemInfo {
	return g.ros
}

// NewGIDToCIDFromMap returns a GIDToCID for a fixed table of CIDs, for a
// character collection the library knows nothing about.  Glyphs missing
// from the table are written as CID 0.  Where several glyphs share a CID,
// the CID selects the smallest of them.  The table must map glyph 0 to
// CID 0 or leave it out, and must map no other glyph to CID 0.
func NewGIDToCIDFromMap(ros *cid.SystemInfo, g2c map[glyph.ID]cid.CID) GIDToCID {
	c2g := make(map[cid.CID]glyph.ID, len(g2c))
	for gid, c := range g2c {
		if other, ok := c2g[c]; !ok || gid < other {
			c2g[c] = gid
		}
	}
	return &gidToCIDFromCMap{ros: ros, g2c: maps.Clone(g2c), c2g: c2g}
}
