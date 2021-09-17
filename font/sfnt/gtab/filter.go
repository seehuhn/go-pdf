// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package gtab

import (
	"seehuhn.de/go/pdf/font"
)

// KeepGlyphFn is used to drop ignored characters in lookups with non-zero
// lookup flags.  Functions of this type return true if the glyph should be
// used, and false if the glyph should be ignored.
type KeepGlyphFn func(font.GlyphID) bool

// Next returns the index of the next glyph to use after pos.  If pos is
// the last used glyph in the sequence, -1 is returned instead.
func (keep KeepGlyphFn) Next(seq []font.Glyph, pos int) int {
	for i := pos + 1; i < len(seq); i++ {
		if keep(seq[i].Gid) {
			return i
		}
	}
	return -1
}

// Prev returns the index of the previous glyph to use before pos.  If pos is
// the first used glyph in the sequence, -1 is returned instead.
func (keep KeepGlyphFn) Prev(seq []font.Glyph, pos int) int {
	for i := pos - 1; i >= 0; i-- {
		if keep(seq[i].Gid) {
			return i
		}
	}
	return -1
}

const (
	ignoreBaseGlyphs       uint16 = 0x0002
	ignoreLigatures        uint16 = 0x0004
	ignoreMarks            uint16 = 0x0008
	useMarkFilteringSet    uint16 = 0x0010
	markAttachmentTypeMask uint16 = 0xFF00
)

func useAllGlyphs(font.GlyphID) bool { return true }

func (g *GTab) makeFilter(lookupFlag uint16) KeepGlyphFn {
	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-table

	if g.gdef == nil {
		return useAllGlyphs
	}

	if lookupFlag&ignoreMarks != 0 {
		lookupFlag &= 0x000F
	}

	var skip glyphClass
	if lookupFlag&ignoreBaseGlyphs != 0 {
		skip |= glyphClassBase
	}
	if lookupFlag&ignoreLigatures != 0 {
		skip |= glyphClassLigature
	}
	if lookupFlag&ignoreMarks != 0 {
		skip |= glyphClassMark
	}
	if g.gdef.GlyphClassDef == nil {
		skip = 0
	}

	if g.gdef.MarkAttachClass == nil {
		lookupFlag &= ^markAttachmentTypeMask
	}

	filterFunc := useAllGlyphs
	if lookupFlag&useMarkFilteringSet != 0 {
		panic("not implemented")
	} else if lookupFlag&markAttachmentTypeMask != 0 {
		attachmentType := int(lookupFlag & markAttachmentTypeMask >> 8)
		filterFunc = func(gid font.GlyphID) bool {
			if g.gdef.GlyphClassDef[gid]&skip != 0 {
				return false
			}
			if g.gdef.GlyphClassDef[gid] == glyphClassMark &&
				g.gdef.MarkAttachClass[gid] != attachmentType {
				return false
			}
			return true
		}
	} else if skip != 0 {
		filterFunc = func(gid font.GlyphID) bool {
			return g.gdef.GlyphClassDef[gid]&skip == 0
		}
	}

	return filterFunc
}
