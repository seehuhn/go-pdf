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

// Package builtin implements support for the 14 built-in PDF fonts.
package builtin

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/type1/names"
)

// Embed loads one of the 14 builtin fonts and embeds it into a PDF file.
// The valid font names are given in [FontNames].
func Embed(w *pdf.Writer, fontName string, resName pdf.Name) (font.Embedded, error) {
	font, err := Font(fontName)
	if err != nil {
		return nil, err
	}
	return font.Embed(w, resName)
}

// EmbedAfm loads a simple Type 1 font described by `afm` and embeds it
// into a PDF file.
func EmbedAfm(w *pdf.Writer, afm *AfmInfo, resName pdf.Name) (font.Embedded, error) {
	font, err := FontAfm(afm)
	if err != nil {
		return nil, err
	}
	return font.Embed(w, resName)
}

// Font returns a Font structure representing one of the 14 builtin fonts.
// The valid font names are given in [FontNames].
func Font(fontName string) (font.Font, error) {
	afm, err := Afm(fontName)
	if err != nil {
		return nil, err
	}
	return FontAfm(afm)
}

// FontAfm returns a Font structure representing a simple Type 1 font,
// described by `afm`.
func FontAfm(afm *AfmInfo) (font.Font, error) {
	if len(afm.Code) == 0 {
		return nil, errors.New("no glyphs in font")
	}

	g := &font.Geometry{
		UnitsPerEm:   1000,
		GlyphExtents: afm.GlyphExtents,
		Widths:       afm.Widths,

		Ascent:       afm.Ascent,
		Descent:      afm.Descent,
		BaseLineSkip: 1200, // TODO(voss): is this ok?
		// TODO(voss): UnderlinePosition
		// TODO(voss): UnderlineThickness
	}

	cmap := make(map[rune]int)
	for gid, name := range afm.GlyphName {
		rr := names.ToUnicode(name, afm.IsDingbats)
		if len(rr) != 1 {
			continue
		}
		r := rr[0]
		if j, exists := cmap[r]; exists && afm.GlyphName[j] < name {
			// In case two names map to the same rune, use the
			// one with the lexicographically earlier name.
			continue
		}
		cmap[r] = gid
	}

	res := &builtin{
		afm:  afm,
		g:    g,
		cmap: cmap,
	}
	return res, nil
}

type builtin struct {
	afm  *AfmInfo
	g    *font.Geometry
	cmap map[rune]int
}

func (f *builtin) GetGeometry() *font.Geometry {
	return f.g
}

func (f *builtin) Layout(s string, ptSize float64) glyph.Seq {
	rr := []rune(s)
	gg := make(glyph.Seq, len(rr))
	for i, r := range rr {
		gid := f.cmap[r]
		gg[i].Gid = glyph.ID(gid)
		gg[i].Text = []rune{r}
	}

	// TODO(voss): merge this into the loop above
	if len(gg) >= 2 {
		var res glyph.Seq
		last := gg[0]
		for _, g := range gg[1:] {
			lig, ok := f.afm.Ligatures[glyph.Pair{Left: last.Gid, Right: g.Gid}]
			if ok {
				last.Gid = lig
				last.Text = append(last.Text, g.Text...)
			} else {
				res = append(res, last)
				last = g
			}
		}
		gg = append(res, last)
	}

	for i := range gg {
		gg[i].Advance = f.afm.Widths[gg[i].Gid]
	}

	if len(gg) >= 2 {
		for i := 0; i < len(gg)-1; i++ {
			kern := f.afm.Kern[glyph.Pair{Left: gg[i].Gid, Right: gg[i+1].Gid}]
			gg[i].Advance += kern
		}
	}

	return gg
}

func (f *builtin) Embed(w *pdf.Writer, resName pdf.Name) (font.Embedded, error) {
	res := &embedded{
		builtin: f,
		w:       w,
		ref:     w.Alloc(),
		resName: resName,
		enc:     cmap.NewSimpleEncoder(),
	}

	w.AutoClose(res)

	return res, nil
}

type embedded struct {
	*builtin
	w       *pdf.Writer
	ref     *pdf.Reference
	resName pdf.Name
	enc     cmap.SimpleEncoder
	closed  bool
}

func (e *embedded) AppendEncoded(s pdf.String, gid glyph.ID, rr []rune) pdf.String {
	return append(s, e.enc.Encode(gid, rr))
}

func (e *embedded) Reference() *pdf.Reference {
	return e.ref
}

func (e *embedded) ResourceName() pdf.Name {
	return e.resName
}

func (e *embedded) Close() error {
	if e.closed {
		return nil
	}
	e.closed = true

	if e.enc.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			e.resName, e.afm.FontName)
	}
	e.enc = cmap.NewFrozenSimpleEncoder(e.enc)

	// See section 9.6.2.1 of PDF 32000-1:2008.
	Font := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name(e.afm.FontName),
	}

	builtinEncoding := make([]glyph.ID, 256)
	for gid, code := range e.afm.Code {
		if code > 0 && code < 256 {
			builtinEncoding[code] = glyph.ID(gid)
		}
	}
	enc := font.DescribeEncoding(e.enc.Encoding(), builtinEncoding,
		e.afm.GlyphName, e.afm.IsDingbats)
	if enc != nil {
		Font["Encoding"] = enc
	}
	if e.w.Version == pdf.V1_0 {
		Font["Name"] = e.resName
	}

	_, err := e.w.Write(Font, e.ref)
	return err
}
