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

type ttfSimple struct {
	Ttf *sfnt.Font

	FontRef           *pdf.Reference
	FontDescriptorRef *pdf.Reference
	WidthsRef         *pdf.Reference
	ToUnicodeRef      *pdf.Reference
	FontFileRef       *pdf.Reference

	text map[font.GlyphID][]rune // GID -> text
	enc  map[font.GlyphID]byte   // GID -> CID
	tidy map[font.GlyphID]byte   // GID -> candidate CID
	used map[byte]bool           // is CID used or not?

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

	// allocate a new cid
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

func (t *ttfSimple) WriteFontDict(w *pdf.Writer) error {
	if t.overflowed {
		return errors.New("too many different glyphs for simple font " + t.Ttf.FontName)
	}

	// TODO(voss): cid2gid is passed down a long call chain.  Can this
	// be simplified?
	cid2gid := make([]font.GlyphID, 256)
	firstCid := 257
	lastCid := -1
	for origGid, cid := range t.enc {
		cid2gid[cid] = origGid

		if int(cid) < firstCid {
			firstCid = int(cid)
		}
		if int(cid) > lastCid {
			lastCid = int(cid)
		}
	}
	subsetTag := makeSubsetTag()

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

	var cid2text []font.SimpleMapping
	for gid, text := range t.text {
		cid := t.enc[gid]
		cid2text = append(cid2text, font.SimpleMapping{Cid: cid, Text: text})
	}

	q := 1000 / float64(t.Ttf.GlyphUnits)
	FontBBox := &pdf.Rectangle{
		LLx: math.Round(float64(left) * q),
		LLy: math.Round(float64(bottom) * q),
		URx: math.Round(float64(right) * q),
		URy: math.Round(float64(top) * q),
	}

	var Widths pdf.Array
	for i := firstCid; i <= lastCid; i++ {
		width := 0
		if t.used[byte(i)] {
			gid := cid2gid[i]
			width = int(float64(t.Ttf.Width[gid])*q + 0.5)
		}
		Widths = append(Widths, pdf.Integer(width))
	}

	// Following section 9.6.6.4 of PDF 32000-1:2008, for PDF versions before
	// 1.3 we mark all fonts as symbolic, so that the CMap for glyph selection
	// works.
	flags := t.Ttf.Flags
	if w.Version < pdf.V1_3 {
		flags &= ^font.FlagNonsymbolic
		flags |= font.FlagSymbolic
	}

	fontName := pdf.Name(subsetTag + "+" + t.Ttf.FontName)

	// See sections 9.6.2.1 and 9.6.3 of PDF 32000-1:2008.
	Font := pdf.Dict{
		"Type":           pdf.Name("Font"),
		"Subtype":        pdf.Name("TrueType"),
		"BaseFont":       fontName,
		"FirstChar":      pdf.Integer(firstCid),
		"LastChar":       pdf.Integer(lastCid),
		"FontDescriptor": t.FontDescriptorRef,
		"Widths":         t.WidthsRef,
		"ToUnicode":      t.ToUnicodeRef,
	}

	// See sections 9.8.1 of PDF 32000-1:2008.
	FontDescriptor := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"FontName":    fontName,
		"Flags":       pdf.Integer(flags),
		"FontBBox":    FontBBox,
		"ItalicAngle": pdf.Number(t.Ttf.ItalicAngle),
		"Ascent":      pdf.Integer(q*float64(t.Ttf.Ascent) + 0.5),
		"Descent":     pdf.Integer(q*float64(t.Ttf.Descent) + 0.5),
		"CapHeight":   pdf.Integer(q*float64(t.Ttf.CapHeight) + 0.5),
		"StemV":       pdf.Integer(70),
		"FontFile2":   t.FontFileRef,
	}

	_, err := w.WriteCompressed(
		[]*pdf.Reference{t.FontRef, t.FontDescriptorRef, t.WidthsRef},
		Font, FontDescriptor, Widths)
	if err != nil {
		return err
	}

	err = font.WriteToUnicodeSimple(w, subsetTag, cid2text, t.ToUnicodeRef)
	if err != nil {
		return err
	}

	err = t.WriteFontFile(w, cid2gid)
	if err != nil {
		return err
	}

	err = t.Ttf.Close()
	if err != nil {
		return err
	}

	return err
}

func (t *ttfSimple) WriteFontFile(w *pdf.Writer, cid2gid []font.GlyphID) error {
	// See section 9.9 of PDF 32000-1:2008.
	size := w.NewPlaceholder(10)
	fontFileDict := pdf.Dict{
		"Length1": size,
	}
	fontFileStream, _, err := w.OpenStream(fontFileDict, t.FontFileRef,
		&pdf.FilterInfo{Name: "FlateDecode"})
	if err != nil {
		return err
	}
	var mapping []sfnt.CMapEntry
	for cid, gid := range cid2gid {
		if gid != 0 {
			mapping = append(mapping, sfnt.CMapEntry{
				CID: uint16(cid),
				GID: gid,
			})
		}
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
		Mapping: mapping,
	}
	n, err := t.Ttf.Export(fontFileStream, exOpt)
	if err != nil {
		return err
	}
	err = size.Set(pdf.Integer(n))
	if err != nil {
		return err
	}
	err = fontFileStream.Close()
	if err != nil {
		return err
	}

	return nil
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
