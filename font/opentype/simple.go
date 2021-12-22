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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
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
// Use of OpenType fonts in PDF requires PDF version 1.6 or higher.
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
// Use of OpenType fonts in PDF requires PDF version 1.6 or higher.
func EmbedFontSimple(w *pdf.Writer, tt *sfnt.Font, instName pdf.Name) (*font.Font, error) {
	if !tt.IsOpenType() {
		return nil, errors.New("not an OpenType font")
	}
	if tt.IsTrueType() {
		return truetype.EmbedFontSimple(w, tt, instName)
	}
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

func newOtfSimple(w *pdf.Writer, tt *sfnt.Font, instName pdf.Name) (*otfSimple, error) {
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

func (t *otfSimple) Layout(rr []rune) []font.Glyph {
	gg := make([]font.Glyph, len(rr))
	for i, r := range rr {
		gid := t.Otf.CMap[r]
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

	return gg
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
	panic("not implemented")
}
