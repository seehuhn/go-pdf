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

package opentype

import (
	"errors"
	"math"
	"sort"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/locale"
)

// EmbedCID embeds an OpenType font into a pdf file as a CIDFont.
//
// In comparison, fonts embedded via EmbedSimple() lead to smaller PDF files,
// but only up to 256 glyphs of the font can be accessed via the returned font
// object.
//
// Use of OpenType fonts in PDF requires PDF version 1.3 or higher.
func EmbedCID(w *pdf.Writer, fileName string, instName pdf.Name, loc *locale.Locale) (*font.Font, error) {
	tt, err := sfnt.Open(fileName, loc)
	if err != nil {
		return nil, err
	}

	return EmbedFontCID(w, tt, instName)
}

// EmbedFontCID embeds an OpenType font into a pdf file as a CIDFont.
//
// This function takes ownership of tt and will close the font tt once it is no
// longer needed.
//
// In comparison, fonts embedded via EmbedFontSimple() lead to smaller PDF files,
// but only up to 256 glyphs of the font can be accessed via the returned font
// object.
//
// Use of OpenType CIDFonts requires PDF version 1.3 or higher.
func EmbedFontCID(w *pdf.Writer, tt *sfnt.Font, instName pdf.Name) (*font.Font, error) {
	if !tt.IsOpenType() {
		return nil, errors.New("not an OpenType font")
	}
	if tt.IsTrueType() {
		return truetype.EmbedFontCID(w, tt, instName)
	}
	err := w.CheckVersion("use of OpenType CIDFonts", pdf.V1_3)
	if err != nil {
		return nil, err
	}

	r, err := tt.GetTableReader("CFF ", nil)
	if err != nil {
		return nil, err
	}
	cff, err := cff.Read(r)
	if err != nil {
		return nil, err
	}

	t := &cidFont{
		Sfnt: tt,
		Cff:  cff,

		FontRef: w.Alloc(),

		text: make(map[font.GlyphID][]rune),
		used: map[font.GlyphID]bool{
			0: true, // TODO(voss): is this needed?
		},
	}

	w.OnClose(t.WriteFont)

	res := &font.Font{
		InstName: instName,
		Ref:      t.FontRef,

		GlyphUnits:  tt.GlyphUnits,
		Ascent:      tt.Ascent,
		Descent:     tt.Descent,
		GlyphExtent: cff.GlyphExtents(),
		Width:       tt.Width,

		Layout: t.Layout,
		Enc:    t.Enc,
	}
	return res, nil
}

type cidFont struct {
	Sfnt *sfnt.Font
	Cff  *cff.Font

	FontRef *pdf.Reference

	text map[font.GlyphID][]rune // GID -> text
	used map[font.GlyphID]bool   // is GID used?
}

func (t *cidFont) Layout(rr []rune) []font.Glyph {
	gg := make([]font.Glyph, len(rr))
	for i, r := range rr {
		gid := t.Sfnt.CMap[r]
		gg[i].Gid = gid
		gg[i].Chars = []rune{r}
	}

	gg = t.Sfnt.GSUB.ApplyAll(gg)
	for i := range gg {
		gg[i].Advance = t.Sfnt.Width[gg[i].Gid]
	}
	gg = t.Sfnt.GPOS.ApplyAll(gg)

	for _, g := range gg {
		if _, seen := t.text[g.Gid]; !seen && len(g.Chars) > 0 {
			// copy the slice, in case the caller modifies it later
			t.text[g.Gid] = append([]rune{}, g.Chars...)
		}
	}

	return gg
}

func (t *cidFont) Enc(gid font.GlyphID) pdf.String {
	t.used[gid] = true
	return pdf.String{byte(gid >> 8), byte(gid)}
}

