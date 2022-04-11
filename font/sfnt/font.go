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
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/funit"
	"seehuhn.de/go/pdf/font/sfnt/gtab"
	"seehuhn.de/go/pdf/font/sfnt/head"
	"seehuhn.de/go/pdf/font/sfnt/hmtx"
	"seehuhn.de/go/pdf/font/sfnt/mac"
	"seehuhn.de/go/pdf/font/sfnt/maxp"
	"seehuhn.de/go/pdf/font/sfnt/os2"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/locale"
)

// Font describes a TrueType or OpenType font file.
type Font struct {
	FontName string
	HeadInfo *head.Info
	HmtxInfo *hmtx.Info

	// TODO(voss): tidy the fields below

	Fd     *os.File
	Header *table.Header

	CMap  map[rune]font.GlyphID
	Flags font.Flags

	GlyphUnits  int
	CapHeight   int
	ItalicAngle float64

	FontBBox *font.Rect // always uses 1000 units to the em (not GlyphUnits)

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

	tt := &Font{
		Fd:     fd,
		Header: header,
	}

	headFd, err := tt.GetTableReader("head", nil)
	if err != nil {
		return nil, err
	}
	tt.HeadInfo, err = head.Read(headFd)
	if err != nil {
		return nil, err
	}

	hheaData, err := tt.Header.ReadTableBytes(tt.Fd, "hhea")
	if err != nil {
		return nil, err
	}
	hmtxData, err := tt.Header.ReadTableBytes(tt.Fd, "hmtx")
	if err != nil {
		return nil, err
	}
	hmtxInfo, err := hmtx.Decode(hheaData, hmtxData)
	if err != nil {
		return nil, err
	}

	maxpFd, err := tt.GetTableReader("maxp", nil)
	if err != nil {
		return nil, err
	}
	maxpInfo, err := maxp.Read(maxpFd)
	if err != nil {
		return nil, err
	}
	NumGlyphs := maxpInfo.NumGlyphs
	if NumGlyphs != len(hmtxInfo.Widths) {
		return nil, errors.New("inconsistent number of glyphs")
	}

	var os2Info *os2.Info
	os2Fd, err := tt.GetTableReader("OS/2", nil)
	if table.IsMissing(err) {
		// pass
	} else if err != nil {
		return nil, err
	} else {
		os2Info, err = os2.Read(os2Fd)
		if err != nil {
			return nil, err
		}
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

	var GlyphExtent []funit.Rect
	if glyf != nil {
		GlyphExtent = make([]funit.Rect, NumGlyphs)
		for i := 0; i < NumGlyphs; i++ {
			GlyphExtent[i].LLx = glyf.Data[i].XMin
			GlyphExtent[i].LLy = glyf.Data[i].YMin
			GlyphExtent[i].URx = glyf.Data[i].XMax
			GlyphExtent[i].URy = glyf.Data[i].YMax
		}
	} else {
		// TODO(voss): get the glyph extents for OpenType fonts
	}
	hmtxInfo.GlyphExtents = GlyphExtent

	if os2Info != nil {
		hmtxInfo.Ascent = os2Info.Ascent
		hmtxInfo.Descent = os2Info.Descent
		hmtxInfo.LineGap = os2Info.LineGap
	}

	var capHeight int
	if os2Info != nil && os2Info.CapHeight != 0 {
		capHeight = int(os2Info.CapHeight)
	} else if H, ok := cmap['H']; ok && GlyphExtent != nil {
		// CapHeight may be set equal to the top of the unscaled and unhinted
		// glyph bounding box of the glyph encoded at U+0048 (LATIN CAPITAL
		// LETTER H)
		capHeight = int(GlyphExtent[H].URy)
	} else {
		capHeight = 800 // TODO(voss): adjust for glyphUnits
	}

	pars, err := gtab.Read(tt.Header, tt.Fd, loc)
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
		switch os2Info.FamilyClass >> 8 {
		case 1, 2, 3, 4, 5, 7:
			flags |= font.FlagSerif
		case 10:
			flags |= font.FlagScript
		}
	}
	if postInfo != nil && postInfo.IsFixedPitch {
		flags |= font.FlagFixedPitch
	}
	IsItalic := tt.HeadInfo.IsItalic
	if os2Info != nil {
		// If the "OS/2" table is present, Windows seems to use this table to
		// decide whether the font is bold/italic.  We follow Window's lead
		// here (overriding the values from the head table).
		IsItalic = os2Info.IsItalic
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

	q := 1000 / float64(tt.HeadInfo.UnitsPerEm)
	fbox := tt.HeadInfo.FontBBox
	bbox := &font.Rect{
		LLx: int16(math.Round(float64(fbox.LLx) * q)),
		LLy: int16(math.Round(float64(fbox.LLy) * q)),
		URx: int16(math.Round(float64(fbox.URx) * q)),
		URy: int16(math.Round(float64(fbox.URy) * q)),
	}

	tt.CMap = cmap
	tt.FontName = fontName
	tt.HmtxInfo = hmtxInfo
	tt.GlyphUnits = int(tt.HeadInfo.UnitsPerEm)
	tt.CapHeight = capHeight
	if postInfo != nil {
		tt.ItalicAngle = postInfo.ItalicAngle
	}
	tt.FontBBox = bbox
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

// NumGlyphs returns the number of glyphs in the font.
// This value always include the ".notdef" glyph.
func (tt *Font) NumGlyphs() int {
	return len(tt.HmtxInfo.Widths)
}

// HasTables returns true, if all the given tables are present in the font.
func (tt *Font) HasTables(names ...string) bool {
	for _, name := range names {
		if _, ok := tt.Header.Toc[name]; !ok {
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

// GetTableReader returns an io.SectionReader which can be used to read
// the table data.  If head is non-nil, binary.Read() is used to
// read the data at the start of the table into the value head points to.
func (tt *Font) GetTableReader(name string, head interface{}) (*io.SectionReader, error) {
	rec, err := tt.Header.Find(name)
	if err != nil {
		return nil, err
	}
	tableFd := io.NewSectionReader(tt.Fd, int64(rec.Offset), int64(rec.Length))

	if head != nil {
		err := binary.Read(tableFd, binary.BigEndian, head)
		if err != nil {
			return nil, err
		}
	}

	return tableFd, nil
}

// SelectCMap chooses one of the sub-tables of the cmap table and reads the
// font's character encoding from there.  If a full unicode mapping is found,
// this is used.  Otherwise, the function tries to use a 16 bit BMP encoding.
// If this fails, a legacy 1,0 record is tried as a last resort.
func (tt *Font) SelectCMap() (map[rune]font.GlyphID, error) {
	rec, err := tt.Header.Find("cmap")
	if err != nil {
		return nil, err
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
		// TODO(voss): declutter this
		return []rune(mac.Decode([]byte{byte(idx)}))[0]
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
