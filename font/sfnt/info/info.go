// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package info

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/parser"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/locale"
)

type Info struct {
	CMap     map[rune]font.GlyphID
	FontName string
	Flags    font.Flags

	GlyphUnits  int
	Ascent      int // Ascent in glyph coordinate units
	Descent     int // Descent in glyph coordinate units, as a negative number
	CapHeight   int
	ItalicAngle float64

	GlyphExtent []font.Rect
	Width       []int

	GSUB, GPOS parser.Lookups
	KernInfo   map[font.GlyphPair]int
}

func GetInfo(tt *sfnt.Font, loc *locale.Locale) (*Info, error) {
	hheaInfo, err := tt.GetHHeaInfo()
	if err != nil {
		return nil, err
	}

	os2Info, err := tt.GetOS2Info()
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}

	hmtx, err := tt.GetHMtxInfo(hheaInfo.NumOfLongHorMetrics)
	if err != nil {
		return nil, err
	}

	glyf, err := tt.GetGlyfInfo()
	if err != nil {
		return nil, err
	}

	postInfo, err := tt.GetPostInfo()
	if err != nil {
		return nil, err
	}

	fontName, err := tt.GetFontName()
	if err != nil {
		// TODO(voss): if FontName == "", invent a name: The name must be no
		// longer than 63 characters and restricted to the printable ASCII
		// subset, codes 33 to 126, except for the 10 characters '[', ']', '(',
		// ')', '{', '}', '<', '>', '/', '%'.
		return nil, err
	}

	cmap, err := tt.SelectCMap()
	if err != nil {
		return nil, err
	}

	GlyphExtent := make([]font.Rect, tt.NumGlyphs)
	for i := 0; i < tt.NumGlyphs; i++ {
		GlyphExtent[i].LLx = int(glyf.Data[i].XMin)
		GlyphExtent[i].LLy = int(glyf.Data[i].YMin)
		GlyphExtent[i].URx = int(glyf.Data[i].XMax)
		GlyphExtent[i].URy = int(glyf.Data[i].YMax)
	}

	Width := make([]int, tt.NumGlyphs)
	for i := 0; i < tt.NumGlyphs; i++ {
		Width[i] = int(hmtx.GetAdvanceWidth(i))
	}

	Ascent := int(hheaInfo.Ascent)
	Descent := int(hheaInfo.Descent)
	// LineGap := int(hheaInfo.LineGap)
	if os2Info != nil && os2Info.V0MSValid {
		if os2Info.V0.Selection&(1<<7) != 0 {
			Ascent = int(os2Info.V0MS.TypoAscender)
			Descent = int(os2Info.V0MS.TypoDescender)
		} else {
			Ascent = int(os2Info.V0MS.WinAscent)
			Descent = -int(os2Info.V0MS.WinDescent)
		}
		// LineGap = int(os2Info.V0MS.TypoLineGap)
	}

	var capHeight int
	if os2Info != nil && os2Info.V0.Version >= 4 {
		capHeight = int(os2Info.V4.CapHeight)
	} else if H, ok := cmap['H']; ok {
		// CapHeight may be set equal to the top of the unscaled and unhinted
		// glyph bounding box of the glyph encoded at U+0048 (LATIN CAPITAL
		// LETTER H)
		capHeight = GlyphExtent[H].URy
	} else {
		capHeight = 800
	}

	pars := parser.New(tt)
	gsub, err := pars.ReadGsubTable(loc)
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	gpos, err := pars.ReadGposTable(loc)
	var kernInfo map[font.GlyphPair]int
	if table.IsMissing(err) { // if no GPOS table is found ...
		kernInfo, err = tt.ReadKernInfo()
	}
	if err != nil { // error from either ReadGposTable() or ReadKernInfo()
		return nil, err
	}

	var flags font.Flags
	if os2Info != nil {
		switch os2Info.V0.FamilyClass >> 8 {
		case 1, 2, 3, 4, 5, 7:
			flags |= font.FlagSerif
		case 10:
			flags |= font.FlagScript
		}
	}
	if postInfo.IsFixedPitch {
		flags |= font.FlagFixedPitch
	}
	IsItalic := tt.Head.MacStyle&(1<<1) != 0
	if os2Info != nil {
		// If the "OS/2" table is present, Windows seems to use this table to
		// decide whether the font is bold/italic.  We follow Window's lead
		// here (overriding the values from the head table).
		IsItalic = os2Info.V0.Selection&(1<<0) != 0
	}
	if IsItalic {
		flags |= font.FlagItalic
	}

	if isSubset(cmap, font.AdobeStandardLatin) {
		flags |= font.FlagNonsymbolic
	} else {
		flags |= font.FlagSymbolic
	}

	// TODO(voss): font.FlagAllCap
	// TODO(voss): font.FlagSmallCap

	res := &Info{
		CMap:        cmap,
		FontName:    fontName,
		GlyphUnits:  int(tt.Head.UnitsPerEm),
		Ascent:      Ascent,
		Descent:     Descent,
		CapHeight:   capHeight,
		ItalicAngle: postInfo.ItalicAngle,

		GlyphExtent: GlyphExtent,
		Width:       Width,
		Flags:       flags,
		GSUB:        gsub,
		GPOS:        gpos,
		KernInfo:    kernInfo,
	}
	return res, nil
}

// isSubset returns true if the font includes only runes from the
// given character set.
func isSubset(cmap map[rune]font.GlyphID, charset map[rune]bool) bool {
	for r := range cmap {
		if !charset[r] {
			return false
		}
	}
	return true
}
