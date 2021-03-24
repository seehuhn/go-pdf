package font

import "seehuhn.de/go/pdf"

type Info struct {
	FontName string
	Type     string

	BBox  *pdf.Rectangle
	Width map[byte]float64
	Kern  []Rect

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
