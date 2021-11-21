package cff

import "seehuhn.de/go/pdf/font"

// Subset returns a copy of the font with only the glyphs in the given
// subset.
func (cff *Font) Subset(subset []font.GlyphID) *Font {
	out := cff.Copy()

	newStrings := make(map[string]sid)
	for i, s := range stdStrings {
		newStrings[s] = sid(i)
	}
	out.strings = nil
	allocString := func(orig sid) sid {
		s := cff.strings.get(orig)
		if new, ok := newStrings[s]; ok {
			return new
		}
		new := sid(len(out.strings)) + nStdString
		out.strings = append(out.strings, s)
		newStrings[s] = new
		return new
	}

	out.charStrings = nil
	out.glyphNames = nil
	for _, gid := range subset {
		out.charStrings = append(out.charStrings, cff.charStrings[gid])
		sid := allocString(cff.glyphNames[gid])
		out.glyphNames = append(out.glyphNames, sid)
	}

	stringKeys := []dictOp{opVersion,
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
		x := out.topDict[key]
		if len(x) != 1 {
			delete(out.topDict, key)
			continue
		}
		val, ok := x[0].(sid)
		if !ok {
			continue
		}
		out.topDict[key] = []interface{}{allocString(val)}
	}
	if x, ok := out.topDict[opROS]; ok && len(x) == 3 {
		if registry, ok := x[0].(sid); ok {
			x[0] = allocString(registry)
		}
		if supplement, ok := x[1].(sid); ok {
			x[1] = allocString(supplement)
		}
	}

	return out
}
