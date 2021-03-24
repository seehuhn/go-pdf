package truetype

// IsSubset returns true if the font includes only runes from the
// given character set.
func (tt *Font) IsSubset(charset map[rune]bool) bool {
	for _, runes := range tt.CMap {
		for _, r := range runes {
			if !charset[r] {
				return false
			}
		}
	}
	return true
}

// IsSuperset returns true if the font includes all runes of the
// given character set.
func (tt *Font) IsSuperset(charset map[rune]bool) bool {
	seen := make(map[rune]bool)
	for _, runes := range tt.CMap {
		for _, r := range runes {
			seen[r] = true
		}
	}

	for r, ok := range charset {
		if ok && !seen[r] {
			return false
		}
	}
	return true
}
