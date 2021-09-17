package truetype

import (
	"sort"

	"seehuhn.de/go/pdf/font"
)

const subsetModulus = 26 * 26 * 26 * 26 * 26 * 26

func getSubsetTag(gg []font.GlyphID, numGlyphs uint16) string {
	sort.Slice(gg, func(i, j int) bool { return gg[i] < gg[j] })

	// mix the information into a single uint32
	X := uint32(numGlyphs)
	for _, g := range gg {
		// 11 is a prime such that subsetModulus * 13 < 1<<32.  We avoid the
		// value 13 here, since this is a divisor of 26.
		X = (X*11 + uint32(g)) % subsetModulus
	}

	// convert to a string of six capital letters
	var buf [6]byte
	for i := range buf {
		buf[i] = 'A' + byte(X%26)
		X /= 26
	}
	return string(buf[:])
}
