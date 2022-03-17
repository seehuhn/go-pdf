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

package builtin

import (
	"errors"
	"sort"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/names"
)

// Embed returns a Font structure representing one of the 14 builtin fonts.
// The valid font names are given in FontNames.
func Embed(w *pdf.Writer, fontName string, instName pdf.Name) (*font.Font, error) {
	afm, err := Afm(fontName)
	if err != nil {
		return nil, err
	}
	return EmbedAfm(w, afm, instName)
}

// EmbedAfm returns a Font structure representing a simple Type 1 font,
// described by `afm`.
func EmbedAfm(w *pdf.Writer, afm *AfmInfo, instName pdf.Name) (*font.Font, error) {
	if len(afm.Code) == 0 {
		return nil, errors.New("no glyphs in font")
	}

	fontRef := w.Alloc()
	fnt := newSimple(afm, fontRef, instName)
	w.OnClose(fnt.WriteFont)

	res := &font.Font{
		InstName: instName,
		Ref:      fnt.FontRef,

		GlyphUnits:  1000,
		Ascent:      afm.Ascent,
		Descent:     afm.Descent,
		GlyphExtent: afm.GlyphExtent,
		Widths:      afm.Width,

		Layout: fnt.Layout,
		Enc:    fnt.Enc,
	}
	return res, nil
}

type simple struct {
	instName pdf.Name
	afm      *AfmInfo

	FontRef *pdf.Reference

	CMap map[rune]font.GlyphID
	char []rune

	enc        map[font.GlyphID]byte
	candidates []*candidate
	used       map[byte]bool // is CharCode used or not?

	overflowed bool
}

func newSimple(afm *AfmInfo, fontRef *pdf.Reference, instName pdf.Name) *simple {
	cmap := make(map[rune]font.GlyphID)
	char := make([]rune, len(afm.Code))
	thisFontEnc := make(fontEnc)
	for gid, code := range afm.Code {
		rr := names.ToUnicode(afm.GlyphName[gid], afm.IsDingbats)
		if len(rr) != 1 {
			// Some names produce no or more than one unicode runes.  Not sure
			// how to handle these cases ...
			continue
		}
		r := rr[0]
		cmap[r] = font.GlyphID(gid)
		char[gid] = r
		if code >= 0 {
			thisFontEnc[r] = byte(code)
		}
	}

	res := &simple{
		instName: instName,
		afm:      afm,

		FontRef: fontRef,

		CMap: cmap,
		char: char,
		enc:  make(map[font.GlyphID]byte),
		used: map[byte]bool{},

		candidates: []*candidate{
			{name: "", enc: thisFontEnc},
			{name: "WinAnsiEncoding", enc: font.WinAnsiEncoding},
			{name: "MacRomanEncoding", enc: font.MacRomanEncoding},
			{name: "MacExpertEncoding", enc: font.MacExpertEncoding},
		},
	}

	return res
}

func (fnt *simple) Layout(rr []rune) []font.Glyph {
	if len(rr) == 0 {
		return nil
	}

	gg := make([]font.Glyph, len(rr))
	for i, r := range rr {
		gid, _ := fnt.CMap[r]
		gg[i].Gid = gid
		gg[i].Chars = []rune{r}
	}

	var res []font.Glyph
	last := gg[0]
	for _, g := range gg[1:] {
		lig, ok := fnt.afm.Ligatures[font.GlyphPair{last.Gid, g.Gid}]
		if ok {
			last.Gid = lig
			last.Chars = append(last.Chars, g.Chars...)
		} else {
			res = append(res, last)
			last = g
		}
	}
	gg = append(res, last)

	for i := range gg {
		gg[i].Advance = int32(fnt.afm.Width[gg[i].Gid])
	}
	if len(gg) < 2 {
		return gg
	}
	for i := 0; i < len(gg)-1; i++ {
		kern := fnt.afm.Kern[font.GlyphPair{gg[i].Gid, gg[i+1].Gid}]
		gg[i].Advance += int32(kern)
	}

	return gg
}

func (fnt *simple) Enc(gid font.GlyphID) pdf.String {
	c, ok := fnt.enc[gid]
	if ok {
		return pdf.String{c}
	}

	r := fnt.char[gid]
	hits := -1
	found := false
	for _, cand := range fnt.candidates {
		if cand.hits < hits {
			continue
		}
		cCand, ok := cand.enc.Encode(r)
		if ok && !fnt.used[cCand] {
			c = cCand
			hits = cand.hits
			found = true
		}
	}
	if !found && len(fnt.used) < 256 {
		for i := 255; i >= 0; i-- {
			c = byte(i)
			if !fnt.used[c] {
				found = true
				break
			}
		}
	}

	if !found {
		// A simple font can only encode 256 different characters. If we run
		// out of character codes, just return 0 here and report an error when
		// we try to write the font dictionary at the end.
		fnt.overflowed = true
		fnt.enc[gid] = 0
		return pdf.String{0}
	}

	fnt.used[c] = true
	fnt.enc[gid] = c
	if c != 0 {
		for _, cand := range fnt.candidates {
			if cTest, ok := cand.enc.Encode(r); ok && cTest == c {
				cand.hits++
			}
		}
	}

	return pdf.String{c}
}

func (fnt *simple) WriteFont(w *pdf.Writer) error {
	if fnt.overflowed {
		return errors.New("too many different glyphs for simple font " + string(fnt.afm.FontName))
	}

	// See section 9.6.2.1 of PDF 32000-1:2008.
	Font := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name(fnt.afm.FontName),
	}

	enc := fnt.DescribeEncoding()
	if enc != nil {
		Font["Encoding"] = enc
	}

	if w.Version == pdf.V1_0 {
		Font["Name"] = pdf.Name(fnt.instName)
	}

	_, err := w.Write(Font, fnt.FontRef)
	return err
}

func (fnt *simple) DescribeEncoding() pdf.Object {
	best := fnt.candidates[0]
	for _, cand := range fnt.candidates[1:] {
		if cand.hits > best.hits {
			best = cand
		}
	}
	var baseName pdf.Object
	if best.name != "" {
		baseName = pdf.Name(best.name)
	}

	type D struct {
		code byte
		char rune
		name string
	}
	var diff []D
	for gid, code := range fnt.enc {
		r := fnt.char[gid]
		if cTest, ok := best.enc.Encode(r); !ok || cTest != code {
			diff = append(diff, D{
				code: code,
				char: r,
				name: fnt.afm.GlyphName[gid],
			})
		}
	}
	if len(diff) == 0 {
		return baseName
	}

	Differences := pdf.Array{}
	sort.Slice(diff, func(i, j int) bool {
		return diff[i].code < diff[j].code
	})
	var next byte
	first := true
	for _, d := range diff {
		if first || d.code != next {
			Differences = append(Differences, pdf.Integer(d.code))
		}
		Differences = append(Differences, pdf.Name(d.name))
		next = d.code + 1
		first = false
	}

	return pdf.Dict{
		"Type":         pdf.Name("Encoding"),
		"BaseEncoding": baseName,
		"Differences":  Differences,
	}
}

type fontEnc map[rune]byte

func (fe fontEnc) Encode(r rune) (byte, bool) {
	c, ok := fe[r]
	return c, ok
}

// this is half of font.Encoding
type encoder interface {
	Encode(r rune) (byte, bool)
}

type candidate struct {
	name string
	enc  encoder
	hits int
}
