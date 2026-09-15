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

// Package cidsetup derives the glyph-to-CID mapping and the encoder of a
// composite font from the CMap it is written with.  The composite font
// writers share this code.
package cidsetup

import (
	"errors"
	"io/fs"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
)

// FromCMap chooses the GID-to-CID mapping and the encoder for a composite
// font written with the CMap f.  A nil f selects Identity-H.
//
// A CMap with mappings fixes the codes; its character collection decides
// the CIDs, or, for Identity-H and Identity-V, leaves them free.  A CMap
// without mappings is a template: codes are allocated from its code space
// as the text requires, which is supported for the UTF-8 code space.
//
// A non-nil gidToCID is used in place of the mapping derived from the CMap.
// Its character collection must be the CMap's, unless the CMap leaves the
// CIDs free.
func FromCMap(f *cmap.File, gidToCID cmap.GIDToCID, info *sfnt.Font, notdefWidth float64) (cmap.GIDToCID, cidenc.CIDEncoder, error) {
	if f == nil {
		var err error
		f, err = cmap.Predefined("Identity-H")
		if err != nil {
			return nil, nil, err
		}
	}

	hasMappings := len(f.CIDSingles)+len(f.CIDRanges) > 0 || f.Parent != nil
	if !hasMappings {
		if !f.CodeSpaceRange.Equivalent(charcode.UTF8) {
			return nil, nil, errors.New("CMap without mappings must use the UTF-8 code space")
		}
		if gidToCID == nil {
			gidToCID = cmap.NewGIDToCIDSequential()
		}
		return gidToCID, cidenc.NewCompositeUtf8(notdefWidth, f.WMode), nil
	}

	if f.ROS == nil {
		return nil, nil, errors.New("CMap has no CIDSystemInfo")
	}
	enc, err := cidenc.NewFromCMap(f, notdefWidth)
	if err != nil {
		return nil, nil, err
	}

	if gidToCID != nil {
		// The font dictionary permits any CIDSystemInfo alongside an
		// Identity CMap; every other CMap fixes the collection.
		if !isIdentity(f.ROS) && !sameCollection(gidToCID.ROS(), f.ROS) {
			return nil, nil, errors.New("GIDToCID and CMap use different character collections")
		}
		return gidToCID, enc, nil
	}

	lookup, err := info.CMapTable.GetBest()
	if err != nil {
		return nil, nil, err
	}
	gidToCID, err = cmap.NewGIDToCIDFromCMap(f, lookup)
	if errors.Is(err, fs.ErrNotExist) && isIdentity(f.ROS) {
		// The Identity CMaps do not fix the CIDs, so they can be allocated
		// as glyphs are used.
		gidToCID, err = cmap.NewGIDToCIDSequential(), nil
	}
	if err != nil {
		return nil, nil, err
	}
	return gidToCID, enc, nil
}

func isIdentity(ros *cid.SystemInfo) bool {
	return ros.Registry == "Adobe" && ros.Ordering == "Identity"
}

func sameCollection(a, b *cid.SystemInfo) bool {
	return a != nil && a.Registry == b.Registry && a.Ordering == b.Ordering
}
