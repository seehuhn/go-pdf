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

package truetype

import (
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/parser"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/locale"
)

// TrueType fonts with <=255 glyphs (PDF 1.1)
//   Type=Font, Subtype=TrueType
//   --FontDescriptor-> Type=FontDescriptor
//   --FontFile2-> Length1=...

// TrueType fonts with >255 glyphs (PDF 1.3)
//   Type=Font, Subtype=Type0
//   --DescendantFonts-> Type=Font, Subtype=CIDFontType2
//   --FontDescriptor-> Type=FontDescriptor
//   --FontFile2-> Length1=...

// EmbedSimple embeds the given TrueType font into a pdf file as a simple font.
// Up to 255 arbitrary glyphs from the font file can be accessed via the
// returned font object.
// This requires PDF version 1.1 or higher.
func EmbedSimple(w *pdf.Writer, name string, fname string, loc *locale.Locale) (*font.Font, error) {
	tt, err := sfnt.Open(fname)
	if err != nil {
		return nil, err
	}
	// TODO(voss): where/when to close this file

	return EmbedFontSimple(w, tt, name, loc)
}

// EmbedFontSimple embeds a TrueType font into a pdf file as a simple font. Up
// to 255 arbitrary glyphs from the font file can be accessed via the returned
// font object.
// This requires PDF version 1.1 or higher.
func EmbedFontSimple(w *pdf.Writer, tt *sfnt.Font, instName string, loc *locale.Locale) (*font.Font, error) {
	err := w.CheckVersion("use of TrueType fonts", pdf.V1_1)
	if err != nil {
		return nil, err
	}

	t, err := newTruetype(w, tt, instName, loc)
	if err != nil {
		return nil, err
	}
	w.OnClose(t.WriteFontDict)

	res := &font.Font{
		InstName:    pdf.Name(instName),
		Ref:         t.Ref,
		CMap:        t.CMap,
		Layout:      t.Layout,
		Enc:         t.Enc,
		GlyphUnits:  t.GlyphUnits,
		Ascent:      t.Ascent,
		Descent:     t.Descent,
		GlyphExtent: t.GlyphExtent,
		Width:       t.Width,
	}
	return res, nil
}

type truetype struct {
	InstName string
	Ttf      *sfnt.Font
	CMap     map[rune]font.GlyphID
	os2Info  *table.OS2

	// Information for the Font dictionary
	FontName    pdf.Name
	Ref         *pdf.Reference
	GlyphUnits  int
	Ascent      float64 // Ascent in glyph coordinate units
	Descent     float64 // Descent in glyph coordinate units, as a negative number
	GlyphExtent []font.Rect
	Width       []int

	Lookups  parser.Lookups
	KernInfo map[font.GlyphPair]int

	enc  map[font.GlyphID]byte
	used map[byte]bool
	tidy map[font.GlyphID]byte

	overflowed bool
}

