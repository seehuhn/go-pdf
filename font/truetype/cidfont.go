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
	"seehuhn.de/go/pdf/font/sfnt/info"
	"seehuhn.de/go/pdf/locale"
)

// EmbedCID embeds a TrueType font into a pdf file as a CIDFont.
//
// In comparison, fonts embedded via EmbedSimple() lead to smaller PDF files,
// but only up to 256 glyphs of the font can be via the returned font object.
//
// Use of TrueType-based CIDFonts in PDF requires PDF version 1.3 or higher.
func EmbedCID(w *pdf.Writer, instName string, fileName string, loc *locale.Locale) (*font.Font, error) {
	tt, err := sfnt.Open(fileName)
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
// but only up to 256 glyphs of the font can be via the returned font object.
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
	w.OnClose(t.WriteFontDict)

	res := &font.Font{
		InstName: pdf.Name(t.InstName),
		Ref:      t.Ref,
		Layout:   t.Layout,
		Enc:      t.Enc,

		GlyphUnits:  t.Info.GlyphUnits,
		Ascent:      t.Info.Ascent,
		Descent:     t.Info.Descent,
		GlyphExtent: t.Info.GlyphExtent,
		Width:       t.Info.Width,
	}
	return res, nil
}

type ttfCID struct {
	Ttf      *sfnt.Font
	InstName string
	Ref      *pdf.Reference

	Info *info.Info
}

func newTtfCID(w *pdf.Writer, tt *sfnt.Font, instName string, loc *locale.Locale) (*ttfCID, error) {
	if !tt.IsTrueType() {
		return nil, errors.New("not a TrueType font")
	}

	info, err := info.GetInfo(tt, loc)
	if err != nil {
		return nil, err
	}

	res := &ttfCID{
		Ttf:      tt,
		InstName: instName,
		Ref:      w.Alloc(),
		Info:     info,
	}

	return res, nil
}

func (t *ttfCID) Layout(rr []rune) ([]font.Glyph, error) {
	gg := make([]font.Glyph, len(rr))
	for i, r := range rr {
		gid, ok := t.Info.CMap[r]
		if !ok {
			return nil, fmt.Errorf("font %q cannot encode rune %04x %q",
				t.Info.FontName, r, string([]rune{r}))
		}
		gg[i].Gid = gid
		gg[i].Chars = []rune{r}
	}

	gg = t.Info.GSUB.ApplyAll(gg)
	for i := range gg {
		gg[i].Advance = t.Info.Width[gg[i].Gid]
	}
	gg = t.Info.GPOS.ApplyAll(gg)

	if t.Info.KernInfo != nil {
		for i := 0; i+1 < len(gg); i++ {
			pair := font.GlyphPair{gg[i].Gid, gg[i+1].Gid}
			if dx, ok := t.Info.KernInfo[pair]; ok {
				gg[i].Advance += dx
			}
		}
	}

	return gg, nil
}

func (t *ttfCID) Enc(gid font.GlyphID) pdf.String {
	return pdf.String{byte(gid >> 8), byte(gid)}
}

func (t *ttfCID) WriteFontDict(w *pdf.Writer) error {
	subsetTag := makeSubsetTag()

	FontDescriptorRef, err := t.WriteFontDescriptor(w, subsetTag)
	if err != nil {
		return err
	}

	// TODO(voss): make sure there is only one copy of this per PDF file.
	CIDSystemInfoRef, err := w.Write(pdf.Dict{
		"Registry":   pdf.String("Adobe"),
		"Ordering":   pdf.String("Identity"),
		"Supplement": pdf.Integer(0),
	}, nil)
	if err != nil {
		return err
	}

	DW, W := font.EncodeWidths(t.Info.Width)
	WRefs, err := w.WriteCompressed(nil, W)
	if err != nil {
		return err
	}

	CIDFont := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("CIDFontType2"),
		"BaseFont":       pdf.Name(subsetTag + "+" + t.Info.FontName),
		"CIDSystemInfo":  CIDSystemInfoRef,
		"FontDescriptor": FontDescriptorRef,
		"W":              WRefs[0],
	}
	if DW != 1000 {
		CIDFont["DW"] = pdf.Integer(DW)
	}

	CIDFontRef, err := w.Write(CIDFont, nil)
	if err != nil {
		return err
	}

	mm := make(map[uint16]rune)
	for r, gid := range t.Info.CMap {
		mm[uint16(gid)] = r
	}
	ToUnicodeRef, err := font.ToUnicodeCIDFont(w, mm)
	if err != nil {
		return err
	}

	FontDict := pdf.Dict{
		"Type":            pdf.Name("Font"),
		"Subtype":         pdf.Name("Type0"),
		"BaseFont":        pdf.Name(subsetTag + "+" + t.Info.FontName),
		"Encoding":        pdf.Name("Identity-H"),
		"DescendantFonts": pdf.Array{CIDFontRef},
		"ToUnicode":       ToUnicodeRef,
	}
	_, err = w.Write(FontDict, t.Ref)
	if err != nil {
		return err
	}
	return nil
}

func (t *ttfCID) WriteFontDescriptor(w *pdf.Writer, subsetTag string) (*pdf.Reference, error) {
	fontFileReg, err := t.WriteFontFile(w)
	if err != nil {
		return nil, err
	}

	q := 1000 / float64(t.Info.GlyphUnits)

	FontDescriptor := pdf.Dict{
		"Type":     pdf.Name("FontDescriptor"),
		"FontName": pdf.Name(subsetTag + "+" + t.Info.FontName),
		"Flags":    pdf.Integer(t.Info.Flags),
		"FontBBox": &pdf.Rectangle{
			LLx: math.Round(float64(t.Ttf.Head.XMin) * q),
			LLy: math.Round(float64(t.Ttf.Head.YMin) * q),
			URx: math.Round(float64(t.Ttf.Head.XMax) * q),
			URy: math.Round(float64(t.Ttf.Head.YMax) * q),
		},
		"ItalicAngle": pdf.Number(t.Info.ItalicAngle),
		"Ascent":      pdf.Integer(q*float64(t.Info.Ascent) + 0.5),
		"Descent":     pdf.Integer(q*float64(t.Info.Descent) + 0.5),
		"CapHeight":   pdf.Integer(q*float64(t.Info.CapHeight) + 0.5),
		"StemV":       pdf.Integer(70),
		"FontFile2":   fontFileReg,
	}
	return w.Write(FontDescriptor, nil)
}

func (t *ttfCID) WriteFontFile(w *pdf.Writer) (*pdf.Reference, error) {
	size := w.NewPlaceholder(10)
	dict := pdf.Dict{
		"Length1": size, // TODO(voss): maybe only needed for Subtype=TrueType?
	}
	stm, FontFile, err := w.OpenStream(dict, nil,
		&pdf.FilterInfo{Name: "FlateDecode"})
	if err != nil {
		return nil, err
	}
	exOpt := &sfnt.ExportOptions{
		Include: map[string]bool{
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

	return FontFile, nil
}