func (t *cidFont) WriteFont(w *pdf.Writer) error {
	// Determine the subset of glyphs to include.
	origNumGlyphs := len(t.Sfnt.Width)
	includeGlyphs := []font.GlyphID{0} // always include .notdef
	for gid, ok := range t.used {
		if ok && gid != 0 {
			includeGlyphs = append(includeGlyphs, gid)
		}
	}
	sort.Slice(includeGlyphs, func(i, j int) bool {
		return includeGlyphs[i] < includeGlyphs[j]
	})
	subsetTag := font.GetSubsetTag(includeGlyphs, origNumGlyphs)
	fontName := pdf.Name(subsetTag) + "+" + t.Cff.Info.FontName
	// TODO(voss): check whether we now have two subset tags

	q := 1000 / float64(t.Sfnt.GlyphUnits)
	FontBBox := &pdf.Rectangle{
		LLx: math.Round(float64(t.Sfnt.FontBBox.LLx) * q),
		LLy: math.Round(float64(t.Sfnt.FontBBox.LLy) * q),
		URx: math.Round(float64(t.Sfnt.FontBBox.URx) * q),
		URy: math.Round(float64(t.Sfnt.FontBBox.URy) * q),
	}

	DW, W := font.EncodeCIDWidths(t.Sfnt.Width)

	CIDFontRef := w.Alloc()
	CIDSystemInfoRef := w.Alloc()
	FontDescriptorRef := w.Alloc()
	WRef := w.Alloc()
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

	Font := pdf.Dict{ // See section 9.7.6.1 of PDF 32000-1:2008.
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        fontName + "-" + "Identity-H",
		"Encoding":        pdf.Name("Identity-H"),
		"DescendantFonts": pdf.Array{CIDFontRef},
		"ToUnicode":       ToUnicodeRef,
	}

	CIDFont := pdf.Dict{ // See section 9.7.4.1 of PDF 32000-1:2008.
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType0"),
		"BaseFont":       fontName,
		"CIDSystemInfo":  CIDSystemInfoRef,
		"FontDescriptor": FontDescriptorRef,
	}
	if W != nil {
		CIDFont["W"] = WRef
	}
	if DW != 1000 {
		CIDFont["DW"] = pdf.Integer(DW)
	}

	// TODO(voss): make sure there is only one copy of this per PDF file.
	CIDSystemInfo := pdf.Dict{ // See sections 9.7.3 of PDF 32000-1:2008.
		"Registry":   pdf.String("Adobe"),
		"Ordering":   pdf.String("Identity"),
		"Supplement": pdf.Integer(0),
	}

	FontDescriptor := pdf.Dict{ // See sections 9.8.1 of PDF 32000-1:2008.
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(t.Sfnt.Flags),
		"FontBBox":    FontBBox,
		"ItalicAngle": pdf.Number(t.Sfnt.ItalicAngle),
		"Ascent":      pdf.Integer(q*float64(t.Sfnt.Ascent) + 0.5),
		"Descent":     pdf.Integer(q*float64(t.Sfnt.Descent) + 0.5),
		"CapHeight":   pdf.Integer(q*float64(t.Sfnt.CapHeight) + 0.5),
		"StemV":       pdf.Integer(70), // information not available in sfnt files
		"FontFile3":   FontFileRef,
	}

	_, err := w.WriteCompressed(
		[]*pdf.Reference{
			t.FontRef, CIDFontRef, CIDSystemInfoRef, FontDescriptorRef, WRef,
		},
		Font, CIDFont, CIDSystemInfo, FontDescriptor, W)
	if err != nil {
		return err
	}

	// write all the streams

	// Write the font file itself.
	// See section 9.9 of PDF 32000-1:2008 for details.
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("CIDFontType0C"),
	}
	fontFileStream, _, err := w.OpenStream(fontFileDict, FontFileRef,
		&pdf.FilterInfo{Name: "FlateDecode"})
	if err != nil {
		return err
	}
	cff, err := t.Cff.Subset(includeGlyphs)
	if err != nil {
		return err
	}
	err = cff.EncodeCID(fontFileStream, "Adobe", "Identity", 0)
	if err != nil {
		return err
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	// Write the ToUnicode CMap.
	var cid2text []font.CIDMapping
	for cid, text := range t.text {
		cid2text = append(cid2text, font.CIDMapping{CharCode: uint16(cid), Text: text})
	}
	err = font.WriteToUnicodeCID(w, cid2text, ToUnicodeRef)
	if err != nil {
		return err
	}

	err = t.Sfnt.Close()
	if err != nil {
		return err
	}

	return nil
}
