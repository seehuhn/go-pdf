package gtab

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gdef"
)

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
	attachClass := uint16((flags & LookupMarkAttachmentTypeMask) >> 8)
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
			sel |= filterLigatures
		}
	} else if flags&LookupUseMarkFilteringSet != 0 {
		// If a mark filtering set is specified, this supersedes any mark
		// attachment type indication in the lookup flag.
		if int(meta.MarkFilteringSet) < len(gdefTable.MarkGlyphSets) {
			sel |= filterMarksFromSet
			markGlyphSet = gdefTable.MarkGlyphSets[meta.MarkFilteringSet]
		}
	} else if attachClass != 0 && gdefTable.MarkAttachClass != nil {
		if gdefTable.MarkAttachClass != nil {
			sel |= filterAttachClass
		}
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
			return false
		}
		if sel&filterAttachClass != 0 && gdefTable.MarkAttachClass[gid] != attachClass {
			return false
		}
		return true
	}

	return filterFunc
}

func useAllGlyphs(font.GlyphID) bool { return true }

// LookupFlags contains bits which modify application of a lookup to a glyph string.
// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookupFlags
type LookupFlags uint16

// Bit values for LookupFlag.
const (
	LookupRightToLeft            LookupFlags = 0x0001
	LookupIgnoreBaseGlyphs       LookupFlags = 0x0002
	LookupIgnoreLigatures        LookupFlags = 0x0004
	LookupIgnoreMarks            LookupFlags = 0x0008
	LookupUseMarkFilteringSet    LookupFlags = 0x0010
	LookupMarkAttachmentTypeMask LookupFlags = 0xFF00
)
