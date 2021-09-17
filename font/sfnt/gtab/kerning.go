package gtab

import "seehuhn.de/go/pdf/font"

// KernInfo encodes the kerning for a pair of Glyphs.
// If Kern is less than zero, the character will be moved closer together.
type KernInfo struct {
	Left, Right font.GlyphID
	Kern        int16
}

func KerningAsLookup(kerning []KernInfo) Lookups {
	cov := make(coverage)
	nextClass := 0
	var adjust []map[font.GlyphID]*pairAdjust
	for _, k := range kerning {
		if k.Kern == 0 {
			continue
		}

		class, ok := cov[k.Left]
		if !ok {
			class = nextClass
			adjust = append(adjust, map[font.GlyphID]*pairAdjust{})
			nextClass++

			cov[k.Left] = class
		}

		adjust[class][k.Right] = &pairAdjust{
			first: &valueRecord{XAdvance: k.Kern},
		}
	}

	subtable := &gpos2_1{
		cov:    cov,
		adjust: adjust,
	}
	lookup := &LookupTable{
		Subtables: []LookupSubtable{subtable},
		Filter:    useAllGlyphs,
	}
	return Lookups{lookup}
}
