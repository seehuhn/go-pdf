package truetype

import (
	"encoding/binary"
	"errors"
	"fmt"
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

func (tt *Font) getInfo() (*fontInfo, error) {
	postInfo, err := tt.getPostInfo()
	if err != nil {
		return nil, err
	}
	hheaInfo, err := tt.getHHeaInfo()
	if err != nil {
		return nil, err
	}
	hmtx, err := tt.getHMtxInfo(hheaInfo.NumOfLongHorMetrics)
	if err != nil {
		return nil, err
	}

	os2Info, err := tt.getOS2Info()
	// The "OS/2" table is optional for TrueType fonts, but required for
	// OpenType fonts.
	if _, missingTable := err.(*table.ErrNoTable); err != nil && !missingTable {
		return nil, err
	}

	// factor for converting from TrueType FUnit to PDF glyph units
	q := 1000 / float64(tt.head.UnitsPerEm)

	info := &fontInfo{
		Width: make([]int, tt.NumGlyphs),
		FontBBox: &pdf.Rectangle{
			LLx: float64(tt.head.XMin) * q,
			LLy: float64(tt.head.YMin) * q,
			URx: float64(tt.head.XMax) * q,
			URy: float64(tt.head.YMax) * q,
		},

		IsBold:       tt.head.MacStyle&(1<<0) != 0,
		IsItalic:     tt.head.MacStyle&(1<<1) != 0,
		IsFixedPitch: postInfo.IsFixedPitch,

		ItalicAngle: postInfo.ItalicAngle,
		Ascent:      float64(hheaInfo.Ascent) * q,
		Descent:     float64(hheaInfo.Descent) * q,
		LineGap:     float64(hheaInfo.LineGap) * q,
	}

	for i := 0; i < tt.NumGlyphs; i++ {
		j := i % len(hmtx.HMetrics)
		info.Width[i] = int(float64(hmtx.HMetrics[j].AdvanceWidth)*q + 0.5)
	}

	info.GlyphExtent = make([]font.Rect, tt.NumGlyphs)
	glyf, err := tt.getGlyfInfo()
	if err != nil {
		return nil, err
	}
	for i := 0; i < tt.NumGlyphs; i++ {
		info.GlyphExtent[i].LLx = int(float64(glyf.Data[i].XMin)*q + 0.5)
		info.GlyphExtent[i].LLy = int(float64(glyf.Data[i].YMin)*q + 0.5)
		info.GlyphExtent[i].URx = int(float64(glyf.Data[i].XMax)*q + 0.5)
		info.GlyphExtent[i].URy = int(float64(glyf.Data[i].YMax)*q + 0.5)
	}

	// provisional weight values, updated below
	if info.IsBold {
		info.Weight = 700
	} else {
		info.Weight = 400
	}

	info.FontName, err = tt.getFontName()
	if err != nil {
		// TODO(voss): if FontName == "", invent a name: The name must be no
		// longer than 63 characters and restricted to the printable ASCII
		// subset, codes 33 to 126, except for the 10 characters '[', ']', '(',
		// ')', '{', '}', '<', '>', '/', '%'.
		return nil, err
	}

	if os2Info != nil {
		// If the "OS/2" table is present, Windows seems to use this table to
		// decide whether the font is bold/italic.  We follow Window's lead
		// here (overriding the values from the head table).
		info.IsBold = os2Info.V0.Selection&(1<<5) != 0
		info.IsItalic = os2Info.V0.Selection&(1<<0) != 0

		info.Weight = int(os2Info.V0.WeightClass)

		// we also override ascent, descent and linegap
		if os2Info.V0MSValid {
			var ascent, descent float64
			if os2Info.V0.Selection&(1<<7) != 0 {
				ascent = float64(os2Info.V0MS.TypoAscender)
				descent = float64(os2Info.V0MS.TypoDescender)
			} else {
				ascent = float64(os2Info.V0MS.WinAscent)
				descent = -float64(os2Info.V0MS.WinDescent)
			}
			info.Ascent = ascent * q
			info.Descent = descent * q
			info.LineGap = float64(os2Info.V0MS.TypoLineGap) * q
		}

		if os2Info.V0.Version >= 4 {
			info.CapHeight = float64(os2Info.V4.CapHeight) * q
			info.XHeight = float64(os2Info.V4.XHeight) * q
		} else {
			// TODO(voss): CapHeight may be set equal to the top of the unscaled
			// and unhinted glyph bounding box of the glyph encoded at U+0048
			// (LATIN CAPITAL LETTER H)
			info.CapHeight = 800

			// TODO(voss): XHeight may be set equal to the top of the unscaled and
			// unhinted glyph bounding box of the glyph encoded at U+0078 (LATIN
			// SMALL LETTER X).
		}
	}

	if os2Info != nil {
		switch os2Info.V0.FamilyClass >> 8 {
		case 1, 2, 3, 4, 5, 7:
			info.IsSerif = true
		case 10:
			info.IsScript = true
		}
	}

	info.CMap, err = tt.selectCmap()
	if err != nil {
		return nil, err
	}
	info.IsAdobeLatin = info.IsSubset(font.AdobeStandardLatin)

	info.Kerning, err = tt.getKernInfo(q)
	_, missingTable := err.(*table.ErrNoTable)
	if missingTable {
		// try to use GPOS instead
	} else if err != nil {
		return nil, err
	}

	GPOS, err := tt.ReadGposTable() // TODO(voss): ...
	if GPOS == nil {
		fmt.Println("GPOS:", err)
	} else {
		fmt.Println("GPOS", GPOS)
		fd, _ := tt.Header.ReadTableHead(tt.Fd, "GPOS", nil)
		xxx, err := GPOS.ReadFeatureInfo(fd, "DEU ", "latn")
		if err != nil {
			return nil, err
		}
		fmt.Println(xxx)
	}

	return info, nil
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