func newTruetype(w *pdf.Writer, tt *sfnt.Font, instName string, loc *locale.Locale) (*truetype, error) {
	if !tt.IsTrueType() {
		return nil, errors.New("not a TrueType font")
	}

	hheaInfo, err := tt.GetHHeaInfo()
	if err != nil {
		return nil, err
	}

	os2Info, err := tt.GetOS2Info()
	if err != nil && !table.IsMissing(err) {
		// The "OS/2" table is optional for TrueType fonts, but required for
		// OpenType fonts.
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

	Ascent := float64(hheaInfo.Ascent)
	Descent := float64(hheaInfo.Descent)
	if os2Info != nil && os2Info.V0MSValid {
		if os2Info.V0.Selection&(1<<7) != 0 {
			Ascent = float64(os2Info.V0MS.TypoAscender)
			Descent = float64(os2Info.V0MS.TypoDescender)
		} else {
			Ascent = float64(os2Info.V0MS.WinAscent)
			Descent = -float64(os2Info.V0MS.WinDescent)
		}
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

	tidy := make(map[font.GlyphID]byte)
	for r, gid := range cmap {
		if rOld, used := tidy[gid]; r < 127 && (!used || byte(r) < rOld) {
			tidy[gid] = byte(r)
		}
	}

	subsetTag := "AAAAAA+" // TODO(voss)
	res := &truetype{
		InstName: instName,
		Ttf:      tt,
		os2Info:  os2Info,
		CMap:     cmap,

		FontName:    pdf.Name(subsetTag + fontName),
		Ref:         w.Alloc(),
		GlyphUnits:  int(tt.Head.UnitsPerEm),
		Ascent:      Ascent,
		Descent:     Descent,
		GlyphExtent: GlyphExtent,
		Width:       Width,

		Lookups:  append(gsub, gpos...),
		KernInfo: kernInfo,

		enc:  make(map[font.GlyphID]byte),
		used: map[byte]bool{},
		tidy: tidy,
	}

	return res, nil
}

func (t *truetype) Layout(gg []font.Glyph) []font.Glyph {
	gg = t.Lookups.ApplyAll(gg)

	for i, g := range gg {
		gg[i].Advance = t.Width[g.Gid]
	}

	if t.KernInfo != nil {
		for i := 0; i+1 < len(gg); i++ {
			pair := font.GlyphPair{gg[i].Gid, gg[i+1].Gid}
			if dx, ok := t.KernInfo[pair]; ok {
				gg[i].Advance += dx
			}
		}
	}

	return gg
}

func (t *truetype) Enc(gid font.GlyphID) pdf.String {
	c, ok := t.enc[gid]
	if ok {
		return pdf.String{c}
	}

	c, ok = t.tidy[gid]
	if !ok {
		for i := 127; i < 127+256; i++ {
			if i < 256 {
				c = byte(i)
			} else {
				// 256 -> 126
				// 257 -> 125
				// ...
				c = byte(126 + 256 - i)
			}
			if !t.used[c] {
				ok = true
				break
			}
		}
	}

	if !ok {
		// A simple font can only encode 256 different characters. If we run
		// out of character codes, just return 0 here and report an error when
		// we try to write the font dictionary at the end.
		t.overflowed = true
		t.enc[gid] = 0
		return pdf.String{0}
	}

	t.used[c] = true
	t.enc[gid] = c
	return pdf.String{c}
}

func (t *truetype) WriteFontDict(w *pdf.Writer) error {
	if t.overflowed {
		return errors.New("too many different glyphs for simple font " + t.InstName)
	}
	if len(t.enc) == 0 {
		return nil
	}

	subset := make([]font.GlyphID, 256)
	first := 257
	last := -1
	for origGid, c := range t.enc {
		if int(c) < first {
			first = int(c)
		}
		if int(c) > last {
			last = int(c)
		}
		subset[c] = origGid
	}

	fontDesc, err := t.WriteFontDescriptor(w, subset)
	if err != nil {
		return err
	}

	var ww pdf.Array
	q := 1000 / float64(t.GlyphUnits)
	for i := first; i <= last; i++ {
		gid := subset[i]
		width := int(float64(t.Width[gid])*q + 0.5)
		if !t.used[byte(i)] {
			width = 0
		}
		ww = append(ww, pdf.Integer(width))
	}
	widths, err := w.Write(ww, nil)
	if err != nil {
		return err
	}

	// See sections 9.6.3 and 9.6.2.1 of PDF 32000-1:2008.
	Font := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("TrueType"),
		"BaseFont":       t.FontName,
		"FirstChar":      pdf.Integer(first),
		"LastChar":       pdf.Integer(last),
		"Widths":         widths,
		"FontDescriptor": fontDesc,
		"ToUnicode":      nil, // TODO(voss)
	}

	_, err = w.Write(Font, t.Ref)
	return err
}

func (t *truetype) WriteFontDescriptor(w *pdf.Writer, subset []font.GlyphID) (*pdf.Reference, error) {
	fontFileRef, err := t.WriteFontFile(w, subset)
	if err != nil {
		return nil, err
	}

	tt := t.Ttf
	postInfo, err := tt.GetPostInfo()
	if err != nil {
		return nil, err
	}

	var flags fontFlags
	if t.os2Info != nil {
		fmt.Printf("%#v\n", t.os2Info)
		switch t.os2Info.V0.FamilyClass >> 8 {
		case 1, 2, 3, 4, 5, 7:
			flags |= fontFlagSerif
		case 10:
			flags |= fontFlagScript
		}
	}
	if postInfo.IsFixedPitch {
		flags |= fontFlagFixedPitch
	}
	IsItalic := tt.Head.MacStyle&(1<<1) != 0
	if t.os2Info != nil {
		// If the "OS/2" table is present, Windows seems to use this table to
		// decide whether the font is bold/italic.  We follow Window's lead
		// here (overriding the values from the head table).
		IsItalic = t.os2Info.V0.Selection&(1<<0) != 0
	}
	if IsItalic {
		flags |= fontFlagItalic
	}
	flags |= fontFlagSymbolic
	// TODO(voss): FontFlagAllCap
	// TODO(voss): FontFlagSmallCap

	// compute the font bounding box for the subset
	left := math.MaxInt
	right := math.MinInt
	top := math.MinInt
	bottom := math.MaxInt
	for _, origGid := range subset {
		box := t.GlyphExtent[origGid]
		if box.LLx < left {
			left = box.LLx
		}
		if box.URx > right {
			right = box.URx
		}
		if box.LLy < bottom {
			bottom = box.LLy
		}
		if box.URy > top {
			top = box.URy
		}
	}
	q := 1000 / float64(t.GlyphUnits)
	FontBBox := &pdf.Rectangle{
		LLx: math.Round(float64(left)*q + 0.5),
		LLy: math.Round(float64(bottom)*q + 0.5),
		URx: math.Round(float64(right)*q + 0.5),
		URy: math.Round(float64(top)*q + 0.5),
	}

	var capHeight int
	if H, ok := t.CMap['H']; ok {
		// CapHeight may be set equal to the top of the unscaled and unhinted
		// glyph bounding box of the glyph encoded at U+0048 (LATIN CAPITAL
		// LETTER H)
		capHeight = t.GlyphExtent[H].URy
	} else if t.os2Info != nil && t.os2Info.V0.Version >= 4 {
		capHeight = int(t.os2Info.V4.CapHeight)
	} else {
		capHeight = 800
	}

	// See sections 9.8.1 of PDF 32000-1:2008.
	FontDescriptor := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    t.FontName,
		"Flags":       pdf.Integer(flags),
		"FontBBox":    FontBBox,
		"ItalicAngle": pdf.Number(postInfo.ItalicAngle),
		"Ascent":      pdf.Number(t.Ascent),
		"Descent":     pdf.Number(t.Descent),
		"CapHeight":   pdf.Integer(q*float64(capHeight) + 0.5),
		"StemV":       pdf.Integer(70),
		"FontFile2":   fontFileRef,
	}
	return w.Write(FontDescriptor, nil)
}

