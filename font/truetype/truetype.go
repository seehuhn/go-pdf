package truetype

import (
	"encoding/binary"
	"errors"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/truetype/table"
)

// Font describes a TrueType font file.
type Font struct {
	Fd     *os.File
	Header *table.Header

	head      *table.Head
	NumGlyphs int
}

// HasTables returns true, if all the given tables are present in the font.
func (tt *Font) HasTables(names ...string) bool {
	for _, name := range names {
		if tt.Header.Find(name) == nil {
			return false
		}
	}
	return true
}

// IsTrueType checks whether all required tables for a TrueType font are
// present.
func (tt *Font) IsTrueType() bool {
	return tt.HasTables("cmap", "glyf", "head", "hhea", "hmtx", "loca", "maxp", "name", "post")
}

// IsOpenType checks whether all required tables for an OpenType font are
// present.
func (tt *Font) IsOpenType() bool {
	if !tt.HasTables("cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post") {
		return false
	}
	if tt.HasTables("glyf", "loca") || tt.HasTables("CFF ") {
		return true
	}
	return false
}

// TODO(voss): merge this type with truetype.Font
type fontInfo struct {
	FontName string

	CMap map[rune]font.GlyphIndex

	GlyphExtent []font.Rect
	Width       []int
	Kerning     map[font.GlyphPair]int

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
func (info *fontInfo) IsSubset(charset map[rune]bool) bool {
	for r := range info.CMap {
		if !charset[r] {
			return false
		}
	}
	return true
}

// IsSuperset returns true if the font includes all runes of the
// given character set.
func (info *fontInfo) IsSuperset(charset map[rune]bool) bool {
	for r, ok := range charset {
		if ok && info.CMap[r] == 0 {
			return false
		}
	}
	return true
}

// Open loads a TrueType font into memory.
func Open(fname string) (*Font, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	tt := &Font{
		Header: &table.Header{},
		Fd:     fd,
	}

	err = binary.Read(fd, binary.BigEndian, &tt.Header.Offsets)
	if err != nil {
		return nil, err
	}
	tt.Header.Records = make([]table.Record, tt.Header.Offsets.NumTables)
	err = binary.Read(fd, binary.BigEndian, &tt.Header.Records)
	if err != nil {
		return nil, err
	}

	scalerType := tt.Header.Offsets.ScalerType
	if scalerType != 0x00010000 && scalerType != 0x4F54544F {
		return nil, errors.New("unsupported font type")
	}

	maxp, err := tt.getMaxpInfo()
	if err != nil {
		return nil, err
	}
	if maxp.NumGlyphs < 2 {
		// glyph index 0 denotes a missing character
		return nil, errors.New("no glyphs found")
	}
	tt.NumGlyphs = int(maxp.NumGlyphs)

	tt.head, err = tt.getHeadInfo()
	if err != nil {
		return nil, err
	}

	return tt, nil
}

// Close frees all resources associated with the font.  The Font object
// cannot be used any more after Close() has been called.
func (tt *Font) Close() error {
	return tt.Fd.Close()
}

func (tt *Font) selectCmap() (map[rune]font.GlyphIndex, error) {
	cmapTable, cmapFd, err := tt.getCmapInfo()
	if err != nil {
		return nil, err
	}

	unicode := func(idx int) rune {
		return rune(idx)
	}
	macRoman := func(idx int) rune {
		return macintosh[idx]
	}
	candidates := []struct {
		PlatformID uint16
		EncodingID uint16
		IdxToRune  func(int) rune
	}{
		{3, 10, unicode}, // full unicode
		{0, 4, unicode},
		{3, 1, unicode}, // BMP
		{0, 3, unicode},
		{1, 0, macRoman}, // vintage Apple format
	}

	for _, cand := range candidates {
		subTable := cmapTable.Find(cand.PlatformID, cand.EncodingID)
		if subTable == nil {
			continue
		}

		cmap, err := tt.load(cmapFd, subTable, cand.IdxToRune)
		if err != nil {
			continue
		}

		return cmap, nil
	}
	return nil, errors.New("unsupported character encoding")
}
