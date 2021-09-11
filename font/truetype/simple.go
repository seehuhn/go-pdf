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
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/info"
	"seehuhn.de/go/pdf/locale"
)

// EmbedSimple embeds a TrueType font into a pdf file as a simple font.
// Up to 255 arbitrary glyphs from the font file can be accessed via the
// returned font object.
//
// In comparison, fonts embedded via EmbedCID lead to larger PDF files, but
// there is no limit on the number of glyphs which can be accessed.
//
// Use of simple TrueType fonts in PDF requires PDF version 1.1 or higher.
func EmbedSimple(w *pdf.Writer, instName string, fileName string, loc *locale.Locale) (*font.Font, error) {
	tt, err := sfnt.Open(fileName)
	if err != nil {
		return nil, err
	}

	return EmbedFontSimple(w, tt, instName, loc)
}

// EmbedFontSimple embeds a TrueType font into a pdf file as a simple font.
// Up to 255 arbitrary glyphs from the font file can be accessed via the
// returned font object.
//
// This function takes ownership of tt and will close the font tt once it is no
// longer needed.
//
// In comparison, fonts embedded via EmbedFontCID lead to larger PDF files, but
// there is no limit on the number of glyphs which can be accessed.
//
// Use of simple TrueType fonts in PDF requires PDF version 1.1 or higher.
func EmbedFontSimple(w *pdf.Writer, tt *sfnt.Font, instName string, loc *locale.Locale) (*font.Font, error) {
	err := w.CheckVersion("use of TrueType fonts", pdf.V1_1)
	if err != nil {
		return nil, err
	}

	t, err := newTtfSimple(w, tt, instName, loc)
	if err != nil {
		return nil, err
	}
	w.OnClose(t.WriteFontDict)

	res := &font.Font{
		InstName:    pdf.Name(t.InstName),
		Ref:         t.Ref,
		Layout:      t.Layout,
		Enc:         t.Enc,
		GlyphUnits:  t.Info.GlyphUnits,
		Ascent:      t.Info.Ascent,
		Descent:     t.Info.Descent,
		GlyphExtent: t.Info.GlyphExtent,
		Width:       t.Info.Width,
	}
	return res, nil
}

type ttfSimple struct {
	Ttf      *sfnt.Font
	InstName string
	Ref      *pdf.Reference

	Info *info.Info

	enc  map[font.GlyphID]byte   // GID -> CID
	used map[byte]bool           // is CID used or not?
	text map[font.GlyphID][]rune // GID -> text
	tidy map[font.GlyphID]byte   // GID -> candidate CID

	overflowed bool
}

func newTtfSimple(w *pdf.Writer, tt *sfnt.Font, instName string, loc *locale.Locale) (*ttfSimple, error) {
	if !tt.IsTrueType() {
		return nil, errors.New("not a TrueType font")
	}

	info, err := info.GetInfo(tt, loc)
	if err != nil {
		return nil, err
	}

	tidy := make(map[font.GlyphID]byte)
	for r, gid := range info.CMap {
		if rOld, used := tidy[gid]; r < 127 && (!used || byte(r) < rOld) {
			tidy[gid] = byte(r)
		}
	}

	res := &ttfSimple{
		Ttf:      tt,
		Ref:      w.Alloc(),
		InstName: instName,

		Info: info,

		enc:  make(map[font.GlyphID]byte),
		used: map[byte]bool{},
		text: make(map[font.GlyphID][]rune),
		tidy: tidy,
	}

	return res, nil
}

func (t *ttfSimple) Layout(rr []rune) ([]font.Glyph, error) {
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

	for _, g := range gg {
		if _, seen := t.text[g.Gid]; !seen && len(g.Chars) > 0 {
			// copy the slice, in case the caller modifies it later
			t.text[g.Gid] = append([]rune{}, g.Chars...)
		}
	}

	return gg, nil
}

func (t *ttfSimple) Enc(gid font.GlyphID) pdf.String {
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
		//
		// TODO(voss): turn this into an error returned by .Layout()?
		t.overflowed = true
		t.enc[gid] = 0
		return pdf.String{0}
	}

	t.used[c] = true
	t.enc[gid] = c
	return pdf.String{c}
}

