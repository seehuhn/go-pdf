// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"errors"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/glyphdata"
)

// repairROS makes the character collection and the CMap of a CIDFont
// consistent.  A missing character collection is taken from the CMap, or set
// to Adobe-Identity-0.  The predefined Identity-H and Identity-V are
// compatible with every character collection.  If the two are otherwise
// missing or incompatible, a predefined CMap determines the character
// collection, since its CIDs are those of its own collection.  An embedded
// CMap is replaced by a copy with the font's character collection.
//
// A non-embedded font cannot use Identity-H or Identity-V.  If such a font
// has a standard character collection, the identity mapping gives CIDs of
// that collection, and the Identity CMap is replaced by a copy which
// carries the font's character collection.
func repairROS(ros **cid.SystemInfo, cm **cmap.File, external bool) {
	c := *cm
	if *ros == nil {
		if c.ROS != nil {
			*ros = c.ROS
		} else {
			*ros = &cid.SystemInfo{
				Registry:   "Adobe",
				Ordering:   "Identity",
				Supplement: 0,
			}
		}
	}

	if c.IsIdentity() {
		if external && cmap.IsStandardCollection(*ros) {
			c = c.Clone()
			c.Name = (*ros).Registry + "-" + (*ros).Ordering + "-" + c.Name
			c.ROS = *ros
			*cm = c
		}
		return
	}
	if c.ROS != nil && (*ros).Registry == c.ROS.Registry &&
		(*ros).Ordering == c.ROS.Ordering {
		return
	}

	if c.IsPredefined() && c.ROS != nil {
		*ros = c.ROS
		return
	}
	c = c.Clone()
	c.ROS = *ros
	*cm = c
}

// checkExternalCollection rejects a non-embedded font whose CMap does not use
// a standard character collection.  The CIDs of other collections, including
// those of Identity-H and Identity-V, refer to the glyphs of one particular
// font program, so they cannot select glyphs from a substitute font.
func checkExternalCollection(fontFile *glyphdata.Stream, cm *cmap.File) error {
	if fontFile == nil && !cmap.IsStandardCollection(cm.ROS) {
		return &pdf.MalformedFileError{
			Err: errors.New("external font needs a standard character collection"),
		}
	}
	return nil
}
