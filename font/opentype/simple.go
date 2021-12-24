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

// EmbedSimple embeds an OpenType font into a pdf file as a simple font.
// Up to 256 arbitrary glyphs from the font file can be accessed via the
// returned font object.
//
// In comparison, fonts embedded via EmbedCID lead to larger PDF files, but
// there is no limit on the number of glyphs which can be accessed.
//
// Use of simple OpenType fonts in PDF requires PDF version 1.2 or higher.
func EmbedSimple(w *pdf.Writer, fileName string, instName pdf.Name, loc *locale.Locale) (*font.Font, error) {
	tt, err := sfnt.Open(fileName, loc)
	if err != nil {
		return nil, err
	}

	return EmbedFontSimple(w, tt, instName)
}

// EmbedFontSimple embeds an OpenType font into a pdf file as a simple font.
// Up to 256 arbitrary glyphs from the font file can be accessed via the
// returned font object.
//
// This function takes ownership of tt and will close the font tt once it is no
// longer needed.
//
// In comparison, fonts embedded via EmbedFontCID lead to larger PDF files, but
// there is no limit on the number of glyphs which can be accessed.
//
// Use of simple OpenType fonts in PDF requires PDF version 1.2 or higher.
func EmbedFontSimple(w *pdf.Writer, tt *sfnt.Font, instName pdf.Name) (*font.Font, error) {
	if !tt.IsOpenType() {
		return nil, errors.New("not an OpenType font")
	}
	if tt.IsTrueType() {
		return truetype.EmbedFontSimple(w, tt, instName)
	}
	err := w.CheckVersion("use of simple OpenType fonts", pdf.V1_2)
	if err != nil {
		return nil, err
	}

	fnt := newSimple(w, tt)
	w.OnClose(fnt.WriteFont)

	res := &font.Font{
		InstName: instName,
		Ref:      fnt.FontRef,

		GlyphUnits:  tt.GlyphUnits,
		Ascent:      tt.Ascent,
		Descent:     tt.Descent,
		GlyphExtent: tt.GlyphExtent,
		Width:       tt.Width,

		Layout: fnt.Layout,
		Enc:    fnt.Enc,
	}
	return res, nil
}

type simple struct {
	Sfnt *sfnt.Font

	FontRef           *pdf.Reference
	FontDescriptorRef *pdf.Reference
	WidthsRef         *pdf.Reference
	ToUnicodeRef      *pdf.Reference
	FontFileRef       *pdf.Reference

	text  map[font.GlyphID][]rune // GID -> text
	enc   map[font.GlyphID]byte   // GID -> CharCode
	count int                     // next available CharCode
}

func newSimple(w *pdf.Writer, tt *sfnt.Font) *simple {
	tidy := make(map[font.GlyphID]byte)
	for r, gid := range tt.CMap {
		if rOld, used := tidy[gid]; r < 127 && (!used || byte(r) < rOld) {
			tidy[gid] = byte(r)
		}
	}

	res := &simple{
		Sfnt: tt,

		FontRef:           w.Alloc(),
		FontDescriptorRef: w.Alloc(),
		WidthsRef:         w.Alloc(),
		ToUnicodeRef:      w.Alloc(),
		FontFileRef:       w.Alloc(),

		text: make(map[font.GlyphID][]rune),
		enc:  make(map[font.GlyphID]byte),
	}

	return res
}

func (fnt *simple) Layout(rr []rune) []font.Glyph {
	gg := make([]font.Glyph, len(rr))
	for i, r := range rr {
		gid := fnt.Sfnt.CMap[r]
		gg[i].Gid = gid
		gg[i].Chars = []rune{r}
	}

	gg = fnt.Sfnt.GSUB.ApplyAll(gg)
	for i := range gg {
		gg[i].Advance = fnt.Sfnt.Width[gg[i].Gid]
	}
	gg = fnt.Sfnt.GPOS.ApplyAll(gg)

	for _, g := range gg {
		if _, seen := fnt.text[g.Gid]; !seen && len(g.Chars) > 0 {
			// copy the slice, in case the caller modifies it later
			fnt.text[g.Gid] = append([]rune{}, g.Chars...)
		}
	}

	return gg
}

func (fnt *simple) Enc(gid font.GlyphID) pdf.String {
	c, found := fnt.enc[gid]
	if found {
		return pdf.String{c}
	}

	// increment fnt.count first, so that 0 is allocated last
	fnt.count++
	if _, ok := fnt.enc[0]; ok {
		c = byte(fnt.count - 1)
	} else if gid != 0 {
		c = byte(fnt.count)
	} else {
		c = 0
	}

	fnt.enc[gid] = c
	return pdf.String{c}
}

