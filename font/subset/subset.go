// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package subset

import (
	"errors"

	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt"
	"seehuhn.de/go/sfnt/cff"
	"seehuhn.de/go/sfnt/cmap"
	"seehuhn.de/go/sfnt/glyf"
	"seehuhn.de/go/sfnt/glyph"
)

type Glyph struct {
	// OrigGID is the glyph ID before subsetting.
	OrigGID glyph.ID

	// CID is the character identifier of the glyph in the subsetted font.
	CID type1.CID
}

// Simple constructs a subset of the font, for use in a simple PDF font.
func Simple(info *sfnt.Font, subset []Glyph) (*sfnt.Font, error) {
	if len(subset) == 0 || subset[0].OrigGID != 0 {
		return nil, errors.New("subset does not start with .notdef")
	}
	for _, g := range subset[1:] {
		if g.CID >= 256 {
			return nil, errors.New("CID out of range for simple font")
		}
	}

	res := &sfnt.Font{}
	*res = *info

	switch outlines := info.Outlines.(type) {
	case *cff.Outlines:
		o2 := &cff.Outlines{}
		pIdxMap := make(map[int]int)
		for _, g := range subset {
			o2.Glyphs = append(o2.Glyphs, outlines.Glyphs[g.OrigGID])
			oldPIdx := outlines.FDSelect(g.OrigGID)
			if _, ok := pIdxMap[oldPIdx]; !ok {
				newPIdx := len(o2.Private)
				o2.Private = append(o2.Private, outlines.Private[oldPIdx])
				pIdxMap[oldPIdx] = newPIdx
			}
		}
		if len(o2.Private) != 1 {
			return nil, errors.New("need exactly one private dict for a simple font")
		}
		o2.FDSelect = func(gid glyph.ID) int { return 0 }

		if o2.Glyphs[0].Name == "" {
			// TODO(voss): if this ever becomes a problem, we could try
			// to generate names from info.CMap.
			return nil, errors.New("need glyph names for a simple font")
		}
		o2.Encoding = make([]glyph.ID, 256)
		for subsetGid, g := range subset {
			if subsetGid == 0 {
				continue
			}
			o2.Encoding[g.CID] = glyph.ID(subsetGid)
		}

		res.Outlines = o2

	case *glyf.Outlines:
		newGid := make(map[glyph.ID]glyph.ID)
		todo := make(map[glyph.ID]bool)
		nextGid := glyph.ID(0)
		for _, g := range subset {
			gid := g.OrigGID
			newGid[gid] = nextGid
			nextGid++

			for _, gid2 := range outlines.Glyphs[gid].Components() {
				if _, ok := newGid[gid2]; !ok {
					todo[gid2] = true
				}
			}
		}
		for len(todo) > 0 {
			gid := pop(todo)
			subset = append(subset, Glyph{OrigGID: gid})
			newGid[gid] = nextGid
			nextGid++

			for _, gid2 := range outlines.Glyphs[gid].Components() {
				if _, ok := newGid[gid2]; !ok {
					todo[gid2] = true
				}
			}
		}

		o2 := &glyf.Outlines{
			Tables: outlines.Tables,
			Maxp:   outlines.Maxp,
		}
		for _, g := range subset {
			gid := g.OrigGID
			newGlyph := outlines.Glyphs[gid]
			o2.Glyphs = append(o2.Glyphs, newGlyph.FixComponents(newGid))
			o2.Widths = append(o2.Widths, outlines.Widths[gid])
			// o2.Names = append(o2.Names, outlines.Names[gid])
		}
		res.Outlines = o2

		// Use a format 4 TrueType cmap to specify the mapping from
		// character codes to glyph indices.
		//
		// TODO(voss): Is this correct/needed?
		encoding := cmap.Format4{}
		for subsetGid, g := range subset {
			if subsetGid == 0 {
				continue
			}
			encoding[uint16(g.CID)] = glyph.ID(subsetGid)
		}
		res.CMap = encoding

	default:
		panic("unexpected font type")
	}

	return res, nil
}

// CID constructs a subset of the font for use as a CID-keyed PDF font.
func CID(info *sfnt.Font, subset []Glyph, ROS *type1.CIDSystemInfo) (*sfnt.Font, error) {
	if len(subset) == 0 || subset[0].OrigGID != 0 {
		return nil, errors.New("subset does not start with .notdef")
	}
	if ROS == nil {
		return nil, errors.New("ROS cannot be nil for CID-keyed font")
	}

	res := &sfnt.Font{}
	*res = *info

	switch outlines := info.Outlines.(type) {
	case *cff.Outlines:
		o2 := &cff.Outlines{}
		pIdxMap := make(map[int]int)
		fdSel := make(map[glyph.ID]int)
		for subsetGID, g := range subset {
			o2.Glyphs = append(o2.Glyphs, outlines.Glyphs[g.OrigGID])
			oldPIdx := outlines.FDSelect(g.OrigGID)
			if _, ok := pIdxMap[oldPIdx]; !ok {
				newPIdx := len(o2.Private)
				o2.Private = append(o2.Private, outlines.Private[oldPIdx])
				pIdxMap[oldPIdx] = newPIdx
			}
			fdSel[glyph.ID(subsetGID)] = pIdxMap[oldPIdx]
		}
		o2.FDSelect = func(gid glyph.ID) int { return fdSel[gid] }

		cidEqualsGid := true
		for subsetGid, g := range subset {
			if int(g.CID) != subsetGid && subsetGid != 0 {
				cidEqualsGid = false
				break
			}
		}

		if !cidEqualsGid || len(pIdxMap) > 1 || o2.Glyphs[0].Name == "" {
			// use a CID-keyed font only when necessary
			o2.ROS = ROS
			o2.Gid2Cid = make([]type1.CID, len(subset))
			for subsetGid, g := range subset {
				if subsetGid == 0 {
					continue
				}
				o2.Gid2Cid[subsetGid] = g.CID
			}
		}

		res.Outlines = o2

	case *glyf.Outlines:
		newGid := make(map[glyph.ID]glyph.ID)
		todo := make(map[glyph.ID]bool)
		nextGid := glyph.ID(0)
		for _, g := range subset {
			gid := g.OrigGID
			newGid[gid] = nextGid
			nextGid++

			for _, gid2 := range outlines.Glyphs[gid].Components() {
				if _, ok := newGid[gid2]; !ok {
					todo[gid2] = true
				}
			}
		}
		for len(todo) > 0 {
			gid := pop(todo)
			subset = append(subset, Glyph{OrigGID: gid})
			newGid[gid] = nextGid
			nextGid++

			for _, gid2 := range outlines.Glyphs[gid].Components() {
				if _, ok := newGid[gid2]; !ok {
					todo[gid2] = true
				}
			}
		}

		o2 := &glyf.Outlines{
			Tables: outlines.Tables,
			Maxp:   outlines.Maxp,
		}
		for _, g := range subset {
			gid := g.OrigGID
			newGlyph := outlines.Glyphs[gid]
			o2.Glyphs = append(o2.Glyphs, newGlyph.FixComponents(newGid))
			o2.Widths = append(o2.Widths, outlines.Widths[gid])
			// o2.Names = append(o2.Names, outlines.Names[gid])
		}
		res.Outlines = o2

		// The mapping from CIDs to GIDs is specified in the CIDToGIDMap entry
		// in the CIDFont dictionary.
		res.CMap = nil

	default:
		panic("unexpected font type")
	}

	return res, nil
}

func pop(todo map[glyph.ID]bool) glyph.ID {
	for key := range todo {
		delete(todo, key)
		return key
	}
	panic("empty map")
}
