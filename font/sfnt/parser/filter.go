package parser

import (
	"seehuhn.de/go/pdf/font"
)

type filter func(font.GlyphID) bool

const (
	ignoreBaseGlyphs       uint16 = 0x0002
	ignoreLigatures        uint16 = 0x0004
	ignoreMarks            uint16 = 0x0008
	useMarkFilteringSet    uint16 = 0x0010
	markAttachmentTypeMask uint16 = 0xFF00
)

func (g *gTab) makeFilter(lookupFlag uint16) filter {
	// https://docs.microsoft.com/en-us/typography/opentype/spec/chapter2#lookup-table

	if g.gdef == nil {
		return nil
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

	var filterFunc filter
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
