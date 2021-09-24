// seehuhn.de/go/pdf - a library for reading and writing PDF files
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

package sfnt

import (
	"errors"
	"fmt"
	"io"
	"os"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/gtab"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/locale"
)

// Font describes a TrueType font file.
type Font struct {
	Fd     *os.File
	Header *table.Header

	head *table.Head // TODO(voss): needed?

	CMap     map[rune]font.GlyphID
	FontName string
	Flags    font.Flags

	GlyphUnits  int
	Ascent      int // Ascent in glyph coordinate units
	Descent     int // Descent in glyph coordinate units, as a negative number
	CapHeight   int
	ItalicAngle float64

	FontBBox *font.Rect

	GlyphExtent []font.Rect
	Width       []int

	GSUB, GPOS gtab.Lookups
}

// Open loads a TrueType font into memory.
// The .Close() method must be called once the Font is no longer used.
func Open(fname string, loc *locale.Locale) (*Font, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	header, err := table.ReadHeader(fd)
	if err != nil {
		if err == io.ErrUnexpectedEOF {
			err = errors.New("malformed/corrupted font file")
		}
		return nil, err
	}

	head := &table.Head{}
	_, err = header.GetTableReader(fd, "head", head)
	if err != nil {
		return nil, err
	}
	if head.Version != 0x00010000 {
		return nil, fmt.Errorf(
			"sfnt/head: unsupported version 0x%08X", head.Version)
	}
	if head.MagicNumber != 0x5F0F3CF5 {
		return nil, errors.New("sfnt/head: wrong magic number")
	}

	tt := &Font{
		Fd:     fd,
		Header: header,
		head:   head,
	}

	maxp, err := tt.getMaxpInfo()
	if err != nil {
		return nil, err
	}
	if maxp.NumGlyphs < 2 {
		// glyph index 0 denotes a missing character and is always included
		return nil, errors.New("no glyphs found")
	}
	NumGlyphs := int(maxp.NumGlyphs)

	hheaInfo, err := tt.getHHeaInfo()
	if err != nil {
		return nil, err
	}

	os2Info, err := tt.getOS2Info()
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}

	hmtx, err := tt.getHMtxInfo(NumGlyphs, int(hheaInfo.NumOfLongHorMetrics))
	if err != nil {
		return nil, err
	}

	glyf, err := tt.getGlyfInfo(NumGlyphs)
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}

	postInfo, err := tt.getPostInfo()
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}

	fontName, err := tt.getFontName()
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	// TODO(voss): if FontName == "", invent a name: The name must be no
	// longer than 63 characters and restricted to the printable ASCII
	// subset, codes 33 to 126, except for the 10 characters '[', ']', '(',
	// ')', '{', '}', '<', '>', '/', '%'.

	cmap, err := tt.SelectCMap()
	if err != nil {
		return nil, err
	}

	var GlyphExtent []font.Rect
	if glyf != nil {
		GlyphExtent = make([]font.Rect, NumGlyphs)
		for i := 0; i < NumGlyphs; i++ {
			GlyphExtent[i].LLx = int(glyf.Data[i].XMin)
			GlyphExtent[i].LLy = int(glyf.Data[i].YMin)
			GlyphExtent[i].URx = int(glyf.Data[i].XMax)
			GlyphExtent[i].URy = int(glyf.Data[i].YMax)
		}
	}

	Width := make([]int, NumGlyphs)
	for i := 0; i < NumGlyphs; i++ {
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
	} else if H, ok := cmap['H']; ok && GlyphExtent != nil {
		// CapHeight may be set equal to the top of the unscaled and unhinted
		// glyph bounding box of the glyph encoded at U+0048 (LATIN CAPITAL
		// LETTER H)
		capHeight = GlyphExtent[H].URy
	} else {
		capHeight = 800
	}

	pars, err := gtab.New(tt.Header, tt.Fd, loc)
	if err != nil {
		return nil, err
	}
	gsub, err := pars.ReadGsubTable()
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	gpos, err := pars.ReadGposTable()
	if table.IsMissing(err) { // if no GPOS table is found ...
		gpos, err = tt.readKernInfo()
	}
	if err != nil && !table.IsMissing(err) { // error from either ReadGposTable() or ReadKernInfo()
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
	if postInfo != nil && postInfo.IsFixedPitch {
		flags |= font.FlagFixedPitch
	}
	IsItalic := head.MacStyle&(1<<1) != 0
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

	tt.CMap = cmap
	tt.FontName = fontName
	tt.GlyphUnits = int(tt.head.UnitsPerEm)
	tt.Ascent = Ascent
	tt.Descent = Descent
	tt.CapHeight = capHeight
	if postInfo != nil {
		tt.ItalicAngle = postInfo.ItalicAngle
	}
	tt.FontBBox = &font.Rect{
		LLx: int(head.XMin),
		LLy: int(head.YMin),
		URx: int(head.XMax),
		URy: int(head.YMax),
	}
	tt.GlyphExtent = GlyphExtent
	tt.Width = Width
	tt.Flags = flags
	tt.GSUB = gsub
	tt.GPOS = gpos

	return tt, nil
}

// Close frees all resources associated with the font.  The Font object
// cannot be used any more after Close() has been called.
func (tt *Font) Close() error {
	return tt.Fd.Close()
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

// IsTrueType checks whether the tables required for a TrueType font are
// present.
func (tt *Font) IsTrueType() bool {
	return tt.HasTables(
		"cmap", "head", "hhea", "hmtx", "maxp", "name", "post",
		"glyf", "loca")
}

// IsOpenType checks whether the tables required for an OpenType font are
// present.
func (tt *Font) IsOpenType() bool {
	if !tt.HasTables(
		"cmap", "head", "hhea", "hmtx", "maxp", "name", "post", "OS/2") {
		return false
	}
	if tt.HasTables("glyf", "loca") || tt.HasTables("CFF ") {
		return true
	}
	return false
}

// IsVariable checks whether the font is a "variable font".
func (tt *Font) IsVariable() bool {
	return tt.HasTables("fvar")
}

// SelectCMap chooses one of the sub-tables of the cmap table and reads the
// font's character encoding from there.  If a full unicode mapping is found,
// this is used.  Otherwise, the function tries to use a 16 bit BMP encoding.
// If this fails, a legacy 1,0 record is tried as a last resort.
func (tt *Font) SelectCMap() (map[rune]font.GlyphID, error) {
	rec := tt.Header.Find("cmap")
	if rec == nil {
		return nil, &table.ErrNoTable{Name: "cmap"}
	}
	cmapFd := io.NewSectionReader(tt.Fd, int64(rec.Offset), int64(rec.Length))

	cmapTable, err := table.ReadCMapTable(cmapFd)
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
		encRec := cmapTable.Find(cand.PlatformID, cand.EncodingID)
		if encRec == nil {
			continue
		}

		cmap, err := encRec.LoadCMap(cmapFd, cand.IdxToRune)
		if err != nil {
			continue
		}

		return cmap, nil
	}
	return nil, errors.New("sfnt/cmap: no supported character encoding found")
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
