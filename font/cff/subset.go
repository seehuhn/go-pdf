package cff

import "seehuhn.de/go/pdf/font"

// Subset returns a copy of the font with only the glyphs in the given
// subset.
func (cff *Font) Subset(subset []font.GlyphID) *Font {
	out := &Font{
		FontName:    cff.FontName, // TODO(voss): subset tag needed?
		topDict:     map[dictOp][]interface{}{},
		gsubrs:      cff.gsubrs.Copy(),
		privateDict: map[dictOp][]interface{}{},
		subrs:       cff.subrs.Copy(),
	}

	out.charStrings = nil
	out.glyphNames = nil
	for _, gid := range subset {
		out.charStrings = append(out.charStrings, cff.charStrings[gid])
		out.glyphNames = append(out.glyphNames, cff.glyphNames[gid])
	}

	panic("not implemented")
}
