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
// Use of OpenType fonts in PDF requires PDF version 1.6 or higher.
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
// In comparison, fonts embedded via EmbedSimple() lead to smaller PDF files,
// but only up to 256 glyphs of the font can be accessed via the returned font
// object.
//
// Use of OpenType fonts in PDF requires PDF version 1.6 or higher.
func EmbedFontCID(w *pdf.Writer, tt *sfnt.Font, instName pdf.Name) (*font.Font, error) {
	if !tt.IsOpenType() {
		return nil, errors.New("not an OpenType font")
	}
	if tt.IsTrueType() {
		return truetype.EmbedFontCID(w, tt, instName)
	}
	err := w.CheckVersion("use of OpenType fonts", pdf.V1_6)
	if err != nil {
		return nil, err
	}

	r, err := tt.GetTableReader("CFF ", nil)
	if err != nil {
		return nil, err
	}
	cff, err := cff.ReadCFF(r)
	if err != nil {
		return nil, err
	}

	t := &otfCID{
		Otf: tt,
		Cff: cff,

		FontRef: w.Alloc(),

		text: make(map[font.GlyphID][]rune),
		used: map[font.GlyphID]bool{
			0: true, // always include the .notdef glyph
		},
	}

	w.OnClose(t.WriteFont)

	res := &font.Font{
		InstName: instName,
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
	Otf *sfnt.Font
	Cff *cff.Font

	FontRef *pdf.Reference

	text map[font.GlyphID][]rune // GID -> text
	used map[font.GlyphID]bool   // is GID used?
}

func (t *otfCID) Layout(rr []rune) ([]font.Glyph, error) {
	gg := make([]font.Glyph, len(rr))
	for i, r := range rr {
		gid, ok := t.Otf.CMap[r]
		if !ok {
			return nil, fmt.Errorf("font %q cannot encode rune %04x %q",
				t.Otf.FontName, r, string([]rune{r}))
		}
		gg[i].Gid = gid
		gg[i].Chars = []rune{r}
	}

	gg = t.Otf.GSUB.ApplyAll(gg)
	for i := range gg {
		gg[i].Advance = t.Otf.Width[gg[i].Gid]
	}
	gg = t.Otf.GPOS.ApplyAll(gg)

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
	// TODO(voss): implement subsetting

	fontName := pdf.Name(t.Cff.FontName)

	DW, W := font.EncodeCIDWidths(t.Otf.Width)

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
	// TODO(voss): if tt.Otf.CFF.IsCIDFont is true, use the values
	//     from the ROS operator?
	CIDSystemInfo := pdf.Dict{ // See sections 9.7.3 of PDF 32000-1:2008.
		"Registry":   pdf.String("Adobe"),
		"Ordering":   pdf.String("Identity"),
		"Supplement": pdf.Integer(0),
	}

	q := 1000 / float64(t.Otf.GlyphUnits)
	FontDescriptor := pdf.Dict{ // See sections 9.8.1 of PDF 32000-1:2008.
		"Type":     pdf.Name("FontDescriptor"),
		"FontName": fontName,
		"Flags":    pdf.Integer(t.Otf.Flags),
		"FontBBox": &pdf.Rectangle{
			LLx: math.Round(float64(t.Otf.FontBBox.LLx) * q),
			LLy: math.Round(float64(t.Otf.FontBBox.LLy) * q),
			URx: math.Round(float64(t.Otf.FontBBox.URx) * q),
			URy: math.Round(float64(t.Otf.FontBBox.URy) * q),
		},
		"ItalicAngle": pdf.Number(t.Otf.ItalicAngle),
		"Ascent":      pdf.Integer(q*float64(t.Otf.Ascent) + 0.5),
		"Descent":     pdf.Integer(q*float64(t.Otf.Descent) + 0.5),
		"CapHeight":   pdf.Integer(q*float64(t.Otf.CapHeight) + 0.5),
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
			"CFF ": true,
			"cmap": true,
		},
	}
	_, err = t.Otf.Export(fontFileStream, exOpt)
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

	err = t.Otf.Close()
	if err != nil {
		return err
	}

	return nil
}
