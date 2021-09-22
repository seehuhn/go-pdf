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

package truetype

import (
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/locale"
)

// EmbedCID embeds a TrueType font into a pdf file as a CIDFont.
//
// In comparison, fonts embedded via EmbedSimple() lead to smaller PDF files,
// but only up to 256 glyphs of the font can be accessed via the returned font
// object.
//
// Use of TrueType-based CIDFonts in PDF requires PDF version 1.3 or higher.
func EmbedCID(w *pdf.Writer, instName string, fileName string, loc *locale.Locale) (*font.Font, error) {
	tt, err := sfnt.Open(fileName, loc)
	if err != nil {
		return nil, err
	}

	return EmbedFontCID(w, tt, instName, loc)
}

// EmbedFontCID embeds a TrueType font into a pdf file as a CIDFont.
//
// This function takes ownership of tt and will close the font tt once it is no
// longer needed.
//
// In comparison, fonts embedded via EmbedFontSimple lead to smaller PDF files,
// but only up to 256 glyphs of the font can be accessed via the returned font
// object.
//
// Use of TrueType-based CIDFonts in PDF requires PDF version 1.3 or higher.
func EmbedFontCID(w *pdf.Writer, tt *sfnt.Font, instName string, loc *locale.Locale) (*font.Font, error) {
	err := w.CheckVersion("use of TrueType-based CIDFonts", pdf.V1_3)
	if err != nil {
		return nil, err
	}

	t, err := newTtfCID(w, tt, instName, loc)
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

type ttfCID struct {
	Ttf *sfnt.Font

	FontRef *pdf.Reference

	text map[font.GlyphID][]rune // GID -> text
	used map[uint16]bool         // is CharCode used or not?
}

func newTtfCID(w *pdf.Writer, tt *sfnt.Font, instName string, loc *locale.Locale) (*ttfCID, error) {
	if !tt.IsTrueType() {
		return nil, errors.New("not a TrueType font")
	}

	res := &ttfCID{
		Ttf: tt,

		FontRef: w.Alloc(),

		text: make(map[font.GlyphID][]rune),
		used: map[uint16]bool{},
	}

	return res, nil
}

func (t *ttfCID) Layout(rr []rune) ([]font.Glyph, error) {
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

func (t *ttfCID) Enc(gid font.GlyphID) pdf.String {
	t.used[uint16(gid)] = true
	return pdf.String{byte(gid >> 8), byte(gid)}
}

func (t *ttfCID) WriteFont(w *pdf.Writer) error {
	subsetTag := "AAAAAA" // TODO(voss)

	q := 1000 / float64(t.Ttf.GlyphUnits)

	// TODO(voss): make sure there is only one copy of this per PDF file.
	CIDSystemInfoRef, err := w.Write(pdf.Dict{
		"Registry":   pdf.String("Adobe"),
		"Ordering":   pdf.String("Identity"),
		"Supplement": pdf.Integer(0),
	}, nil)
	if err != nil {
		return err
	}

	mm := make(map[uint16]rune)
	for r, gid := range t.Ttf.CMap {
		mm[uint16(gid)] = r
	}

	DW, W := font.EncodeWidths(t.Ttf.Width)

	fontName := pdf.Name(subsetTag + "+" + t.Ttf.FontName)

	CIDFontRef := w.Alloc()
	FontDescriptorRef := w.Alloc()
	WidthsRef := w.Alloc()
	ToUnicodeRef := w.Alloc()
	FontFileRef := w.Alloc()

	Font := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        fontName,
		"Encoding":        pdf.Name("Identity-H"),
		"DescendantFonts": pdf.Array{CIDFontRef},
		"ToUnicode":       ToUnicodeRef,
	}

	CIDFont := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType2"),
		"BaseFont":       fontName,
		"CIDSystemInfo":  CIDSystemInfoRef,
		"FontDescriptor": FontDescriptorRef,
		"W":              WidthsRef,
	}
	if DW != 1000 {
		CIDFont["DW"] = pdf.Integer(DW)
	}

	FontDescriptor := pdf.Dict{
		"Type":     pdf.Name("FontDescriptor"),
		"FontName": fontName,
		"Flags":    pdf.Integer(t.Ttf.Flags),
		"FontBBox": &pdf.Rectangle{
			LLx: math.Round(float64(t.Ttf.Head.XMin) * q),
			LLy: math.Round(float64(t.Ttf.Head.YMin) * q),
			URx: math.Round(float64(t.Ttf.Head.XMax) * q),
			URy: math.Round(float64(t.Ttf.Head.YMax) * q),
		},
		"ItalicAngle": pdf.Number(t.Ttf.ItalicAngle),
		"Ascent":      pdf.Integer(q*float64(t.Ttf.Ascent) + 0.5),
		"Descent":     pdf.Integer(q*float64(t.Ttf.Descent) + 0.5),
		"CapHeight":   pdf.Integer(q*float64(t.Ttf.CapHeight) + 0.5),
		"StemV":       pdf.Integer(70),
		"FontFile2":   FontFileRef,
	}

	_, err = w.WriteCompressed(
		[]*pdf.Reference{t.FontRef, CIDFontRef, FontDescriptorRef, WidthsRef},
		Font, CIDFont, FontDescriptor, W)
	if err != nil {
		return err
	}

	err = font.WriteToUnicodeCID(w, mm, ToUnicodeRef)
	if err != nil {
		return err
	}

	// Finally, write the font file itself.
	// See section 9.9 of PDF 32000-1:2008 for details.
	size := w.NewPlaceholder(10)
	dict := pdf.Dict{
		"Length1": size, // TODO(voss): maybe only needed for Subtype=TrueType?
	}
	stm, _, err := w.OpenStream(dict, FontFileRef,
		&pdf.FilterInfo{Name: "FlateDecode"})
	if err != nil {
		return err
	}
	exOpt := &sfnt.ExportOptions{
		IncludeTables: map[string]bool{
			// The list of tables to include is from PDF 32000-1:2008, table 126.
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
	n, err := t.Ttf.Export(stm, exOpt)
	if err != nil {
		return err
	}
	err = size.Set(pdf.Integer(n))
	if err != nil {
		return err
	}
	err = stm.Close()
	if err != nil {
		return err
	}

	err = t.Ttf.Close()
	if err != nil {
		return err
	}

	return nil
}
