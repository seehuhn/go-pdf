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
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/locale"
)

// EmbedCID embeds an OpenType font into a pdf file as a CIDFont.
//
// Use of CFF-based OpenType fonts in PDF requires PDF version 1.6 or higher.
// Glyf-based OpenType fonts will be embeded as TrueType fonts, so require
// only PDF version 1.3 or higher.
func EmbedCID(w *pdf.Writer, instName string, fileName string, loc *locale.Locale) (*font.Font, error) {
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
// Use of CFF-based OpenType fonts in PDF requires PDF version 1.6 or higher.
// Glyf-based OpenType fonts will be embeded as TrueType fonts, so require
// only PDF version 1.3 or higher.
func EmbedFontCID(w *pdf.Writer, tt *sfnt.Font, instName string) (*font.Font, error) {
	if tt.IsTrueType() {
		return truetype.EmbedFontCID(w, tt, instName)
	}

	err := w.CheckVersion("use of CFF-based OpenType fonts", pdf.V1_6)
	if err != nil {
		return nil, err
	}

	t, err := newOtfCID(w, tt, instName)
	if err != nil {
		return nil, err
	}

	w.OnClose(t.WriteFont)

	res := &font.Font{
		InstName: pdf.Name(instName),
		Ref:      t.FontRef,

		GlyphUnits:  tt.GlyphUnits,
		Ascent:      tt.Ascent,
		Descent:     tt.Descent,
		GlyphExtent: tt.GlyphExtent,
		Width:       tt.Width,

		Layout: t.Layout,
		Enc:    t.Enc,
	}
	return res, nil
}

type otfCID struct {
	Ttf *sfnt.Font

	FontRef *pdf.Reference

	text map[font.GlyphID][]rune // GID -> text
	used map[font.GlyphID]bool   // is GID used?
}

func newOtfCID(w *pdf.Writer, tt *sfnt.Font, instName string) (*otfCID, error) {
	if !tt.IsOpenType() {
		return nil, errors.New("not an OpenType font")
	}

	res := &otfCID{
		Ttf: tt,

		FontRef: w.Alloc(),

		text: make(map[font.GlyphID][]rune),
		used: map[font.GlyphID]bool{
			0: true, // always include the .notdef glyph
		},
	}

	return res, nil
}

func (t *otfCID) Layout(rr []rune) ([]font.Glyph, error) {
	gg := make([]font.Glyph, len(rr))
	for i, r := range rr {
		gid, ok := t.Ttf.CMap[r]
		if !ok {
			return nil, fmt.Errorf("font %q cannot encode rune %04x %q",
				t.Ttf.FontName, r, string([]rune{r}))
		}
		gg[i].Gid = gid
		gg[i].Chars = []rune{r}
	}

	gg = t.Ttf.GSUB.ApplyAll(gg)
	for i := range gg {
		gg[i].Advance = t.Ttf.Width[gg[i].Gid]
	}
	gg = t.Ttf.GPOS.ApplyAll(gg)

	for _, g := range gg {
		if _, seen := t.text[g.Gid]; !seen && len(g.Chars) > 0 {
			// copy the slice, in case the caller modifies it later
			t.text[g.Gid] = append([]rune{}, g.Chars...)
		}
	}

	return gg, nil
}

func (t *otfCID) Enc(gid font.GlyphID) pdf.String {
	t.used[gid] = true
	return pdf.String{byte(gid >> 8), byte(gid)}
}

func (t *otfCID) WriteFont(w *pdf.Writer) error {
	// TODO(voss): implement font subsetting

	fontName := pdf.Name(t.Ttf.FontName)

	DW, W := font.EncodeCIDWidths(t.Ttf.Width)

	CIDFontRef := w.Alloc()
	CIDSystemInfoRef := w.Alloc()
	FontDescriptorRef := w.Alloc()
	WRef := w.Alloc()
	FontFileRef := w.Alloc()
	ToUnicodeRef := w.Alloc()

	Font := pdf.Dict{ // See section 9.7.6.1 of PDF 32000-1:2008.
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        fontName,
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

	q := 1000 / float64(t.Ttf.GlyphUnits)
	FontDescriptor := pdf.Dict{ // See sections 9.8.1 of PDF 32000-1:2008.
		"Type":     pdf.Name("FontDescriptor"),
		"FontName": fontName,
		"Flags":    pdf.Integer(t.Ttf.Flags),
		"FontBBox": &pdf.Rectangle{
			LLx: math.Round(float64(t.Ttf.FontBBox.LLx) * q),
			LLy: math.Round(float64(t.Ttf.FontBBox.LLy) * q),
			URx: math.Round(float64(t.Ttf.FontBBox.URx) * q),
			URy: math.Round(float64(t.Ttf.FontBBox.URy) * q),
		},
		"ItalicAngle": pdf.Number(t.Ttf.ItalicAngle),
		"Ascent":      pdf.Integer(q*float64(t.Ttf.Ascent) + 0.5),
		"Descent":     pdf.Integer(q*float64(t.Ttf.Descent) + 0.5),
		"CapHeight":   pdf.Integer(q*float64(t.Ttf.CapHeight) + 0.5),
		"StemV":       pdf.Integer(70), // information not available in ttf files
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
		"Subtype": pdf.Name("OpenType"),
	}
	fontFileStream, _, err := w.OpenStream(fontFileDict, FontFileRef,
		&pdf.FilterInfo{Name: "FlateDecode"})
	if err != nil {
		return err
	}
	exOpt := &sfnt.ExportOptions{
		IncludeTables: map[string]bool{
			// The list of tables to include is from PDF 32000-1:2008, table 126.
			// "cvt ": true, // copy
			// "fpgm": true, // copy
			// "prep": true, // copy
			// "head": true, // update CheckSumAdjustment, Modified and indexToLocFormat
			// "hhea": true, // update various fields, including numberOfHMetrics
			// "maxp": true, // update numGlyphs
			// "hmtx": true, // rewrite
			"CFF ": true,
			"cmap": true,
		},
	}
	_, err = t.Ttf.Export(fontFileStream, exOpt)
	if err != nil {
		return err
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	var cc2text []font.CIDMapping
	for gid, text := range t.text {
		cc2text = append(cc2text, font.CIDMapping{CharCode: uint16(gid), Text: text})
	}
	err = font.WriteToUnicodeCID(w, cc2text, ToUnicodeRef)
	if err != nil {
		return err
	}

	err = t.Ttf.Close()
	if err != nil {
		return err
	}

	return nil
}
