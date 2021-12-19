package cff

import "seehuhn.de/go/pdf/font"

// Subset returns a copy of the font with only the glyphs in the given
// subset.
func (cff *Font) Subset(subset []font.GlyphID) *Font {
	out := &Font{
		FontName:    cff.FontName, // TODO(voss): subset tag needed?
		topDict:     cff.topDict,  // TODO(voss): any updates needed?
		gsubrs:      cff.gsubrs,
		privateDict: cff.privateDict,
		subrs:       cff.subrs,

		gid2cid: append([]font.GlyphID{}, subset...),
	}

	out.charStrings = nil
	out.glyphNames = nil
	for _, gid := range subset {
		out.charStrings = append(out.charStrings, cff.charStrings[gid])
		out.glyphNames = append(out.glyphNames, cff.glyphNames[gid])
	}

	// TODO(voss): prune unused subrs

	return out
}
