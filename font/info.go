package font

import "seehuhn.de/go/pdf"

type Info struct {
	FontName string

	CMap map[rune]GlyphIndex

	GlyphExtent []Rect
	Width       []int

	FontBBox *pdf.Rectangle

	IsAdobeLatin bool // is a subset of the Adobe standard Latin character set
	IsBold       bool
	IsFixedPitch bool
	IsItalic     bool
	IsScript     bool // glyphs resemble cursive handwriting
	IsSerif      bool

	Weight int // 300 = light, 400 = regular, 700 = bold

	ItalicAngle float64
	Ascent      float64
	Descent     float64
	LineGap     float64
	CapHeight   float64
	XHeight     float64
}

// IsSubset returns true if the font includes only runes from the
// given character set.
func (info *Info) IsSubset(charset map[rune]bool) bool {
	for r := range info.CMap {
		if !charset[r] {
			return false
		}
	}
	return true
}

// IsSuperset returns true if the font includes all runes of the
// given character set.
func (info *Info) IsSuperset(charset map[rune]bool) bool {
	for r, ok := range charset {
		if ok && info.CMap[r] == 0 {
			return false
		}
	}
	return true
}
