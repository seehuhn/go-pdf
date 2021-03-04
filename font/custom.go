package font

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

func CustomEncoding(runes []rune) Encoding {
	// Find the standard encoding with the largest overlap in character set.
	matches := map[string]int{}
	for _, r := range runes {
		for name, enc := range stdEncs {
			_, ok := enc.Encode(r)
			if ok {
				matches[name]++
			}
		}
	}
	bestCount := -1
	var bestEnc Encoding
	for name, enc := range stdEncs {
		if count := matches[name]; count > bestCount {
			bestCount = count
			bestEnc = enc
		}
	}

	// Keep the glyphs present in both character sets in their standard
	// position.
	from := make([]rune, 256)
	for c := range from {
		from[c] = noRune
	}
	var missing []rune
	for _, r := range runes {
		c, ok := bestEnc.Encode(r)
		if ok {
			from[c] = r
		} else {
			missing = append(missing, r)
		}
	}

	// Put the remaining characters into the gaps.
	for ci := 32; ci < 256 && len(missing) > 0; ci++ {
		c := byte(ci)
		if from[c] != noRune {
			continue
		}
		from[c] = missing[0]
		missing = missing[1:]
	}
	for ci := 31; ci >= 0 && len(missing) > 0; ci-- {
		c := byte(ci)
		if from[c] != noRune {
			continue
		}
		from[c] = missing[0]
		missing = missing[1:]
	}

	// Construct the reverse map.
	to := make(map[rune]byte)
	for i, r := range from {
		if r == noRune || rune(i) == r {
			continue
		}
		to[r] = byte(i)
	}

	return &tables{to, from}
}

func Describe(enc Encoding) pdf.Object {
	matches := map[string]int{}
	for ci := 0; ci < 256; ci++ {
		c := byte(ci)
		for name, stdEnc := range stdEncs {
			if enc.Decode(c) == stdEnc.Decode(c) {
				matches[name]++
			}
		}
	}
	bestCount := -1
	bestName := ""
	var bestEnc Encoding
	for name, enc := range stdEncs {
		if count := matches[name]; count > bestCount {
			bestCount = count
			bestName = name
			bestEnc = enc
		}
	}
	if bestCount == 256 {
		return pdf.Name(bestName)
	}

	var diff pdf.Array
	pos := -1
	for i := 0; i < 256; i++ {
		c := byte(i)
		rOrig := bestEnc.Decode(c)
		rNew := enc.Decode(c)
		if rOrig == rNew {
			continue
		}
		if pos != i {
			diff = append(diff, pdf.Integer(i))
			pos = i
		}
		var name string
		if rNew == noRune {
			name = ".notdef"
		} else {
			name = fmt.Sprintf("u%04X", rNew)
		}
		diff = append(diff, pdf.Name(name))
		pos++
	}

	res := pdf.Dict{
		"Type":         pdf.Name("Encoding"),
		"BaseEncoding": pdf.Name(bestName),
		"Differences":  diff,
	}
	return res
}
