package cff

import "seehuhn.de/go/pdf/font"

// Subset returns a copy of the font with only the glyphs in the given
// subset.
func (cff *Font) Subset(subset []font.GlyphID) *Font {
	out := &Font{
		FontName:    cff.FontName, // TODO(voss): subset tag needed?
		topDict:     map[dictOp][]interface{}{},
		strings:     &cffStrings{},
		gsubrs:      cff.gsubrs.Copy(),
		privateDict: map[dictOp][]interface{}{},
		subrs:       cff.subrs.Copy(),
	}

	out.charStrings = nil
	out.glyphNames = nil
	for _, gid := range subset {
		out.charStrings = append(out.charStrings, cff.charStrings[gid])
		s, _ := cff.strings.get(cff.glyphNames[gid])
		newSID := out.strings.lookup(s)
		out.glyphNames = append(out.glyphNames, newSID)
	}

	stringKeys := []dictOp{
		opVersion,
		opNotice,
		opCopyright,
		opFullName,
		opFamilyName,
		opWeight,
		opPostScript,
		opBaseFontName,
		opFontName,
	}
	for _, key := range stringKeys {
		oldSID, ok := cff.topDict.getSID(key)
		if !ok {
			continue
		}
		s, _ := cff.strings.get(cff.glyphNames[oldSID])
		newSID := out.strings.lookup(s)
		out.topDict[key] = []interface{}{int32(newSID)}
	}
	if x, ok := out.topDict[opROS]; ok && len(x) == 3 {
		if registry, ok := x[0].(sid); ok {
			s, _ := cff.strings.get(registry)
			x[0] = int32(out.strings.lookup(s))
		}
		if supplement, ok := x[1].(sid); ok {
			s, _ := cff.strings.get(supplement)
			x[0] = int32(out.strings.lookup(s))
		}
	}

	panic("not implemented")
}
