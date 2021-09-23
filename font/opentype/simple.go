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
	"sort"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/locale"
)

// EmbedSimple embeds an OpenType font into a pdf file as a simple font.
// Up to 256 arbitrary glyphs from the font file can be accessed via the
// returned font object.
//
// In comparison, fonts embedded via EmbedCID lead to larger PDF files, but
// there is no limit on the number of glyphs which can be accessed.
//
// Use of OpenType fonts in PDF requires PDF version 1.6 or higher.
func EmbedSimple(w *pdf.Writer, instName string, fileName string, loc *locale.Locale) (*font.Font, error) {
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
// Use of OpenType fonts in PDF requires PDF version 1.6 or higher.
func EmbedFontSimple(w *pdf.Writer, tt *sfnt.Font, instName string) (*font.Font, error) {
	err := w.CheckVersion("use of OpenType fonts", pdf.V1_6)
	if err != nil {
		return nil, err
	}

	t, err := newOtfSimple(w, tt, instName)
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

type otfSimple struct {
	Otf *sfnt.Font

	FontRef           *pdf.Reference
	FontDescriptorRef *pdf.Reference
	WidthsRef         *pdf.Reference
	ToUnicodeRef      *pdf.Reference
	FontFileRef       *pdf.Reference

	text map[font.GlyphID][]rune // GID -> text
	enc  map[font.GlyphID]byte   // GID -> CharCode
	tidy map[font.GlyphID]byte   // GID -> candidate CharCode
	used map[byte]bool           // is CharCode used or not?

	overflowed bool
}

func newOtfSimple(w *pdf.Writer, tt *sfnt.Font, instName string) (*otfSimple, error) {
	if !tt.IsOpenType() {
		return nil, errors.New("not an OpenType font")
	}
	if !tt.HasTables("glyf") {
		return nil, errors.New("CFF-based OpenType fonts not supported")
	}

	// TODO(voss): "... conforming writers, instead of using a simple font,
	// shall use a Type 0 font with an Identity-H encoding and use the glyph
	// indices as character codes ..."
	// (bottom of page 291 of PDF 32000-1:2008)

	tidy := make(map[font.GlyphID]byte)
	for r, gid := range tt.CMap {
		if rOld, used := tidy[gid]; r < 127 && (!used || byte(r) < rOld) {
			tidy[gid] = byte(r)
		}
	}

	res := &otfSimple{
		Otf: tt,

		FontRef:           w.Alloc(),
		FontDescriptorRef: w.Alloc(),
		WidthsRef:         w.Alloc(),
		ToUnicodeRef:      w.Alloc(),
		FontFileRef:       w.Alloc(),

		text: make(map[font.GlyphID][]rune),
		enc:  make(map[font.GlyphID]byte),
		tidy: tidy,
		used: map[byte]bool{},
	}

	return res, nil
}

func (t *otfSimple) Layout(rr []rune) ([]font.Glyph, error) {
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

func (t *otfSimple) Enc(gid font.GlyphID) pdf.String {
	c, ok := t.enc[gid]
	if ok {
		return pdf.String{c}
	}

	// allocate a new character code
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

func (t *otfSimple) WriteFont(w *pdf.Writer) error {
	if t.overflowed {
		return errors.New("too many different glyphs for simple font " + t.Otf.FontName)
	}

	var mapping []font.CMapEntry
	for origGid, charCode := range t.enc {
		if origGid == 0 {
			continue
		}
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

	subsetMapping, includeGlyphs := font.MakeSubset(mapping)

	// Compute the font bounding box for the subset.
	left := math.MaxInt
	right := math.MinInt
	top := math.MinInt
	bottom := math.MaxInt
	for _, origGid := range includeGlyphs {
		if origGid == 0 {
			continue
		}
		box := t.Otf.GlyphExtent[origGid]
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

	q := 1000 / float64(t.Otf.GlyphUnits)
	FontBBox := &pdf.Rectangle{
		LLx: math.Round(float64(left) * q),
		LLy: math.Round(float64(bottom) * q),
		URx: math.Round(float64(right) * q),
		URy: math.Round(float64(top) * q),
	}

	var cc2text []font.SimpleMapping
	for gid, text := range t.text {
		charCode := t.enc[gid]
		cc2text = append(cc2text, font.SimpleMapping{CharCode: charCode, Text: text})
	}

	subsetTag := font.GetSubsetTag(includeGlyphs, len(t.Otf.Width))
	fontName := pdf.Name(subsetTag + "+" + t.Otf.FontName)

	// See sections 9.6.2.1 and 9.6.3 of PDF 32000-1:2008.
	Font := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("TrueType"),
		"BaseFont":       fontName,
		"FirstChar":      pdf.Integer(firstCharCode),
		"LastChar":       pdf.Integer(lastCharCode),
		"FontDescriptor": t.FontDescriptorRef,
		"Widths":         t.WidthsRef,
		"ToUnicode":      t.ToUnicodeRef,
	}

	// See sections 9.8.1 of PDF 32000-1:2008.
	FontDescriptor := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(t.Otf.Flags),
		"FontBBox":    FontBBox,
		"ItalicAngle": pdf.Number(t.Otf.ItalicAngle),
		"Ascent":      pdf.Integer(q*float64(t.Otf.Ascent) + 0.5),
		"Descent":     pdf.Integer(q*float64(t.Otf.Descent) + 0.5),
		"CapHeight":   pdf.Integer(q*float64(t.Otf.CapHeight) + 0.5),
		"StemV":       pdf.Integer(70), // information not available in otf files
		"FontFile3":   t.FontFileRef,
	}

	var Widths pdf.Array
	pos := 0
	for i := firstCharCode; i <= lastCharCode; i++ {
		width := 0
		if i == mapping[pos].CharCode {
			gid := mapping[pos].GID
			width = int(float64(t.Otf.Width[gid])*q + 0.5)
			pos++
		}
		Widths = append(Widths, pdf.Integer(width))
	}

	_, err := w.WriteCompressed(
		[]*pdf.Reference{t.FontRef, t.FontDescriptorRef, t.WidthsRef},
		Font, FontDescriptor, Widths)
	if err != nil {
		return err
	}

	err = font.WriteToUnicodeSimple(w, subsetTag, cc2text, t.ToUnicodeRef)
	if err != nil {
		return err
	}

	// Finally, write the font file itself.
	// See section 9.9 of PDF 32000-1:2008 for details.
	fontFileDict := pdf.Dict{
		"Subtype": pdf.Name("OpenType"),
	}
	fontFileStream, _, err := w.OpenStream(fontFileDict, t.FontFileRef,
		&pdf.FilterInfo{Name: "FlateDecode"})
	if err != nil {
		return err
	}
	exOpt := &sfnt.ExportOptions{
		IncludeTables: map[string]bool{
			// The list of tables to include is from PDF 32000-1:2008, table 126.
			"cvt ": true, // copy
			"fpgm": true, // copy
			"glyf": true, // rewrite
			"head": true, // update CheckSumAdjustment, Modified and indexToLocFormat
			"hhea": true, // update various fields, including numberOfHMetrics
			"hmtx": true, // rewrite
			"loca": true, // rewrite
			"maxp": true, // update numGlyphs
			"prep": true, // copy

			// We use a CMap to map character codes to Glyph IDs
			"cmap": true, // generate
		},
		SubsetMapping: subsetMapping,
		IncludeGlyphs: includeGlyphs,
	}
	_, err = t.Otf.Export(fontFileStream, exOpt)
	if err != nil {
		return err
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	err = t.Otf.Close()
	if err != nil {
		return err
	}

	return err
}
