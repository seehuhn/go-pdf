package truetype

import "seehuhn.de/go/pdf/font"

// IsAdobeStandardLatin returns true if the font uses the Adobe standard Latin
// character set or a subset of it.
func (tt *Font) IsAdobeStandardLatin() bool {
	for _, runes := range tt.CMap {
		for _, r := range runes {
			if !font.IsAdobeStandardLatin[r] {
				return false
			}
		}
	}
	return true
}
