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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/parser"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/locale"
)

// OldEmbed embeds a TrueType font into a pdf file.
func OldEmbed(w *pdf.Writer, name string, fname string, subset map[rune]bool) (*font.Font, error) {
	tt, err := sfnt.Open(fname)
	if err != nil {
		return nil, err
	}
	defer tt.Close()

	return OldEmbedFont(w, name, tt, subset)
}

// OldEmbedFont embeds a TrueType font into a pdf file.
func OldEmbedFont(w *pdf.Writer, name string, tt *sfnt.Font, subset map[rune]bool) (*font.Font, error) {
	if !tt.IsTrueType() {
		return nil, errors.New("not a TrueType font")
	}

	// step 1: determine which glyphs to include
	CMap := tt.CMap
	if subset != nil && !isSuperset(CMap, subset) {
		var missing []rune
		for r, ok := range subset {
			if !ok {
				continue
			}
			if CMap[r] == 0 {
				missing = append(missing, r)
			}
		}
		return nil, fmt.Errorf("missing glyphs: %q", string(missing))
	}

	var glyphs []font.GlyphID
	glyphs = append(glyphs, 0) // always include the placeholder glyph
	for r, idx := range CMap {
		if subset == nil || subset[r] {
			glyphs = append(glyphs, idx)
		}
	}

	// TODO(voss): also include glyphs used for ligatures

	// TODO(voss): subset the font as needed
	// TODO(voss): if len(glyphs) <= 256, write a Type 2 font.

	err := w.CheckVersion("use of TrueType-based CIDFonts", pdf.V1_3)
	if err != nil {
		return nil, err
	}

	// step 2: store a copy of the font file in the font stream.
	size := w.NewPlaceholder(10)
	dict := pdf.Dict{
		"Length1": size, // TODO(voss): maybe only needed for Subtype=TrueType?
	}
	opt := &pdf.StreamOptions{
		Filters: []*pdf.FilterInfo{
			{Name: "FlateDecode"},
		},
	}
	stm, FontFile, err := w.OpenStream(dict, nil, opt)
	if err != nil {
		return nil, err
	}
	exOpt := &sfnt.ExportOptions{
		Include: map[string]bool{
			// The list of tables to include is from PDF 32000-1:2008, table 126.
			// TODO(voss): include "gasp"?
			"glyf": true,
			"head": true,
			"hhea": true,
			"hmtx": true,
			"loca": true,
			"maxp": true,
			"cvt ": true,
			"fpgm": true,
			"prep": true,
			"gasp": true,
		},
	}
	n, err := tt.Export(stm, exOpt)
	if err != nil {
		return nil, err
	}
	err = size.Set(pdf.Integer(n))
	if err != nil {
		return nil, err
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	// factor for converting from TrueType FUnit to PDF glyph units
	q := 1000 / float64(tt.Head.UnitsPerEm) // TODO(voss): fix this

	postInfo, err := tt.GetPostInfo()
	if err != nil {
		return nil, err
	}

	hheaInfo, err := tt.GetHHeaInfo()
	if err != nil {
		return nil, err
	}

	hmtx, err := tt.GetHMtxInfo(hheaInfo.NumOfLongHorMetrics)
	if err != nil {
		return nil, err
	}

	os2Info, err := tt.GetOS2Info()
	// The "OS/2" table is optional for TrueType fonts, but required for
	// OpenType fonts.
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}

	Width := make([]int, tt.NumGlyphs)

	IsItalic := tt.Head.MacStyle&(1<<1) != 0

	Ascent := float64(hheaInfo.Ascent) * q
	Descent := float64(hheaInfo.Descent) * q
	// LineGap := float64(hheaInfo.LineGap) * q

	for i := 0; i < tt.NumGlyphs; i++ {
		j := i % len(hmtx.HMetrics)
		Width[i] = int(float64(hmtx.HMetrics[j].AdvanceWidth)*q + 0.5)
	}

	glyf, err := tt.GetGlyfInfo()
	if err != nil {
		return nil, err
	}
	GlyphExtent := make([]font.Rect, tt.NumGlyphs)
	for i := 0; i < tt.NumGlyphs; i++ {
		GlyphExtent[i].LLx = int(float64(glyf.Data[i].XMin)*q + 0.5)
		GlyphExtent[i].LLy = int(float64(glyf.Data[i].YMin)*q + 0.5)
		GlyphExtent[i].URx = int(float64(glyf.Data[i].XMax)*q + 0.5)
		GlyphExtent[i].URy = int(float64(glyf.Data[i].YMax)*q + 0.5)
	}

	// provisional weight values, updated below
	var Weight int // 300 = light, 400 = regular, 700 = bold
	if tt.Head.MacStyle&(1<<0) != 0 {
		// bold font
		Weight = 700
	} else {
		Weight = 400
	}

	FontName, err := tt.GetFontName()
	if err != nil {
		// TODO(voss): if FontName == "", invent a name: The name must be no
		// longer than 63 characters and restricted to the printable ASCII
		// subset, codes 33 to 126, except for the 10 characters '[', ']', '(',
		// ')', '{', '}', '<', '>', '/', '%'.
		return nil, err
	}

	var CapHeight float64
	if os2Info != nil {
		// If the "OS/2" table is present, Windows seems to use this table to
		// decide whether the font is bold/italic.  We follow Window's lead
		// here (overriding the values from the head table).
		IsItalic = os2Info.V0.Selection&(1<<0) != 0

		Weight = int(os2Info.V0.WeightClass)

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
			Ascent = ascent * q
			Descent = descent * q
			// LineGap = float64(os2Info.V0MS.TypoLineGap) * q
		}

		if os2Info.V0.Version >= 4 {
			CapHeight = float64(os2Info.V4.CapHeight) * q
		}
	}

	if CapHeight == 0 {
		// TODO(voss): CapHeight may be set equal to the top of the unscaled
		// and unhinted glyph bounding box of the glyph encoded at U+0048
		// (LATIN CAPITAL LETTER H)
		CapHeight = 800
	}

	var flags fontFlags
	if os2Info != nil {
		switch os2Info.V0.FamilyClass >> 8 {
		case 1, 2, 3, 4, 5, 7:
			flags |= fontFlagSerif
		case 10:
			flags |= fontFlagScript
		}
	}
	if postInfo.IsFixedPitch {
		flags |= fontFlagFixedPitch
	}
	if IsItalic {
		flags |= fontFlagItalic
	}
	if isSubset(CMap, font.AdobeStandardLatin) {
		flags |= fontFlagNonsymbolic
	} else {
		flags |= fontFlagSymbolic
	}
	// TODO(voss): FontFlagAllCap
	// TODO(voss): FontFlagSmallCap

	// step 3: write the FontDescriptor dictionary
	FontDescriptor := pdf.Dict{
		"Type":     pdf.Name("FontDescriptor"),
		"FontName": pdf.Name(FontName),
		"Flags":    pdf.Integer(flags),
		"FontBBox": &pdf.Rectangle{
			LLx: float64(tt.Head.XMin) * q,
			LLy: float64(tt.Head.YMin) * q,
			URx: float64(tt.Head.XMax) * q,
			URy: float64(tt.Head.YMax) * q,
		},
		"ItalicAngle": pdf.Number(postInfo.ItalicAngle),
		"Ascent":      pdf.Number(Ascent),
		"Descent":     pdf.Number(Descent),
		"CapHeight":   pdf.Number(CapHeight),

		// TrueType files don't contain this information, so make up a value.
		// The coefficients were found using linear regression over the fonts
		// found in a large collection of PDF files.  The fit is not good, but
		// I guess this is still better than just saying 70.
		"StemV": pdf.Integer(0.0838*float64(Weight) + 36.0198 + 0.5),

		"FontFile2": FontFile,
	}
	FontDescriptorRef, err := w.Write(FontDescriptor, nil)
	if err != nil {
		return nil, err
	}

	// TODO(voss): make sure there is only one copy of this per PDF file.
	CIDSystemInfoRef, err := w.Write(pdf.Dict{
		"Registry":   pdf.String("Adobe"),
		"Ordering":   pdf.String("Identity"),
		"Supplement": pdf.Integer(0),
	}, nil)
	if err != nil {
		return nil, err
	}

	DW := mostFrequent(Width)
	W := encodeWidths(Width, DW)
	WRefs, err := w.WriteCompressed(nil, W)
	if err != nil {
		return nil, err
	}

	CIDFont := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType2"),
		"BaseFont":       pdf.Name(FontName),
		"CIDSystemInfo":  CIDSystemInfoRef,
		"FontDescriptor": FontDescriptorRef,
		"W":              WRefs[0],
	}
	if DW != 1000 {
		CIDFont["DW"] = pdf.Integer(DW)
	}

	CIDFontRef, err := w.Write(CIDFont, nil)
	if err != nil {
		return nil, err
	}

	cmapStream, ToUnicodeRef, err := w.OpenStream(pdf.Dict{}, nil, &pdf.StreamOptions{
		Filters: []*pdf.FilterInfo{
			{Name: "FlateDecode"},
		},
	})
	if err != nil {
		return nil, err
	}
	xxx := make(map[font.GlyphID]rune)
	for r, c := range CMap {
		xxx[c] = r
	}
	cmapInfo := &cmapInfo{
		Name:     "Adobe-Identity-UCS",
		Registry: "Adobe",
		Ordering: "UCS",
	}
	cmapInfo.FillRanges(xxx)
	err = cMapTmpl.Execute(cmapStream, cmapInfo)
	if err != nil {
		return nil, err
	}
	err = cmapStream.Close()
	if err != nil {
		return nil, err
	}

	FontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(FontName),
		"Encoding":        pdf.Name("Identity-H"),
		"DescendantFonts": pdf.Array{CIDFontRef},
		"ToUnicode":       ToUnicodeRef,
	}
	if w.Version == pdf.V1_0 {
		FontDict["Name"] = pdf.Name(name)
	}
	FontRef, err := w.Write(FontDict, nil)
	if err != nil {
		return nil, err
	}

	fontObj := &font.Font{
		InstName: pdf.Name(name),
		Ref:      FontRef,
		CMap:     CMap,
		Enc: func(gid font.GlyphID) pdf.String {
			return pdf.String{byte(gid >> 8), byte(gid)}
		},
		GlyphExtent: GlyphExtent,
		GlyphUnits:  1000, // TODO(voss): use font design units here
		Width:       Width,
		Ascent:      Ascent,
		Descent:     Descent,
	}

	// TODO(voss): set the locale properly, somehow
	loc := locale.EnGB

	pars := parser.New(tt)
	gsub, err := pars.ReadGsubTable(loc)
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}

	gpos, err := pars.ReadGposTable(loc)
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}

	allLookups := append(gsub, gpos...)

	var kernFn func(glyphs []font.Glyph)
	if gpos == nil {
		kerning, err := tt.ReadKernInfo()
		if err != nil && !table.IsMissing(err) {
			return nil, err
		}
		if kerning != nil {
			kernFn = func(glyphs []font.Glyph) {
				for i, glyph := range glyphs {
					if i > 0 {
						pair := font.GlyphPair{glyphs[i-1].Gid, glyph.Gid}
						if dx, ok := kerning[pair]; ok {
							glyphs[i-1].Advance += dx
						}
					}
				}
			}
		}
	}

	fontObj.Layout = func(gg []font.Glyph) []font.Glyph {
		gg = allLookups.ApplyAll(gg)
		for i, g := range gg {
			gg[i].Advance = fontObj.Width[g.Gid]
		}
		if kernFn != nil {
			kernFn(gg)
		}
		return gg
	}

	return fontObj, nil
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

// isSuperset returns true if the font includes all runes of the
// given character set.
func isSuperset(cmap map[rune]font.GlyphID, charset map[rune]bool) bool {
	for r, ok := range charset {
		if ok && cmap[r] == 0 {
			return false
		}
	}
	return true
}