func (t *truetype) WriteFontFile(w *pdf.Writer, subset []font.GlyphID) (*pdf.Reference, error) {
	// See section 9.9 of PDF 32000-1:2008.
	size := w.NewPlaceholder(10)
	fontFileDict := pdf.Dict{
		"Length1": size,
	}
	opt := &pdf.StreamOptions{
		Filters: []*pdf.FilterInfo{
			{Name: "FlateDecode"},
		},
	}
	fontFileStream, fontFile, err := w.OpenStream(fontFileDict, nil, opt)
	if err != nil {
		return nil, err
	}
	exOpt := &sfnt.ExportOptions{
		Include: map[string]bool{
			// The list of tables to include is from PDF 32000-1:2008, table 126.
			"cvt ": true, // copy
			"fpgm": true, // copy
			"prep": true, // copy
			"head": true, // update CheckSumAdjustment, Modified and indexToLocFormat
			"hhea": true, // update various fields, including numberOfHMetrics
			"maxp": true, // update numGlyphs
			"hmtx": true, // rewrite
			"loca": true, // rewrite
			"glyf": true, // rewrite
		},
		Subset: subset,
	}
	n, err := t.Ttf.Export(fontFileStream, exOpt)
	if err != nil {
		return nil, err
	}
	err = size.Set(pdf.Integer(n))
	if err != nil {
		return nil, err
	}
	err = fontFileStream.Close()
	if err != nil {
		return nil, err
	}
	return fontFile, nil
}

type fontFlags int

const (
	fontFlagFixedPitch  fontFlags = 1 << 0  // All glyphs have the same width (as opposed to proportional or variable-pitch fonts, which have different widths).
	fontFlagSerif       fontFlags = 1 << 1  // Glyphs have serifs, which are short strokes drawn at an angle on the top and bottom of glyph stems. (Sans serif fonts do not have serifs.)
	fontFlagSymbolic    fontFlags = 1 << 2  // Font contains glyphs outside the Adobe standard Latin character set. This flag and the Nonsymbolic flag shall not both be set or both be clear.
	fontFlagScript      fontFlags = 1 << 3  // Glyphs resemble cursive handwriting.
	fontFlagNonsymbolic fontFlags = 1 << 5  // Font uses the Adobe standard Latin character set or a subset of it.
	fontFlagItalic      fontFlags = 1 << 6  // Glyphs have dominant vertical strokes that are slanted.
	fontFlagAllCap      fontFlags = 1 << 16 // Font contains no lowercase letters; typically used for display purposes, such as for titles or headlines.
	fontFlagSmallCap    fontFlags = 1 << 17 // Font contains both uppercase and lowercase letters.  The uppercase letters are similar to those in the regular version of the same typeface family. The glyphs for the lowercase letters have the same shapes as the corresponding uppercase letters, but they are sized and their proportions adjusted so that they have the same size and stroke weight as lowercase glyphs in the same typeface family.
	fontFlagForceBold   fontFlags = 1 << 18 // ...
)
