// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package cidsetup derives the glyph-to-CID mapping and the encoder of a
// composite font from the CMap it is written with, and the glyph metrics
// of the CIDFont dictionary from the encoder.  The composite font writers
// share this code.
package cidsetup

import (
	"errors"
	"io/fs"
	"math"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
)

// FromCMap chooses the GID-to-CID mapping and the encoder for a composite
// font written with the CMap f, which must be non-nil.
//
// A CMap with mappings fixes the codes; its character collection decides
// the CIDs, or, for Identity-H and Identity-V, leaves them free.  A CMap
// without mappings is a template: codes are allocated from its code space
// as the text requires, which is supported for the UTF-8 code space.
//
// A non-nil gidToCID is used in place of the mapping derived from the CMap.
// Its character collection must be the CMap's, unless the CMap leaves the
// CIDs free.
//
// CID 0 is always glyph 0, and the encoder gives it the metrics of glyph 0
// of info.  A non-nil gidToCID which breaks this rule is rejected.
func FromCMap(f *cmap.File, gidToCID cmap.GIDToCID, info *sfnt.Font) (cmap.GIDToCID, cidenc.CIDEncoder, error) {
	hasMappings := len(f.CIDSingles)+len(f.CIDRanges) > 0 || f.Parent != nil
	if !hasMappings {
		if !f.CodeSpaceRange.Equivalent(charcode.UTF8) {
			return nil, nil, errors.New("CMap without mappings must use the UTF-8 code space")
		}
		if gidToCID == nil {
			gidToCID = cmap.NewGIDToCIDSequential()
		}
		if err := checkNotdef(gidToCID); err != nil {
			return nil, nil, err
		}
		notdefWidth, notdefV := NotdefMetrics(info)
		enc := cidenc.NewCompositeUtf8(gidToCID.ROS(), notdefWidth, notdefV, f.WMode)
		return gidToCID, enc, nil
	}

	if f.ROS == nil {
		return nil, nil, errors.New("CMap has no CIDSystemInfo")
	}

	if gidToCID != nil {
		// The font dictionary permits any CIDSystemInfo alongside an
		// Identity CMap; every other CMap fixes the collection.
		if !isIdentity(f.ROS) && !sameCollection(gidToCID.ROS(), f.ROS) {
			return nil, nil, errors.New("GIDToCID and CMap use different character collections")
		}
	} else {
		lookup, err := info.CMapTable.GetBest()
		if err != nil {
			return nil, nil, err
		}
		gidToCID, err = cmap.NewGIDToCIDFromCMap(f, lookup)
		if errors.Is(err, fs.ErrNotExist) && isIdentity(f.ROS) {
			// The Identity CMaps do not fix the CIDs, so they can be
			// allocated as glyphs are used.
			gidToCID, err = cmap.NewGIDToCIDSequential(), nil
		}
		if err != nil {
			return nil, nil, err
		}
	}

	if err := checkNotdef(gidToCID); err != nil {
		return nil, nil, err
	}

	// The font dictionary is written with the character collection of
	// gidToCID, which decides the text a CID implies.
	notdefWidth, notdefV := NotdefMetrics(info)
	enc, err := cidenc.NewFromCMap(f, gidToCID.ROS(), notdefWidth, notdefV)
	if err != nil {
		return nil, nil, err
	}
	return gidToCID, enc, nil
}

// checkNotdef checks that CID 0 and glyph 0 correspond in gidToCID.
func checkNotdef(gidToCID cmap.GIDToCID) error {
	if gidToCID.GID(0) != 0 || gidToCID.CID(0, "") != 0 {
		return errors.New("GIDToCID does not map CID 0 and glyph 0 to each other")
	}
	return nil
}

// NotdefMetrics returns the width and the vertical metrics of glyph 0 of
// info, which is CID 0, rounded to integer PDF glyph space units, as the
// composite font writers store glyph metrics in the encoder.
func NotdefMetrics(info *sfnt.Font) (float64, cidenc.VMetrics) {
	return math.Round(info.GlyphWidthPDF(0)), GlyphVMetrics(info, 0)
}

func isIdentity(ros *cid.SystemInfo) bool {
	return ros.Registry == "Adobe" && ros.Ordering == "Identity"
}

func sameCollection(a, b *cid.SystemInfo) bool {
	return a != nil && a.Registry == b.Registry && a.Ordering == b.Ordering
}