func (fnt *simple) WriteFont(w *pdf.Writer) error {
	if fnt.count > 256 {
		return errors.New("too many different glyphs for simple font " + fnt.Sfnt.FontName)
	}

	// Determine the subset of glyphs to include.
	var mapping []font.CMapEntry
	for origGid, charCode := range fnt.enc {
		mapping = append(mapping, font.CMapEntry{
			CharCode: uint16(charCode),
			GID:      origGid,
		})
	}
	if len(mapping) == 0 {
		// It is not clear how a font with no glyphs should be included
		// in a PDF file.  In order to avoid problems, add a dummy glyph.
		mapping = append(mapping, font.CMapEntry{
			CharCode: 0,
			GID:      0,
		})
	}
	sort.Slice(mapping, func(i, j int) bool { return mapping[i].CharCode < mapping[j].CharCode })
	firstCharCode := mapping[0].CharCode
	lastCharCode := mapping[len(mapping)-1].CharCode
	_, includeGlyphs := font.MakeSubset(mapping)
	subsetTag := font.GetSubsetTag(includeGlyphs, len(fnt.Sfnt.Width))
	fontName := pdf.Name(subsetTag + "+" + fnt.Sfnt.FontName)

	r, err := fnt.Sfnt.GetTableReader("CFF ", nil)
	if err != nil {
		return err
	}
	cff, err := cff.Read(r)
	if err != nil {
		return err
	}
	cff = cff.Subset(includeGlyphs)

	q := 1000 / float64(fnt.Sfnt.GlyphUnits)
	FontBBox := &pdf.Rectangle{
		LLx: math.Round(float64(fnt.Sfnt.FontBBox.LLx) * q),
		LLy: math.Round(float64(fnt.Sfnt.FontBBox.LLy) * q),
		URx: math.Round(float64(fnt.Sfnt.FontBBox.URx) * q),
		URy: math.Round(float64(fnt.Sfnt.FontBBox.URy) * q),
	}

	FontDescriptorRef := w.Alloc()
	WidthsRef := w.Alloc()
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

	Font := pdf.Dict{ // See section 9.6.2.1 of PDF 32000-1:2008.
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("Type1"),
		"BaseFont":       fontName,
		"FirstChar":      pdf.Integer(firstCharCode),
		"LastChar":       pdf.Integer(lastCharCode),
		"FontDescriptor": FontDescriptorRef,
		"Widths":         WidthsRef,
		"ToUnicode":      ToUnicodeRef,
	}

	FontDescriptor := pdf.Dict{ // See section 9.8.1 of PDF 32000-1:2008.
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(fnt.Sfnt.Flags),
		"FontBBox":    FontBBox,
		"ItalicAngle": pdf.Number(fnt.Sfnt.ItalicAngle),
		"Ascent":      pdf.Integer(q*float64(fnt.Sfnt.Ascent) + 0.5),
		"Descent":     pdf.Integer(q*float64(fnt.Sfnt.Descent) + 0.5),
		"CapHeight":   pdf.Integer(q*float64(fnt.Sfnt.CapHeight) + 0.5),
		"StemV":       pdf.Integer(70), // information not available in sfnt files
		"FontFile3":   FontFileRef,
	}

	var Widths pdf.Array
	pos := 0
	for i := firstCharCode; i <= lastCharCode; i++ {
		width := 0
		if i == mapping[pos].CharCode {
			gid := mapping[pos].GID
			width = int(float64(fnt.Sfnt.Width[gid])*q + 0.5)
			pos++
		}
		Widths = append(Widths, pdf.Integer(width))
	}

	_, err = w.WriteCompressed(
		[]*pdf.Reference{fnt.FontRef, FontDescriptorRef, WidthsRef},
		Font, FontDescriptor, Widths)
	if err != nil {
		return err
	}

	// write all the streams

	// Write the font file itself.
	// See section 9.9 of PDF 32000-1:2008 for details.
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("Type1C"),
	}
	fontFileStream, _, err := w.OpenStream(fontFileDict, FontFileRef,
		&pdf.FilterInfo{Name: "FlateDecode"})
	if err != nil {
		return err
	}

	err = cff.Encode(fontFileStream)
	if err != nil {
		return err
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	var cc2text []font.SimpleMapping
	for gid, text := range fnt.text {
		charCode := fnt.enc[gid]
		cc2text = append(cc2text, font.SimpleMapping{CharCode: charCode, Text: text})
	}
	err = font.WriteToUnicodeSimple(w, subsetTag, cc2text, ToUnicodeRef)
	if err != nil {
		return err
	}

	err = fnt.Sfnt.Close()
	if err != nil {
		return err
	}

	return err
}