func (t *ttfSimple) WriteFontDict(w *pdf.Writer) error {
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

	subsetTag := makeSubsetTag()
	fontDesc, err := t.WriteFontDescriptor(w, subset, subsetTag)
	if err != nil {
		return err
	}

	var ww pdf.Array
	q := 1000 / float64(t.Info.GlyphUnits)
	for i := first; i <= last; i++ {
		gid := subset[i]
		width := int(float64(t.Info.Width[gid])*q + 0.5)
		if !t.used[byte(i)] {
			width = 0
		}
		ww = append(ww, pdf.Integer(width))
	}
	widths, err := w.Write(ww, nil)
	if err != nil {
		return err
	}

	var mm []font.SimpleMapping
	for gid, text := range t.text {
		cid := t.enc[gid]
		mm = append(mm, font.SimpleMapping{Cid: cid, Text: text})
	}
	toUnicodeRef, err := font.ToUnicodeSimple(w, subsetTag, mm)
	if err != nil {
		return err
	}

	// See sections 9.6.2.1 and 9.6.3 of PDF 32000-1:2008.
	Font := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("TrueType"),
		"BaseFont":       pdf.Name(subsetTag + "+" + t.Info.FontName),
		"FirstChar":      pdf.Integer(first),
		"LastChar":       pdf.Integer(last),
		"Widths":         widths,
		"FontDescriptor": fontDesc,
		"ToUnicode":      toUnicodeRef,
	}

	_, err = w.Write(Font, t.Ref)
	if err != nil {
		return err
	}

	err = t.Ttf.Close()
	if err != nil {
		return err
	}

	return err
}

func (t *ttfSimple) WriteFontDescriptor(w *pdf.Writer,
	subset []font.GlyphID, subsetTag string) (*pdf.Reference, error) {

	fontFileRef, err := t.WriteFontFile(w, subset)
	if err != nil {
		return nil, err
	}

	// compute the font bounding box for the subset
	left := math.MaxInt
	right := math.MinInt
	top := math.MinInt
	bottom := math.MaxInt
	for _, origGid := range subset {
		box := t.Info.GlyphExtent[origGid]
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

	q := 1000 / float64(t.Info.GlyphUnits)
	FontBBox := &pdf.Rectangle{
		LLx: math.Round(float64(left) * q),
		LLy: math.Round(float64(bottom) * q),
		URx: math.Round(float64(right) * q),
		URy: math.Round(float64(top) * q),
	}

	// See sections 9.8.1 of PDF 32000-1:2008.
	FontDescriptor := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    pdf.Name(subsetTag + "+" + t.Info.FontName),
		"Flags":       pdf.Integer(t.Info.Flags),
		"FontBBox":    FontBBox,
		"ItalicAngle": pdf.Number(t.Info.ItalicAngle),
		"Ascent":      pdf.Integer(q*float64(t.Info.Ascent) + 0.5),
		"Descent":     pdf.Integer(q*float64(t.Info.Descent) + 0.5),
		"CapHeight":   pdf.Integer(q*float64(t.Info.CapHeight) + 0.5),
		"StemV":       pdf.Integer(70),
		"FontFile2":   fontFileRef,
	}
	return w.Write(FontDescriptor, nil)
}

func (t *ttfSimple) WriteFontFile(w *pdf.Writer, subset []font.GlyphID) (*pdf.Reference, error) {
	// See section 9.9 of PDF 32000-1:2008.
	size := w.NewPlaceholder(10)
	fontFileDict := pdf.Dict{
		"Length1": size,
	}
	fontFileStream, fontFile, err := w.OpenStream(fontFileDict, nil,
		&pdf.FilterInfo{Name: "FlateDecode"})
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

func makeSubsetTag() string {
	var letters []rune
	t := time.Now().UnixNano() // TODO(voss): be more clever here?
	for len(letters) < 6 {
		letters = append(letters, rune(t%26)+'A')
		t /= 26
	}
	return string(letters)
}
