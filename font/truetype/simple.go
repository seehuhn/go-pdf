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
	tt, err := sfnt.Open(fileName, loc)
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
		InstName: pdf.Name(instName),
		Ref:      t.Ref,

		GlyphUnits:  t.Ttf.GlyphUnits,
		Ascent:      t.Ttf.Ascent,
		Descent:     t.Ttf.Descent,
		GlyphExtent: t.Ttf.GlyphExtent,
		Width:       t.Ttf.Width,

		Layout: t.Layout,
		Enc:    t.Enc,
	}
	return res, nil
}

type ttfSimple struct {
	Ttf *sfnt.Font
	Ref *pdf.Reference

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

	tidy := make(map[font.GlyphID]byte)
	for r, gid := range tt.CMap {
		if rOld, used := tidy[gid]; r < 127 && (!used || byte(r) < rOld) {
			tidy[gid] = byte(r)
		}
	}

	res := &ttfSimple{
		Ttf: tt,
		Ref: w.Alloc(),

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
		return errors.New("too many different glyphs for simple font " + t.Ttf.FontName)
	}

	// TODO(voss): cid2gid is passed down a long call chain.  Can this
	// be simplified?
	cid2gid := make([]font.GlyphID, 256)
	first := 257
	last := -1
	for origGid, c := range t.enc {
		if int(c) < first {
			first = int(c)
		}
		if int(c) > last {
			last = int(c)
		}
		cid2gid[c] = origGid
	}
	subsetTag := makeSubsetTag()

	fontDesc, err := t.WriteFontDescriptor(w, cid2gid, subsetTag)
	if err != nil {
		return err
	}

	var ww pdf.Array
	q := 1000 / float64(t.Ttf.GlyphUnits)
	for i := first; i <= last; i++ {
		width := 0
		if t.used[byte(i)] {
			gid := cid2gid[i]
			width = int(float64(t.Ttf.Width[gid])*q + 0.5)
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
		"BaseFont":       pdf.Name(subsetTag + "+" + t.Ttf.FontName),
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
	cid2gid []font.GlyphID, subsetTag string) (*pdf.Reference, error) {

	fontFileRef, err := t.WriteFontFile(w, cid2gid)
	if err != nil {
		return nil, err
	}

	// Compute the font bounding box for the subset.
	// We always include glyph 0:
	left := t.Ttf.GlyphExtent[0].LLx
	right := t.Ttf.GlyphExtent[0].URx
	top := t.Ttf.GlyphExtent[0].URy
	bottom := t.Ttf.GlyphExtent[0].LLy
	for _, origGid := range cid2gid {
		if origGid == 0 {
			continue
		}
		box := t.Ttf.GlyphExtent[origGid]
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

	q := 1000 / float64(t.Ttf.GlyphUnits)
	FontBBox := &pdf.Rectangle{
		LLx: math.Round(float64(left) * q),
		LLy: math.Round(float64(bottom) * q),
		URx: math.Round(float64(right) * q),
		URy: math.Round(float64(top) * q),
	}

	// Following section 9.6.6.4 of PDF 32000-1:2008, for PDF versions before
	// 1.3 we mark all fonts as symbolic, so that the CMap for glyph selection
	// works.
	flags := t.Ttf.Flags
	if w.Version < pdf.V1_3 {
		flags &= ^font.FlagNonsymbolic
		flags |= font.FlagSymbolic
	}

	// See sections 9.8.1 of PDF 32000-1:2008.
	FontDescriptor := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    pdf.Name(subsetTag + "+" + t.Ttf.FontName),
		"Flags":       pdf.Integer(flags),
		"FontBBox":    FontBBox,
		"ItalicAngle": pdf.Number(t.Ttf.ItalicAngle),
		"Ascent":      pdf.Integer(q*float64(t.Ttf.Ascent) + 0.5),
		"Descent":     pdf.Integer(q*float64(t.Ttf.Descent) + 0.5),
		"CapHeight":   pdf.Integer(q*float64(t.Ttf.CapHeight) + 0.5),
		"StemV":       pdf.Integer(70),
		"FontFile2":   fontFileRef,
	}
	return w.Write(FontDescriptor, nil)
}

func (t *ttfSimple) WriteFontFile(w *pdf.Writer, cid2gid []font.GlyphID) (*pdf.Reference, error) {
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
		Cid2Gid: cid2gid,
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
