// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
)

// The following table shows the frequency of different lookupflag bits
// in the lookup tables in the fonts on my laptop.
//
// count   |  base ligature marks filtset attchtype
// --------+----------------------------------------
// 104671  |    .      .      .      .      .
//   7317  |    .      .      X      .      .
//   4602  |    .      .      .      .      X
//    876  |    .      .      .      X      .
//    194  |    X      X      .      .      .
//     80  |    X      X      .      .      X
//     58  |    .      X      .      .      .
//     36  |    X      .      .      .      .
//     32  |    .      X      X      .      .
//     20  |    X      .      .      .      X
//      6  |    X      .      .      X      .
//      6  |    .      .      X      .      X
//      4  |    X      X      .      X      .

// KeepGlyphFn is used to drop ignored characters in lookups with non-zero
// lookup flags.  Functions of this type return true if the glyph should be
// used, and false if the glyph should be ignored.
type KeepGlyphFn func(font.GlyphID) bool

// MakeFilter returns a function which filters glyphs according to the
// lookup flags.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookupFlags
func MakeFilter(meta *LookupMetaInfo, gdefTable *gdef.Table) KeepGlyphFn {
	if gdefTable == nil {
		return useAllGlyphs
	}

	flags := meta.LookupFlag
	markAttachType := uint16((flags & LookupMarkAttachTypeMask) >> 8)
	var markGlyphSet coverage.Table

	type filterSel int
	const (
		filterBase filterSel = 1 << iota
		filterLigatures
		filterAllMarks
		filterMarksFromSet
		filterAttachClass
	)
	var sel filterSel
	if flags&LookupIgnoreBaseGlyphs != 0 && gdefTable.GlyphClass != nil {
		sel |= filterBase
	}
	if flags&LookupIgnoreLigatures != 0 && gdefTable.GlyphClass != nil {
		sel |= filterLigatures
	}
	if flags&LookupIgnoreMarks != 0 {
		// If the IGNORE_MARKS bit is set, this supersedes any mark filtering set
		// or mark attachment type indications.
		if gdefTable.GlyphClass != nil {
			sel |= filterAllMarks
		}
	} else if flags&LookupUseMarkFilteringSet != 0 {
		// If a mark filtering set is specified, this supersedes any mark
		// attachment type indication in the lookup flag.
		if int(meta.MarkFilteringSet) < len(gdefTable.MarkGlyphSets) {
			sel |= filterMarksFromSet
			markGlyphSet = gdefTable.MarkGlyphSets[meta.MarkFilteringSet]
		}
	} else if markAttachType != 0 && gdefTable.MarkAttachClass != nil {
		sel |= filterAttachClass
	}

	if sel == 0 {
		return useAllGlyphs
	}

	filterFunc := func(gid font.GlyphID) bool {
		if sel&filterBase != 0 && gdefTable.GlyphClass[gid] == gdef.GlyphClassBase {
			return false
		}
		if sel&filterLigatures != 0 && gdefTable.GlyphClass[gid] == gdef.GlyphClassLigature {
			return false
		}
		if sel&filterAllMarks != 0 && gdefTable.GlyphClass[gid] == gdef.GlyphClassMark {
			return false
		}
		if sel&filterMarksFromSet != 0 && !markGlyphSet.Contains(gid) {
			// TODO(voss): does this only apply to mark glyphs?
			return false
		}
		if sel&filterAttachClass != 0 && gdefTable.MarkAttachClass[gid] != markAttachType {
			// TODO(voss): does this only apply to mark glyphs?
			return false
		}
		return true
	}

	return filterFunc
}

func useAllGlyphs(font.GlyphID) bool { return true }
